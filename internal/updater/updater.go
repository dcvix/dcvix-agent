//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

// Package updater sends updates to the director

package updater

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/dcvix/dcvix-agent/internal/certificate"
	"github.com/dcvix/dcvix-agent/internal/client"
	"github.com/dcvix/dcvix-agent/internal/config"
	"github.com/dcvix/dcvix-agent/internal/dcv"
	"github.com/dcvix/dcvix-agent/internal/stats"
	log "github.com/sirupsen/logrus"
)

type DirectorUpdate struct {
	Sessions []dcv.Session      `json:"sessions"`
	Stats    *stats.SystemStats `json:"stats"`
	Tags     []string           `json:"tags"`
}

type Updater struct {
	config     *config.Config
	dcvManager *dcv.DCVManager
	mTLSClient *client.MTLSClient
}

func NewUpdater(cfg *config.Config, dcvManager *dcv.DCVManager, certManager *certificate.Manager) (*Updater, error) {
	mTLSClient, err := client.NewMTLSClient(certManager)
	if err != nil {
		return nil, fmt.Errorf("failed to create mTLS client: %w", err)
	}

	return &Updater{
		config:     cfg,
		dcvManager: dcvManager,
		mTLSClient: mTLSClient,
	}, nil
}

func (u *Updater) SendUpdate() error {
	sessions, err := u.dcvManager.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	sysStats, err := stats.CollectStats()
	if err != nil {
		return fmt.Errorf("failed to collect system stats: %w", err)
	}

	update := DirectorUpdate{
		Sessions: sessions,
		Stats:    sysStats,
		Tags:     u.config.Agent.Tags,
	}

	data, err := json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal update: %w", err)
	}

	url := fmt.Sprintf("https://%s:%s/v1/agent/update",
		u.config.Agent.DirectorHost,
		u.config.Agent.DirectorPort)

	log.Debugf("Sending data to %v: %s", url, data)
	_, statusCode, err := u.mTLSClient.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to send update: %w", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("director returned status: %d", statusCode)
	}

	return nil
}

func (u *Updater) StartUpdater() {

	interval := u.config.Agent.UpdateInterval
	if interval == 0 {
		log.Infof("Updater disabled (update_interval = 0)")
		return
	}

	// Random delay to stagger updater start times across agents
	n := rand.Intn(interval)
	log.Infof("Waiting %d seconds (random delay) before starting", n)
	time.Sleep(time.Duration(n) * time.Second)

	// Fire immediately on startup so the director knows the agent exists
	if err := u.SendUpdate(); err != nil {
		log.Errorf("Initial update failed: %v", err)
	}

	ticker := time.NewTicker(time.Duration(u.config.Agent.UpdateInterval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if err := u.SendUpdate(); err != nil {
			log.Errorf("Scheduled update failed: %v", err)
		}
	}
}
