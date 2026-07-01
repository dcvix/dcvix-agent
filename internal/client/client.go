//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dcvix/dcvix-agent/internal/certificate"
)

type MTLSClient struct {
	httpClient *http.Client
}

// NewMTLSClient creates an mTLS client using the certificate manager.
// The client cert is loaded dynamically via GetClientCertificate callback.
// Uses CA cert from the manager for strict server verification.
// Hostname verification is intentionally bypassed, the director is identified by its CA.
func NewMTLSClient(certManager *certificate.Manager) (*MTLSClient, error) {
	caPool := certManager.CACertPool()
	if caPool == nil {
		return nil, fmt.Errorf("CA certificate not available")
	}

	verifyPeerCert := func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		opts := x509.VerifyOptions{
			Roots:         caPool,
			CurrentTime:   time.Now(),
			Intermediates: x509.NewCertPool(),
		}
		cert, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return fmt.Errorf("failed to parse certificate: %w", err)
		}
		_, err = cert.Verify(opts)
		return err
	}

	tlsConfig := &tls.Config{
		GetClientCertificate: func(hello *tls.CertificateRequestInfo) (*tls.Certificate, error) {
			cert, err := certManager.LoadCertificate()
			return &cert, err
		},
		RootCAs:               caPool,
		InsecureSkipVerify:    false,
		VerifyPeerCertificate: verifyPeerCert,
		MinVersion:            tls.VersionTLS12,
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: 30 * time.Second,
	}

	return &MTLSClient{httpClient: httpClient}, nil
}

// NewRegistrationClient creates a simple HTTPS client for TOFU registration.
// If caPool is non-nil (pre-deployed CA), it uses strict server verification.
// Otherwise, InsecureSkipVerify is true and any server cert is accepted.
func NewRegistrationClient(caPool *x509.CertPool) *http.Client {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if caPool != nil {
		tlsConfig.RootCAs = caPool
		tlsConfig.InsecureSkipVerify = false
	} else {
		tlsConfig.InsecureSkipVerify = true
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: 30 * time.Second,
	}
}

func (c *MTLSClient) Get(url string) ([]byte, int, error) {
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

func (c *MTLSClient) Post(url string, contentType string, body io.Reader) ([]byte, int, error) {
	resp, err := c.httpClient.Post(url, contentType, body)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	return respBody, resp.StatusCode, err
}
