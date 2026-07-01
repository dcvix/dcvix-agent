// SPDX-FileCopyrightText: 2026 Diego Cortassa
// SPDX-License-Identifier: MIT

package registrator

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/dcvix/dcvix-agent/internal/certificate"
	"github.com/dcvix/dcvix-agent/internal/client"
	log "github.com/sirupsen/logrus"
)

type registerRequest struct {
	CSR      string `json:"csr"`
	GUID     string `json:"guid"`
	Hostname string `json:"hostname"`
}

type registerResponse struct {
	Certificate           string `json:"certificate"`
	CA                    string `json:"ca"`
	AgentID               string `json:"agentId"`
	RenewalIntervalSeconds int    `json:"renewalIntervalSeconds"`
}

type registerErrorResponse struct {
	Error string `json:"error"`
}

type Registrator struct {
	certManager *certificate.Manager
	directorURL string
}

func NewRegistrator(certManager *certificate.Manager, directorHost, directorPort string) *Registrator {
	return &Registrator{
		certManager: certManager,
		directorURL: fmt.Sprintf("https://%s:%s/v1/agent/register", directorHost, directorPort),
	}
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// Register loops until registration succeeds or context is cancelled.
func (r *Registrator) Register(ctx context.Context) error {
	guid, err := r.certManager.EnsureGUID()
	if err != nil {
		return fmt.Errorf("failed to ensure GUID: %w", err)
	}

	if _, err := r.certManager.EnsureKeyPair(); err != nil {
		return fmt.Errorf("failed to ensure key pair: %w", err)
	}

	caPool := r.certManager.CACertPool()
	httpClient := client.NewRegistrationClient(caPool)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		csrDER, err := r.certManager.GenerateCSR(guid)
		if err != nil {
			log.Warnf("Failed to generate CSR: %v", err)
			time.Sleep(30 * time.Second)
			continue
		}

		req := registerRequest{
			CSR:      base64.StdEncoding.EncodeToString(csrDER),
			GUID:     guid,
			Hostname: hostname(),
		}

		body, err := json.Marshal(req)
		if err != nil {
			log.Warnf("Failed to marshal request: %v", err)
			time.Sleep(30 * time.Second)
			continue
		}

		resp, err := httpClient.Post(r.directorURL, "application/json", bytes.NewBuffer(body))
		if err != nil {
			log.Warnf("Connection failed: %v", err)
			time.Sleep(30 * time.Second)
			continue
		}

		switch resp.StatusCode {
		case http.StatusOK:
			var regResp registerResponse
			if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
				resp.Body.Close()
				log.Warnf("Failed to decode response: %v", err)
				time.Sleep(30 * time.Second)
				continue
			}
			resp.Body.Close()
			if err := r.certManager.StoreCertificate([]byte(regResp.Certificate)); err != nil {
				return fmt.Errorf("failed to store certificate: %w", err)
			}
			if err := r.certManager.StoreCACert([]byte(regResp.CA)); err != nil {
				return fmt.Errorf("failed to store CA certificate: %w", err)
			}
			log.Infof("Agent %s registered successfully", guid)
			return nil

		case http.StatusForbidden:
			var errResp registerErrorResponse
			json.NewDecoder(resp.Body).Decode(&errResp)
			resp.Body.Close()
			log.Infof("Pending approval (GUID: %s): %s", guid, errResp.Error)
			time.Sleep(30 * time.Second)

		default:
			resp.Body.Close()
			log.Warnf("Unexpected status %d", resp.StatusCode)
			time.Sleep(30 * time.Second)
		}
	}
}
