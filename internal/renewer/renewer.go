// SPDX-FileCopyrightText: 2026 Diego Cortassa
// SPDX-License-Identifier: MIT

package renewer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dcvix/dcvix-agent/internal/certificate"
	"github.com/dcvix/dcvix-agent/internal/client"
	log "github.com/sirupsen/logrus"
)

type renewRequest struct {
	CSR string `json:"csr"`
}

type renewResponse struct {
	Certificate            string `json:"certificate"`
	RenewalIntervalSeconds int    `json:"renewalIntervalSeconds"`
}

type Renewer struct {
	certManager *certificate.Manager
	directorURL string
	interval    time.Duration
}

func NewRenewer(certManager *certificate.Manager, directorHost, directorPort string) *Renewer {
	return &Renewer{
		certManager: certManager,
		directorURL: fmt.Sprintf("https://%s:%s/v1/agent/renew", directorHost, directorPort),
		interval:    12 * time.Hour,
	}
}

func (r *Renewer) StartRenewer(ctx context.Context) {
	guid, err := r.certManager.EnsureGUID()
	if err != nil {
		log.Errorf("Cannot load GUID: %v", err)
		return
	}

	mTLSClient, err := client.NewMTLSClient(r.certManager)
	if err != nil {
		log.Errorf("Failed to create mTLS client: %v", err)
		return
	}

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("Stopping")
			return
		case <-ticker.C:
			if err := r.tryRenew(mTLSClient, guid); err != nil {
				log.Warnf("Renewal failed: %v", err)
			}
		}
	}
}

// tryRenew attempts to renew the certificate if it needs renewal.
func (r *Renewer) tryRenew(mTLSClient *client.MTLSClient, guid string) error {
	if !r.certManager.CertificateNeedsRenewal() {
		log.Debug("Certificate does not need renewal")
		return nil
	}

	csrDER, err := r.certManager.GenerateCSR(guid)
	if err != nil {
		return fmt.Errorf("failed to generate CSR: %w", err)
	}

	req := renewRequest{
		CSR: base64.StdEncoding.EncodeToString(csrDER),
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	respBody, statusCode, err := mTLSClient.Post(r.directorURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("renew request failed: %w", err)
	}

	if statusCode != http.StatusOK {
		return fmt.Errorf("director returned status %d", statusCode)
	}

	var renewResp renewResponse
	if err := json.Unmarshal(respBody, &renewResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if err := r.certManager.StoreCertificate([]byte(renewResp.Certificate)); err != nil {
		return fmt.Errorf("failed to store renewed certificate: %w", err)
	}

	log.Info("Certificate renewed successfully")
	return nil
}
