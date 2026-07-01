//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

// Package service handles the service lifecycle and configuration.

package service

import (
	"fmt"

	"github.com/dcvix/dcvix-agent/internal/certificate"
	"github.com/dcvix/dcvix-agent/internal/config"
	"github.com/dcvix/dcvix-agent/internal/dcv"
	"github.com/dcvix/dcvix-agent/internal/logger"
	"github.com/dcvix/dcvix-agent/internal/reaper"
	"github.com/dcvix/dcvix-agent/internal/server"
	"github.com/dcvix/dcvix-agent/internal/updater"
)

// InitBase loads config, sets up logging, and creates components that don't
// require the CA certificate (only available after registration with the director).
func InitBase(configPath string) (*config.Config, *certificate.Manager, *dcv.DCVManager, *reaper.Reaper, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	if err := logger.SetupLogger(cfg.Log); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to setup logger: %w", err)
	}


	dcvManager := dcv.NewDCVManager(cfg.Agent.DCVPath)
	certManager, err := certificate.NewManager(cfg.Agent.DataDir)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create certificate manager: %w", err)
	}

	r, err := reaper.NewReaper(cfg, dcvManager)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create reaper: %w", err)
	}

	return cfg, certManager, dcvManager, r, nil
}

// InitMTLS creates components that require the CA certificate.
// Must be called after successful registration with the director.
func InitMTLS(cfg *config.Config, dcvManager *dcv.DCVManager, certManager *certificate.Manager) (*updater.Updater, *server.Server, error) {
	u, err := updater.NewUpdater(cfg, dcvManager, certManager)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create updater: %w", err)
	}

	s := server.NewServer(cfg, dcvManager, certManager)

	return u, s, nil
}
