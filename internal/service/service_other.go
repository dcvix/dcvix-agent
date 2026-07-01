//  SPDX-FileCopyrightText: 2026 Diego Cortassa
//  SPDX-License-Identifier: MIT

//go:build !windows

package service

// InstallService is a stub for non-Windows platforms
func InstallService() error {
	return nil
}

// UninstallService is a stub for non-Windows platforms
func UninstallService() error {
	return nil
}

// RunService is a stub for non-Windows platforms
func RunService() error {
	return nil
}

// IsWindowsService always returns false on non-Windows platforms
func IsWindowsService() (bool, error) {
	return false, nil
}
