//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
)

type ConfigEntry struct {
	Section string `json:"section"`
	Key     string `json:"key"`
	Value   string `json:"value"`
}

// handleSetConfig handles POST /v1/config.
func (s *Server) handleSetConfig(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Config []ConfigEntry `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Errorf("POST /v1/config: invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	for _, entry := range request.Config {
		if entry.Section == "" || entry.Key == "" {
			log.Error("POST /v1/config: section and key must not be empty")
			http.Error(w, "section and key must not be empty", http.StatusBadRequest)
			return
		}
		if err := s.dcvManager.SetConfig(entry.Section, entry.Key, entry.Value); err != nil {
			log.Errorf("POST /v1/config: set failed: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}
