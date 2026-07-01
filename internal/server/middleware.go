//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

package server

import (
	"net"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

// requireDirectorAddress restricts access to the director's IP.
func (s *Server) requireDirectorAddress(next http.HandlerFunc) http.HandlerFunc {
	directorIPs, err := net.LookupHost(s.config.Agent.DirectorHost)
	if err != nil {
		log.Fatalf("Failed to resolve director_host %s: %v", s.config.Agent.DirectorHost, err)
	}
	ipSet := make(map[string]struct{}, len(directorIPs))
	for _, ip := range directorIPs {
		ipSet[ip] = struct{}{}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		clientIP := extractIP(r.RemoteAddr)
		if _, ok := ipSet[clientIP]; !ok {
			log.Warnf("Access denied from IP %s, not director_host %s", clientIP, s.config.Agent.DirectorHost)
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// extractIP extracts the IP address from an address string.
func extractIP(remoteAddr string) string {
	lastColon := strings.LastIndex(remoteAddr, ":")
	if lastColon == -1 {
		return remoteAddr
	}
	ip := remoteAddr[:lastColon]
	ip = strings.TrimPrefix(ip, "[")
	ip = strings.TrimSuffix(ip, "]")
	return ip
}
