//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

package dcv

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const commandTimeout = 30 * time.Second

// NullableTime wraps time.Time to support JSON null values during unmarshalling.
type NullableTime struct {
	time.Time
}

const ctLayout = "2006-01-02T15:04:05.000000Z" // parse string layout for time

// UnmarshalJSON implements json.Unmarshaler for NullableTime, accepting null or
// timestamps in the format "2006-01-02T15:04:05.000000Z".
func (ct *NullableTime) UnmarshalJSON(b []byte) (err error) {
	s := strings.Trim(string(b), "\"")
	if s == "null" || s == "" {
		ct.Time = time.Time{}
		return
	}
	ct.Time, err = time.Parse(ctLayout, s)
	return
}

// Session represents a DCV session returned by the list-sessions command.
type Session struct {
	ID                    string       `json:"id"`
	Owner                 string       `json:"owner"`
	User                  string       `json:"user"`
	NumOfConnections      int          `json:"num-of-connections"`
	CreationTime          NullableTime `json:"creation-time"`
	LastDisconnectionTime NullableTime `json:"last-disconnection-time"`
	// Licenses              []struct {
	// 	Product        string    `json:"product"`
	// 	Status         string    `json:"status"`
	// 	CheckTimestamp time.Time `json:"check-timestamp"`
	// 	ExpirationDate time.Time `json:"expiration-date"`
	// } `json:"licenses"`
	LicensingMode string `json:"licensing-mode"`
	StorageRoot   string `json:"storage-root"`
	Type          string `json:"type"`
	Status        string `json:"status"`
	X11Display    string `json:"x11-display"`
	X11Authority  string `json:"x11-authority"`
	// DisplayLayout []struct {
	// 	Width  int `json:"width"`
	// 	Height int `json:"height"`
	// 	X      int `json:"x"`
	// 	Y      int `json:"y"`
	// } `json:"display-layout"`
}

// DCVManager wraps the DCV CLI binary and provides methods to manage DCV sessions.
type DCVManager struct {
	dcvPath string
}

// NewDCVManager creates a new DCVManager that uses the given path to the dcv CLI binary.
func NewDCVManager(dcvPath string) *DCVManager {
	return &DCVManager{
		dcvPath: dcvPath,
	}
}

// ListSessions returns all DCV sessions by running the list-sessions command.
func (d *DCVManager) ListSessions() ([]Session, error) {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, d.dcvPath, "list-sessions", "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %s: %w", output, err)
	}

	var sessions []Session
	if err := json.Unmarshal(output, &sessions); err != nil {
		return nil, fmt.Errorf("failed to parse sessions output: %w", err)
	}

	return sessions, nil
}

// CreateSession creates a new DCV session with the given owner, type, and session ID.
func (d *DCVManager) CreateSession(userID, sessionType, sessionID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	log.Debugf("Running command: %s create-session --owner %s --type %s %s", d.dcvPath, userID, sessionType, sessionID)
	cmd := exec.CommandContext(ctx, d.dcvPath, "create-session", "--owner", userID, "--type", sessionType, sessionID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create session: %s: %w", output, err)
	}
	return nil
}

// CloseSession closes the DCV session with the given ID.
func (d *DCVManager) CloseSession(SessionID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	log.Debugf("Running command: %s close-session %s", d.dcvPath, SessionID)
	cmd := exec.CommandContext(ctx, d.dcvPath, "close-session", SessionID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to close session: %s: %w", output, err)
	}
	return nil
}

// SetConfig sets a DCV configuration key in the specified section to the given value.
func (d *DCVManager) SetConfig(section, key, value string) error {
	if section == "" || key == "" || value == "" {
		return fmt.Errorf("section, key and value must not be empty")
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	log.Debugf("Running command: %s set-config --section %s --key %s %s", d.dcvPath, section, key, value)
	cmd := exec.CommandContext(ctx, d.dcvPath, "set-config", "--section", section, "--key", key, value)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set config key %s/%s: %s: %w", section, key, output, err)
	}
	return nil
}
