//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

// Package reaper closes idle sessions.

package reaper

import (
	"fmt"
	"time"

	"github.com/dcvix/dcvix-agent/internal/config"
	"github.com/dcvix/dcvix-agent/internal/dcv"
	log "github.com/sirupsen/logrus"
)

type Reaper struct {
	config     *config.Config
	dcvManager *dcv.DCVManager
}

func NewReaper(cfg *config.Config, dcvManager *dcv.DCVManager) (*Reaper, error) {
	return &Reaper{
		config:     cfg,
		dcvManager: dcvManager,
	}, nil
}

func (r *Reaper) ReapIdleSessions() error {
	sessions, err := r.dcvManager.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	idleThreshold := time.Duration(r.config.Agent.ReapInterval) * time.Second

	for _, session := range sessions {

		// Don't close sessions with active connections
		if session.NumOfConnections > 0 {
			continue
		}

		// if the session was never connected close only if old, to avoid closing immediately after creation.
		if session.LastDisconnectionTime.IsZero() && time.Since(session.CreationTime.Time) <= idleThreshold {
			continue
		}

		log.Infof("Reaping idle session %s (owner: %s)", session.ID, session.Owner)
		if err := r.dcvManager.CloseSession(session.ID); err != nil {
			log.Errorf("Failed to close idle session %s: %v", session.ID, err)
		}
	}

	return nil
}

func (r *Reaper) StartReaper() {
	interval := r.config.Agent.ReapInterval
	if interval == 0 {
		log.Infof("Session reaper disabled (reap_interval = 0)")
		return
	}
	log.Infof("Session reaper started, checking every %d seconds for idle sessions", interval)

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := r.ReapIdleSessions(); err != nil {
			log.Errorf("Reap idle sessions failed: %v", err)
		}
	}
}
