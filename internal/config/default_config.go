//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

package config

import _ "embed"

//go:embed dcvix-agent.conf.default
var defaultConfig []byte // Embed the file content as a byte slice
