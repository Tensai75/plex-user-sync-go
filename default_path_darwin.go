//go:build darwin

package main

import (
	"os"
	"path/filepath"
)

func defaultDBPath() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		plexPath := filepath.Join(home, "Library", "Application Support", "Plex Media Server")
		if _, err := os.Stat(plexPath); err == nil {
			return returnPlexDBPath(plexPath)
		}
	}
	return ""
}

func defaultLogFilePath() string {
	logPath := "/var/log"
	if _, err := os.Stat(logPath); err == nil {
		return returnLogFilePath(logPath)
	}
	tempDir := os.TempDir()
	if tempDir != "" {
		if _, err := os.Stat(tempDir); err == nil {
			return returnLogFilePath(tempDir)
		}
	}
	return ""
}
