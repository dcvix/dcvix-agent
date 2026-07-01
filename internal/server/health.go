//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// handleHealth handles GET /v1/health.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	log.Infof("GET /v1/health: check from %s", r.RemoteAddr)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}
