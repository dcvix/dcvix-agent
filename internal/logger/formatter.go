// SPDX-FileCopyrightText: 2026 Diego Cortassa
// SPDX-License-Identifier: MIT

package logger

import (
	"bytes"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type dcvixFormatter struct{}

func (f *dcvixFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	level := strings.ToUpper(entry.Level.String())
	timestamp := entry.Time.Format(time.RFC3339)

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("%-5s[%s] ", level, timestamp))
	if entry.Caller != nil {
		buf.WriteString(fmt.Sprintf("%s:%d  ", path.Base(entry.Caller.File), entry.Caller.Line))
	}
	buf.WriteString(strings.TrimSuffix(entry.Message, "\n"))
	buf.WriteString("\n")

	return buf.Bytes(), nil
}
