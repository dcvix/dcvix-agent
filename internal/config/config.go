//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/ini.v1"
)

type Config struct {
	Agent AgentConfig
	Log   LogConfig
}

type AgentConfig struct {
	DirectorHost   string
	DirectorPort   string
	AgentPort      int
	BindAddress    string
	TLSStrict      bool
	CAFile         string
	CertFile       string
	KeyFile        string
	Tags           []string
	DCVPath        string
	UpdateInterval int
	ReapInterval   int
	DataDir        string
}

type LogConfig struct {
	Level     string
	Directory string
	Rotation  int
}

func getDefaultDCVPath() string {
	if runtime.GOOS == "windows" {
		amazonPath := `C:\Program Files\Amazon\DCV\Server\bin\dcv.exe`
		if _, err := os.Stat(amazonPath); err == nil {
			return amazonPath
		}
		return `C:\Program Files\NICE\DCV\Server\bin\dcv.exe`
	}
	return "/usr/bin/dcv"
}

func getExecutablePath() (string, error) {
	ex, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not get executable path: %w", err)
	}
	return filepath.Dir(ex), nil
}

func getProgramData() string {
	progData := os.Getenv("ProgramData")
	if progData == "" {
		progData = `C:\ProgramData`
	}
	return progData
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func findConfig(configPath string) (string, error) {
	if configPath != "" {
		return configPath, nil
	}

	if runtime.GOOS == "windows" {
		path := filepath.Join(getProgramData(), "dcvix", "Agent", "dcvix-agent.conf")
		if fileExists(path) {
			return path, nil
		}
	} else {
		path := "/etc/dcvix-agent/dcvix-agent.conf"
		if fileExists(path) {
			return path, nil
		}

		execPath, err := getExecutablePath()
		if err == nil {
			fallbackPath := filepath.Join(execPath, "dcvix-agent.conf")
			if fileExists(fallbackPath) {
				return fallbackPath, nil
			}
		}
	}

	if fileExists("./dcvix-agent.conf") {
		return "./dcvix-agent.conf", nil
	}

	return "", fmt.Errorf("configuration file not found")
}

func loadEmbeddedConfig() (*Config, error) {
	defaultFile, err := ini.Load(defaultConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded default config: %w", err)
	}

	dAgent := defaultFile.Section("agent")
	dLog := defaultFile.Section("log")

	return &Config{
		Agent: AgentConfig{
			DirectorHost:   dAgent.Key("director_host").String(),
			DirectorPort:   dAgent.Key("director_port").String(),
			AgentPort:      dAgent.Key("agent_port").MustInt(8446),
			BindAddress:    dAgent.Key("bind_address").String(),
			Tags:           parseTagList(dAgent.Key("tags").String()),
			DCVPath:        getDefaultDCVPath(),
			UpdateInterval: dAgent.Key("update_interval").MustInt(30),
			ReapInterval:   dAgent.Key("reap_interval").MustInt(60),
			DataDir:        dAgent.Key("data_dir").String(),
		},
		Log: LogConfig{
			Level:     dLog.Key("level").String(),
			Directory: dLog.Key("directory").String(),
			Rotation:  dLog.Key("rotation").MustInt(2),
		},
	}, nil
}

func parseTagList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func LoadConfig(configPath string) (*Config, error) {
	cfg, err := loadEmbeddedConfig()
	if err != nil {
		return nil, err
	}

	configPath, err = findConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to locate config file: %w", err)
	}

	file, err := ini.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Load Agent section
	agentSection := file.Section("agent")
	if agentSection.HasKey("director_host") {
		cfg.Agent.DirectorHost = agentSection.Key("director_host").String()
	}
	if cfg.Agent.DirectorHost == "" {
		return nil, fmt.Errorf("failed to load config: director_host is required")
	}
	if agentSection.HasKey("director_port") {
		cfg.Agent.DirectorPort = agentSection.Key("director_port").String()
	}
	cfg.Agent.AgentPort = agentSection.Key("agent_port").MustInt(cfg.Agent.AgentPort)
	if agentSection.HasKey("bind_address") {
		cfg.Agent.BindAddress = agentSection.Key("bind_address").String()
	}
	if agentSection.HasKey("tags") {
		cfg.Agent.Tags = parseTagList(agentSection.Key("tags").String())
	}
	if agentSection.HasKey("dcv_path") {
		cfg.Agent.DCVPath = agentSection.Key("dcv_path").String()
	}
	cfg.Agent.UpdateInterval = agentSection.Key("update_interval").MustInt(cfg.Agent.UpdateInterval)
	cfg.Agent.ReapInterval = agentSection.Key("reap_interval").MustInt(cfg.Agent.ReapInterval)
	if agentSection.HasKey("data_dir") {
		cfg.Agent.DataDir = agentSection.Key("data_dir").String()
	}

	// Load Log section
	logSection := file.Section("log")
	if logSection.HasKey("level") {
		cfg.Log.Level = logSection.Key("level").String()
	}
	if logSection.HasKey("directory") {
		cfg.Log.Directory = logSection.Key("directory").String()
	}
	cfg.Log.Rotation = logSection.Key("rotation").MustInt(cfg.Log.Rotation)

	log.Infof("Loaded configuration file: %s", configPath)

	// Platform-aware default for data_dir
	if cfg.Agent.DataDir == "" {
		if runtime.GOOS == "windows" {
			cfg.Agent.DataDir = filepath.Join(getProgramData(), "dcvix", "Agent")
		} else {
			cfg.Agent.DataDir = "/var/lib/dcvix-agent"
		}
	}

	// Add default OS tag
	cfg.Agent.Tags = append(cfg.Agent.Tags, runtime.GOOS)

	// Platform-aware default for log directory
	if cfg.Log.Directory == "" {
		if runtime.GOOS == "windows" {
			cfg.Log.Directory = filepath.Join(getProgramData(), "dcvix", "Agent", "log")
		} else {
			cfg.Log.Directory = "/var/log/dcvix-agent"
		}
	}

	return cfg, nil
}
