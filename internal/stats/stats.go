//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

// Package stats collects system stats.

package stats

import (
	"os"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

type SystemStats struct {
	Hostname    string  `json:"hostname"`
	FreeMemory  uint64  `json:"free_memory"`
	TotalMemory uint64  `json:"total_memory"`
	Cores       int     `json:"cores"`
	CPUUsage    float64 `json:"cpu_usage"`
	Load1       float64 `json:"load1"`
	Load5       float64 `json:"load5"`
	Load15      float64 `json:"load15"`
}

func CollectStats() (*SystemStats, error) {
	stats := &SystemStats{}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	stats.Hostname = hostname

	// Get memory stats
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	stats.FreeMemory = vmStat.Free
	stats.TotalMemory = vmStat.Total

	// Get CPU cores
	cpuCores, err := cpu.Counts(false)
	if err != nil {
		return nil, err
	}
	stats.Cores = int(cpuCores)

	// Get CPU stats
	cpuPercent, err := cpu.Percent(1*time.Second, false)
	if err != nil {
		return nil, err
	}
	if len(cpuPercent) > 0 {
		stats.CPUUsage = cpuPercent[0]
	}

	// Get CPU load average
	loadAvg, err := load.Avg()
	if err != nil {
		return nil, err
	}
	stats.Load1 = loadAvg.Load1
	stats.Load5 = loadAvg.Load5
	stats.Load15 = loadAvg.Load15

	return stats, nil
}
