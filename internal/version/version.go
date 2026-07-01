//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

package version

import "fmt"

var (
	// Vars set up at build time via "-ldflags"
	// Version holds the current version injected at build time from git tag
	Version = "dev"
	// Commit holds the git commit hash
	Commit = "none"
	// BuildTime holds the build timestamp
	BuildTime = "unknown"
)

func String() string {
	return fmt.Sprintf("%s (commit: %s, built at: %s)", Version, Commit, BuildTime)
}
