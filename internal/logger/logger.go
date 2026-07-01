//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dcvix/dcvix-agent/internal/config"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// errIgnoreWriter wraps io.Writer.Write to always return success, so a broken
// stdout (like in a Windows services) doesn't block writes to other writers in
// io.MultiWriter.
type errIgnoreWriter struct{ io.Writer }

// Write always reports success so io.MultiWriter continues to subsequent writers.
func (w errIgnoreWriter) Write(p []byte) (int, error) {
	w.Writer.Write(p)
	return len(p), nil
}

func SetupLogger(cfg config.LogConfig) error {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(cfg.Directory, 0755); err != nil {
		return fmt.Errorf("failed to create log directory %s: %w", cfg.Directory, err)
	}

	// Parse log level
	logLevel, err := logrus.ParseLevel(cfg.Level)
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}

	// Set up log rotation
	logFile := &lumberjack.Logger{
		Filename:   filepath.Join(cfg.Directory, "dcvix-agent.log"),
		MaxSize:    100,          // megabytes
		MaxBackups: cfg.Rotation, // keep at most N rotated files
	}

	// Configure logrus
	logrus.SetLevel(logLevel)
	if logLevel <= logrus.DebugLevel {
		logrus.SetReportCaller(true)
	}
	logrus.SetFormatter(&dcvixFormatter{})
	// errIgnoreWriter ensures a broken stdout doesn't block writes to the log file
	mw := io.MultiWriter(errIgnoreWriter{os.Stdout}, logFile)
	logrus.SetOutput(mw)

	return nil
}
