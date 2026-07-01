// SPDX-FileCopyrightText: 2026 Diego Cortassa
// SPDX-License-Identifier: MIT

package certificate

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Manager struct {
	dataDir string
}

func NewManager(dataDir string) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory %s: %w", dataDir, err)
	}
	return &Manager{dataDir: dataDir}, nil
}

func (m *Manager) keyPath() string  { return filepath.Join(m.dataDir, "agent.key") }
func (m *Manager) certPath() string { return filepath.Join(m.dataDir, "agent.crt") }
func (m *Manager) caPath() string   { return filepath.Join(m.dataDir, "ca.pem") }
func (m *Manager) guidPath() string { return filepath.Join(m.dataDir, "agent.guid") }

// EnsureKeyPair generates an Ed25519 key pair if agent.key doesn't exist.
// Returns the public key (existing or newly generated).
func (m *Manager) EnsureKeyPair() (crypto.PublicKey, error) {
	if _, err := os.Stat(m.keyPath()); err == nil {
		keyPEM, err := os.ReadFile(m.keyPath())
		if err != nil {
			return nil, fmt.Errorf("failed to read agent.key: %w", err)
		}
		block, _ := pem.Decode(keyPEM)
		if block == nil {
			return nil, fmt.Errorf("failed to decode agent.key PEM")
		}
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse agent.key: %w", err)
		}
		signer, ok := key.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("agent.key is not a signing key")
		}
		return signer.Public(), nil
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 key: %w", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}
	pemBlock := &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}
	if err := os.WriteFile(m.keyPath(), pem.EncodeToMemory(pemBlock), 0600); err != nil {
		return nil, fmt.Errorf("failed to write agent.key: %w", err)
	}
	return pub, nil
}

// EnsureGUID reads agent.guid from disk, or generates and persists a new UUIDv4.
func (m *Manager) EnsureGUID() (string, error) {
	if data, err := os.ReadFile(m.guidPath()); err == nil {
		return string(data), nil
	}

	guid, err := newUUID()
	if err != nil {
		return "", fmt.Errorf("failed to generate GUID: %w", err)
	}
	if err := os.WriteFile(m.guidPath(), []byte(guid), 0644); err != nil {
		return "", fmt.Errorf("failed to write agent.guid: %w", err)
	}
	return guid, nil
}

// GenerateCSR creates a DER-encoded CSR with CN = dcvix-agent-<guid>.
func (m *Manager) GenerateCSR(guid string) ([]byte, error) {
	keyPEM, err := os.ReadFile(m.keyPath())
	if err != nil {
		return nil, fmt.Errorf("failed to read agent.key: %w", err)
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode agent.key PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent.key: %w", err)
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("agent.key is not a signing key")
	}

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: fmt.Sprintf("dcvix-agent-%s", guid),
		},
	}
	return x509.CreateCertificateRequest(rand.Reader, template, signer)
}

// StoreCertificate writes the PEM-encoded signed certificate to agent.crt.
func (m *Manager) StoreCertificate(certPEM []byte) error {
	return os.WriteFile(m.certPath(), certPEM, 0644)
}

// StoreCACert writes the PEM-encoded CA certificate to ca.pem.
func (m *Manager) StoreCACert(certPEM []byte) error {
	return os.WriteFile(m.caPath(), certPEM, 0644)
}

// LoadCertificate loads agent.crt and agent.key for TLS serving.
func (m *Manager) LoadCertificate() (tls.Certificate, error) {
	return tls.LoadX509KeyPair(m.certPath(), m.keyPath())
}

// CACertPool returns an x509.CertPool with ca.pem loaded, or nil if missing.
func (m *Manager) CACertPool() *x509.CertPool {
	caPEM, err := os.ReadFile(m.caPath())
	if err != nil {
		return nil
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil
	}
	return pool
}

// CertificateNeedsRenewal returns true if agent.crt is missing or expires within 24h.
func (m *Manager) CertificateNeedsRenewal() bool {
	cert, err := tls.LoadX509KeyPair(m.certPath(), m.keyPath())
	if err != nil {
		return true
	}
	leaf := cert.Leaf
	if leaf == nil {
		parsed, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return true
		}
		leaf = parsed
	}
	return time.Now().Add(24 * time.Hour).After(leaf.NotAfter)
}

func newUUID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16]), nil
}
