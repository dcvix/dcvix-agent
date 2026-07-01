//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/dcvix/dcvix-agent/internal/registrator"
	"github.com/dcvix/dcvix-agent/internal/renewer"
	"github.com/dcvix/dcvix-agent/internal/service"
	"github.com/dcvix/dcvix-agent/internal/version"

	log "github.com/sirupsen/logrus"
)

func main() {

	showVersion := flag.Bool("version", false, "Show version information")
	configPath := flag.String("conf", "", "Path to configuration file")
	installService := flag.Bool("install", false, "Install Windows service")
	uninstallService := flag.Bool("uninstall", false, "Uninstall Windows service")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.String())
		os.Exit(0)
	}

	// Handle Windows service installation/uninstallation
	if runtime.GOOS == "windows" {
		if *installService {
			if err := service.InstallService(); err != nil {
				log.Fatalf("Failed to install service: %v", err)
			}
			fmt.Println("Service installed successfully")
			os.Exit(0)
		}
		if *uninstallService {
			if err := service.UninstallService(); err != nil {
				log.Fatalf("Failed to uninstall service: %v", err)
			}
			fmt.Println("Service uninstalled successfully")
			os.Exit(0)
		}

		// Check if running as a service
		isService, err := service.IsWindowsService()
		if err != nil {
			log.Fatalf("Failed to determine if running as service: %v", err)
		}
		if isService {
			if err := service.RunService(); err != nil {
				log.Fatalf("Failed to run as service: %v", err)
			}
			return
		}
	}

	// Phase 1: load config, setup logger, create components that don't need CA cert
	cfg, certManager, dcvManager, reaper, err := service.InitBase(*configPath)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	log.Infof("Version: %s", version.String())
	log.Info("Starting")

	// Phase 2: register with director (blocks until registered)
	reg := registrator.NewRegistrator(certManager, cfg.Agent.DirectorHost, cfg.Agent.DirectorPort)
	regCtx, regCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	if err := reg.Register(regCtx); err != nil {
		log.Fatalf("Failed to register with director: %v", err)
	}
	regCancel()

	// Phase 3: create mTLS-dependent components now that CA cert exists
	u, s, err := service.InitMTLS(cfg, dcvManager, certManager)
	if err != nil {
		log.Fatalf("Failed to create mTLS components: %v", err)
	}

	// Start background loops
	renCtx, renCancel := context.WithCancel(context.Background())
	go renewer.NewRenewer(certManager, cfg.Agent.DirectorHost, cfg.Agent.DirectorPort).StartRenewer(renCtx)
	log.Info("Renewer started")

	go u.StartUpdater()
	log.Infof("Updater started, using director %s:%s", cfg.Agent.DirectorHost, cfg.Agent.DirectorPort)

	go reaper.StartReaper()
	log.Infof("Session reaper started, checking every %d seconds", cfg.Agent.ReapInterval)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := s.Start(); err != nil {
			log.Infof("Server error: %v", err)
			sigChan <- syscall.SIGTERM
		}
	}()
	log.Infof("Listening on port %d", cfg.Agent.AgentPort)

	// Wait for shutdown signal
	<-sigChan
	log.Info("Shutting down")
	renCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		log.Errorf("Server forced to shutdown: %v", err)
	}
}
