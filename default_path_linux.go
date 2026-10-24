//go:build linux

package main

import (
	"os"
	"path/filepath"
)

func defaultDBPath() string {

	// Check PLEX_HOME environment variable first
	plexHome := os.Getenv("PLEX_HOME")
	if plexHome != "" {
		plexHome = filepath.Join(plexHome, "Library", "Application Support", "Plex Media Server")
		return returnPlexDBPath(plexHome)
	}

	// Common Linux Plex paths to check if PLEX_HOME is not set
	// Taken from https://support.plex.tv/articles/202915258-where-is-the-plex-media-server-data-directory-located/
	linuxPlexPaths := []string{
		"/var/lib/plexmediaserver/Library/Application Support/Plex Media Server/", // Debian, Fedora, CentOS, Ubuntu
		"/volume1/Plex/Library/",                                                           // ASUSTOR
		"/usr/local/plexdata/Plex Media Server/",                                           // FreeBSD
		"/apps/plexmediaserver/MediaLibrary/Plex Media Server/",                            // ReadyNAS
		"/var/snap/plexmediaserver/common/Library/Application Support/",                    // Snap
		"/volume1/PlexMediaServer/AppData/Plex Media Server/",                              // Synology DSM 7
		"/volume1/Plex/Library/Application Support/Plex Media Server/",                     // Synology DSM 6 / TerraMaster
		"/raid/data/module/Plex/sys/Plex Media Server/",                                    // Thecus
		"/mnt/HD/HD_a2/plex_conf/Plex Media Server/",                                       // Western Digital
		"/shares/Storage/.wdcache/.plexmediaserver/Application Support/Plex Media Server/", // Western Digital My Passport Wireless Pro
	}
	for _, path := range linuxPlexPaths {
		if _, err := os.Stat(path); err == nil {
			return returnPlexDBPath(path)
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
