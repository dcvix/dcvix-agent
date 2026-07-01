//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

package server

import (
	"context"
	"crypto/tls"
	"fmt"
	stdliblog "log"
	"net"
	"net/http"
	"time"

	"github.com/dcvix/dcvix-agent/internal/certificate"
	"github.com/dcvix/dcvix-agent/internal/config"
	"github.com/dcvix/dcvix-agent/internal/dcv"
	log "github.com/sirupsen/logrus"
)

type Server struct {
	config      *config.Config
	dcvManager  *dcv.DCVManager
	certManager *certificate.Manager
	httpServer  *http.Server
}

func NewServer(cfg *config.Config, dcvManager *dcv.DCVManager, certManager *certificate.Manager) *Server {
	return &Server{
		config:      cfg,
		dcvManager:  dcvManager,
		certManager: certManager,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", s.handleHealth)
	mux.HandleFunc("GET /v1/sessions", s.requireDirectorAddress(s.handleListSessions))
	mux.HandleFunc("POST /v1/sessions", s.requireDirectorAddress(s.handleCreateSession))
	mux.HandleFunc("DELETE /v1/sessions/{id}", s.requireDirectorAddress(s.handleCloseSession))
	mux.HandleFunc("POST /v1/config", s.requireDirectorAddress(s.handleSetConfig))
	mux.HandleFunc("GET /v1/stats", s.requireDirectorAddress(s.handleStats))
	// NOTE: could add endpoint /v1/get-screenshot which runs `dcv get-screenshot autosession --json`
	// but raises privacy concerns.

	caPool := s.certManager.CACertPool()
	if caPool == nil {
		return fmt.Errorf("CA certificate not available")
	}

	tlsConfig := &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert, err := s.certManager.LoadCertificate()
			return &cert, err
		},
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  caPool,
		MinVersion: tls.VersionTLS12,
	}

	addr := fmt.Sprintf("%s:%d", s.config.Agent.BindAddress, s.config.Agent.AgentPort)
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ErrorLog:          stdliblog.New(log.StandardLogger().WriterLevel(log.WarnLevel), "", 0),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	tlsListener := tls.NewListener(listener, tlsConfig)

	log.Infof("Listening on https://%s", addr)
	return s.httpServer.Serve(tlsListener)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}


