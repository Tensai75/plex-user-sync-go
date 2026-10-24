package main

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"
)

// Config holds all configuration options
type Config struct {
	DBPath             string
	SourceUser         string
	TargetUser         string
	DryRun             bool
	BiDirectional      bool
	BulkMode           bool
	Daemon             bool
	IntervalMinutes    int
	MaxErrors          int
	Log                bool
	LogFile            string
	JobsConfig         string
	BusyTimeoutSeconds int
}

// ParseFlags parses command line arguments and returns Config
func ParseFlags() (*Config, error) {
	cfg := &Config{}

	pflag.StringVarP(&cfg.DBPath, "db", "d", DefaultDBPath, "Database path")
	pflag.BoolVarP(&cfg.DryRun, "dry-run", "n", false, "Preview only")
	pflag.BoolVarP(&cfg.BiDirectional, "bi-directional", "b", false, "Sync both directions (newer status wins)")
	pflag.BoolVarP(&cfg.BulkMode, "bulk", "k", false, "Bulk copy sync (deletes target watch history)")
	pflag.BoolVarP(&cfg.Daemon, "daemon", "D", false, "Run as daemon and sync every X seconds")
	pflag.IntVarP(&cfg.IntervalMinutes, "interval", "i", daemonSyncInterval, "Daemon sync interval in minutes")
	pflag.IntVarP(&cfg.MaxErrors, "max-errors", "e", maxErrors, "Maximum number of consecutive errors before stopping the daemon")
	pflag.BoolVarP(&cfg.Log, "log", "l", false, "Enable logging to file")
	pflag.StringVarP(&cfg.LogFile, "log-file", "f", DefaultLogFilePath, "Path to log file")
	pflag.StringVarP(&cfg.JobsConfig, "jobs-config", "j", "", "YAML file containing jobs to sync")
	pflag.IntVarP(&cfg.BusyTimeoutSeconds, "busy-timeout", "t", busyTimeout, "SQLite busy timeout in seconds")

	pflag.Usage = func() {
		fmt.Printf("%s %s\n", appName, appVersion)
		fmt.Printf("Sync Plex watch history between users\n\n")
		fmt.Printf("Usage: %s [options] [<source_user> <target_user>]\n\n", appExeName)
		fmt.Printf("Options:\n")
		pflag.PrintDefaults()
		fmt.Printf("\nExamples:\n")
		fmt.Printf("  %s user1 user2              # Directional sync: user1 -> user2\n", appExeName)
		fmt.Printf("  %s -n user1 user2           # Directional sync: user1 -> user2 (dry run)\n", appExeName)
		fmt.Printf("  %s -b user1 user2           # Bi-directional sync: user1 <-> user2\n", appExeName)
		fmt.Printf("  %s -k user1 user2           # Bulk copy sync: user1 -> user2 (deletes target watch history of user2)\n", appExeName)
		fmt.Printf("  %s -j jobs.yaml             # One-time sync with YAML jobs config file\n", appExeName)
		fmt.Printf("  %s -n -j jobs.yaml          # One-time sync with YAML jobs config file (dry run)\n", appExeName)
		fmt.Printf("  %s -D -i 5 user1 user2      # Daemon mode: directional sync every 5 minutes\n", appExeName)
		fmt.Printf("  %s -D -i 10 -b user1 user2  # Daemon mode: bi-directional sync every 10 minutes\n", appExeName)
		fmt.Printf("  %s -D -l -j jobs.yaml       # Daemon mode with YAML jobs config file and logging enabled\n", appExeName)
		fmt.Printf("  %s -n -D -j jobs.yaml       # Daemon mode with YAML jobs config file (dry run)\n\n", appExeName)
	}

	if err := pflag.CommandLine.Parse(os.Args[1:]); err != nil {
		pflag.Usage()
		return nil, err
	}

	// Validate positional arguments based on whether jobs config is provided or not
	// Positional args policy:
	// - with --jobs-config: 0 args
	// - without --jobs-config: exactly 2 args (source_user target_user)
	args := pflag.Args()
	if cfg.JobsConfig != "" {
		if len(args) != 0 {
			pflag.Usage()
			return nil, fmt.Errorf("no positional usernames are allowed when --jobs-config is set")
		}
	} else {
		if len(args) != 2 {
			pflag.Usage()
			return nil, fmt.Errorf("source and target usernames are required when --jobs-config is not set")
		}
		if args[0] == args[1] {
			pflag.Usage()
			return nil, fmt.Errorf("source and target usernames cannot be the same")
		}
		cfg.SourceUser = args[0]
		cfg.TargetUser = args[1]
	}

	// Validate that the database path exists
	if cfg.DBPath == "" {
		pflag.Usage()
		return nil, fmt.Errorf("could not determine default Plex database path; --db is required")
	}

	// Validate that the log file path exists
	if cfg.Log && cfg.LogFile == "" {
		pflag.Usage()
		return nil, fmt.Errorf("could not determine default log file path; --log-file is required when logging is enabled")
	}

	// Validate that the daemon interval is greater than 0 seconds when running in daemon mode
	if cfg.Daemon && cfg.IntervalMinutes <= 0 {
		pflag.Usage()
		return nil, fmt.Errorf("daemon interval must be greater than 0 minutes")
	}

	// Validate that the maximum number of errors is greater than 0 when running in daemon mode
	if cfg.Daemon && cfg.MaxErrors <= 0 {
		pflag.Usage()
		return nil, fmt.Errorf("maximum number of errors must be greater than 0 when running in daemon mode")
	}

	// Validate that the busy timeout is greater than zero or zero (non-negative)
	if cfg.BusyTimeoutSeconds < 0 {
		pflag.Usage()
		return nil, fmt.Errorf("busy timeout must be zero or greater")
	}

	// Validate incompatible options
	if cfg.BulkMode && cfg.BiDirectional {
		pflag.Usage()
		return nil, fmt.Errorf("bulk mode cannot be used with bi-directional sync")
	}
	if cfg.BulkMode && cfg.Daemon {
		pflag.Usage()
		return nil, fmt.Errorf("bulk mode cannot be used in daemon mode")
	}

	return cfg, nil
}
