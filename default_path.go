package main

import "path/filepath"

func returnPlexDBPath(plexHomePath string) string {
	return filepath.Join(plexHomePath, "Plug-in Support", "Databases", "com.plexapp.plugins.library.db")
}

func returnLogFilePath(logPath string) string {
	return filepath.Join(logPath, "plex-user-sync-go.log")
}
