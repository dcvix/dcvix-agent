//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"net/http"

	"github.com/dcvix/dcvix-agent/internal/stats"
	"github.com/dcvix/dcvix-agent/internal/updater"
	log "github.com/sirupsen/logrus"
)

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	log.Debugf("GET /v1/stats: request from %s", r.RemoteAddr)
	sessions, err := s.dcvManager.ListSessions()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	sysStats, err := stats.CollectStats()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	update := updater.DirectorUpdate{
		Sessions: sessions,
		Stats:    sysStats,
		Tags:     s.config.Agent.Tags,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(update)
}
