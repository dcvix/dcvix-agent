//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

package server

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// handleListSessions handles GET /v1/sessions.
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.dcvManager.ListSessions()
	if err != nil {
		log.Errorf("GET /v1/sessions: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// handleCreateSession handles POST /v1/sessions.
func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	log.Debugf("POST /v1/sessions: request from %s", r.RemoteAddr)

	var request struct {
		UserID      string `json:"userId"`
		SessionType string `json:"sessionType"`
		SessionID   string `json:"sessionId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Errorf("POST /v1/sessions: invalid request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	log.Debugf("POST /v1/sessions: request body: %v", request)

	sessions, err := s.dcvManager.ListSessions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, session := range sessions {
		if session.Owner == request.UserID && session.Type == request.SessionType {
			log.Debugf("POST /v1/sessions: session already exists: %s", request.SessionID)
			// Session already exist just return session id to director
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"sessionID": session.ID})
			return
		}
	}

	if err := s.dcvManager.CreateSession(request.UserID, request.SessionType, request.SessionID); err != nil {
		log.Errorf("POST /v1/sessions: create failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"sessionID": request.SessionID})
}

// handleCloseSession handles DELETE /v1/sessions/{id}.
func (s *Server) handleCloseSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	if err := s.dcvManager.CloseSession(sessionID); err != nil {
		log.Errorf("DELETE /v1/sessions/{id}: close failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
