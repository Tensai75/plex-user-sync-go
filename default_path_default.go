//go:build !linux && !darwin && !windows

package main

func defaultDBPath() string {
	return ""
}

func defaultLogFilePath() string {
	return ""
}
