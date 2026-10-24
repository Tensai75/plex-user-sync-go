//go:build windows

package main

import (
	"os"
	"path/filepath"
)

func defaultDBPath() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		return returnPlexDBPath(filepath.Join(localAppData, "Plex Media Server"))
	}
	return ""
}

func defaultLogFilePath() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		if _, err := os.Stat(localAppData); err == nil {
			return returnLogFilePath(filepath.Join(localAppData, "PlexUserSyncGo"))
		}
	}
	tempDir := os.TempDir()
	if tempDir != "" {
		if _, err := os.Stat(tempDir); err == nil {
			return returnLogFilePath(tempDir)
		}
	}
	return ""
}
