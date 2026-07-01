//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

//go:build windows

package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/dcvix/dcvix-agent/internal/certificate"
	"github.com/dcvix/dcvix-agent/internal/config"
	"github.com/dcvix/dcvix-agent/internal/reaper"
	"github.com/dcvix/dcvix-agent/internal/registrator"
	"github.com/dcvix/dcvix-agent/internal/renewer"
	"github.com/dcvix/dcvix-agent/internal/server"
	"github.com/dcvix/dcvix-agent/internal/updater"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

const serviceName = "DcvixAgent"
const serviceDisplayName = "Dcvix Agent"
const serviceDescription = "Dcvix Agent for managing DCV sessions"

type agentService struct {
	cfg         *config.Config
	certManager *certificate.Manager
	updater     *updater.Updater
	reaper      *reaper.Reaper
	srv         *server.Server
	renewer     *renewer.Renewer
	logger      *eventlog.Log
	stop        chan struct{}
}

func (s *agentService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {

	exePath, err := os.Executable()
	if err != nil {
		s.logger.Error(1, fmt.Sprintf("Could not get executable path: %v", err))
	} else {
		exeDir := filepath.Dir(exePath)
		err = os.Chdir(exeDir)
		if err != nil {
			s.logger.Error(1, fmt.Sprintf("Could not change working directory: %v", err))
		} else {
			s.logger.Info(1, fmt.Sprintf("Changed working directory to: %s", exeDir))
		}
	}

	changes <- svc.Status{State: svc.StartPending}

	cfg, certManager, dcvManager, reaper, err := InitBase("")
	if err != nil {
		s.logger.Error(1, fmt.Sprintf("Failed to initialize application: %v", err))
		return true, 1
	}

	s.logger.Info(1, "Starting")

	// Phase 1: register with director (synchronous)
	regCtx, regCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer regCancel()
	reg := registrator.NewRegistrator(certManager, cfg.Agent.DirectorHost, cfg.Agent.DirectorPort)
	if err := reg.Register(regCtx); err != nil {
		s.logger.Error(1, fmt.Sprintf("Registration failed: %v", err))
		return true, 1
	}

	// Phase 2: create mTLS-dependent components now that CA cert exists
	u, srv, err := InitMTLS(cfg, dcvManager, certManager)
	if err != nil {
		s.logger.Error(1, fmt.Sprintf("Failed to create mTLS components: %v", err))
		return true, 1
	}

	ren := renewer.NewRenewer(certManager, cfg.Agent.DirectorHost, cfg.Agent.DirectorPort)
	renCtx, renCancel := context.WithCancel(context.Background())
	go ren.StartRenewer(renCtx)

	go u.StartUpdater()
	s.logger.Info(1, fmt.Sprintf("Updater started, using director %s:%s", cfg.Agent.DirectorHost, cfg.Agent.DirectorPort))

	go reaper.StartReaper()
	s.logger.Info(1, fmt.Sprintf("Session reaper started, checking every %d seconds", cfg.Agent.ReapInterval))

	s.cfg = cfg
	s.certManager = certManager
	s.updater = u
	s.reaper = reaper
	s.srv = srv
	s.renewer = ren
	s.stop = make(chan struct{})

	// Start the server in a goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error(1, fmt.Sprintf("Server goroutine panic: %v", r))
				close(s.stop)
			}
		}()
		if err := s.srv.Start(); err != nil {
			s.logger.Error(1, fmt.Sprintf("Server error: %v", err))
			close(s.stop)
		}
	}()

	s.logger.Info(1, fmt.Sprintf("Listening on port %d", cfg.Agent.AgentPort))

	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	// Wait for stop signal
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				s.logger.Info(1, "Interrogating service")
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				s.logger.Info(1, "Stopping service in select")
				changes <- svc.Status{State: svc.StopPending}
				regCancel()
				renCancel()
				close(s.stop)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := s.srv.Shutdown(ctx); err != nil {
					s.logger.Error(1, fmt.Sprintf("Server forced to shutdown on stop: %v", err))
				}
				return false, 0
			default:
				s.logger.Error(1, fmt.Sprintf("Unexpected control request: %v", c))
			}
		case <-s.stop:
			s.logger.Info(1, "Stopping service")
			regCancel()
			renCancel()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := s.srv.Shutdown(ctx); err != nil {
				s.logger.Error(1, fmt.Sprintf("Server forced to shutdown on stop channel: %v", err))
			}
			return false, 0
		}
	}
}

func InstallService() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", serviceName)
	}

	s, err = m.CreateService(serviceName,
		exePath,
		mgr.Config{
			DisplayName: serviceDisplayName,
			Description: serviceDescription,
			StartType:   mgr.StartAutomatic,
		})
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}
	defer s.Close()

	// Set recovery actions
	recoveryActions := []mgr.RecoveryAction{
		{Type: mgr.ServiceRestart, Delay: 30 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 60 * time.Second},
		{Type: mgr.ServiceRestart, Delay: 120 * time.Second},
	}
	resetPeriod := uint32(24 * time.Hour / time.Second) // Reset after 24 hours

	if err := s.SetRecoveryActions(recoveryActions, resetPeriod); err != nil {
		return fmt.Errorf("failed to set recovery actions: %w", err)
	}

	// Create event log
	if err := eventlog.InstallAsEventCreate(serviceName, eventlog.Error|eventlog.Warning|eventlog.Info); err != nil {
		s.Delete()
		return fmt.Errorf("failed to install event logger: %w", err)
	}

	// Start the service after installation
	if err := s.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	return nil
}

func UninstallService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("service %s not found", serviceName)
	}
	defer s.Close()

	// Stop the service if it is running
	status, err := s.Query()
	if err == nil && status.State != svc.Stopped {
		_, err = s.Control(svc.Stop)
		if err != nil {
			return fmt.Errorf("failed to stop service: %w", err)
		}
		// Wait for the service to actually stop
		for i := 0; i < 30; i++ { // wait up to ~15 seconds
			status, err = s.Query()
			if err != nil {
				break
			}
			if status.State == svc.Stopped {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Remove event log
	if err := eventlog.Remove(serviceName); err != nil {
		log.Printf("Failed to remove event logger: %v", err)
	}

	// Delete service
	if err := s.Delete(); err != nil {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	return nil
}

func RunService() error {
	elog, err := eventlog.Open(serviceName)
	if err != nil {
		return fmt.Errorf("failed to open event log: %w", err)
	}
	defer elog.Close()

	return svc.Run(serviceName, &agentService{logger: elog})
}

func IsWindowsService() (bool, error) {
	return svc.IsWindowsService()
}
