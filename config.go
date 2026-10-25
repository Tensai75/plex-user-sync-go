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
	LogToFile          bool
	LogFile            string
	LogLevel           int
	LogMaxSizeMB       int
	LogMaxBackups      int
	LogMaxAgeDays      int
	LogCompress        bool
	LogToConsole       bool
	JobsConfig         string
	BusyTimeoutSeconds int
}

// ParseFlags parses command line arguments and returns Config
func ParseFlags() (*Config, error) {
	cfg := &Config{}

	pflag.StringVarP(&cfg.DBPath, "db", "d", dbPath, "Database path")
	pflag.BoolVarP(&cfg.DryRun, "dry-run", "n", dryRun, "Preview only")
	pflag.BoolVarP(&cfg.BiDirectional, "bi-directional", "b", biDirectional, "Sync both directions (newer status wins)")
	pflag.BoolVarP(&cfg.BulkMode, "bulk", "k", bulkMode, "Bulk copy sync (deletes target watch history)")
	pflag.BoolVarP(&cfg.Daemon, "daemon", "D", daemon, "Run as daemon and sync every X seconds")
	pflag.IntVarP(&cfg.IntervalMinutes, "interval", "i", daemonSyncInterval, "Daemon sync interval in minutes")
	pflag.IntVarP(&cfg.MaxErrors, "max-errors", "e", maxErrors, "Maximum number of consecutive errors before stopping the daemon")
	pflag.StringVarP(&cfg.JobsConfig, "jobs-config", "j", "", "YAML file containing jobs to sync")
	pflag.BoolVarP(&cfg.LogToFile, "log", "l", logToFile, "Enable logging to file")
	pflag.StringVarP(&cfg.LogFile, "log-file", "f", logFilePath, "Path to log file")
	pflag.IntVarP(&cfg.LogLevel, "log-level", "L", logLevel, "Log level (0 = none, 1 = debug, 2 = info, 3 = warn, 4 = success, 5 = error, 6 = fatal)")
	pflag.IntVar(&cfg.LogMaxSizeMB, "log-max-size", logMaxSizeMB, "Maximum log file size in MB; 0 disables rotation")
	pflag.IntVar(&cfg.LogMaxBackups, "log-max-backups", logMaxBackups, "Maximum number of rotated log files to keep")
	pflag.IntVar(&cfg.LogMaxAgeDays, "log-max-age", logMaxAgeDays, "Maximum age in days for rotated log files")
	pflag.BoolVar(&cfg.LogCompress, "log-compress", logCompress, "Compress rotated log files")
	pflag.BoolVar(&cfg.LogToConsole, "log-console", logToConsole, "Write logs to console")
	pflag.IntVar(&cfg.BusyTimeoutSeconds, "busy-timeout", busyTimeout, "SQLite busy timeout in seconds")

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
	if cfg.LogToFile && cfg.LogFile == "" {
		pflag.Usage()
		return nil, fmt.Errorf("could not determine default log file path; --log-file is required when logging is enabled")
	}

	// Validate that the log level is within the valid range
	if cfg.LogLevel < 0 || cfg.LogLevel > 6 {
		pflag.Usage()
		return nil, fmt.Errorf("invalid log level: must be between 0 and 6")
	}

	// Validate that the log rotation limits
	if cfg.LogToFile && (cfg.LogMaxSizeMB < 0 || cfg.LogMaxBackups < 0 || cfg.LogMaxAgeDays < 0) {
		pflag.Usage()
		return nil, fmt.Errorf("log rotation limits cannot be negative")
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

	// Ensure that log file is empty if logging to file is not enabled
	if !cfg.LogToFile {
		cfg.LogFile = ""
	}

	return cfg, nil
}
