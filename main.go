package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Job struct {
	plexJob         *PlexSyncJob
	intervalMinutes int
	maxErrors       int
}

// Global variables
var (
	appName            = "Plex User Sync Go" // Application name
	appVersion         = "1.0.0"             // Application version
	Log                = NewLogger()         // Global logger instance
	daemonSyncInterval = 5                   // Default daemon sync interval in minutes
	maxErrors          = 3                   // Default maximum number of consecutive errors before stopping the daemon
	busyTimeout        = 60                  // Default database busy timeout in seconds
	appExeName         = filepath.Base(os.Args[0])
)

func main() {
	// Parse command-line flags and configuration
	config, err := ParseFlags()
	if err != nil {
		Log.Fatal("failed to parse flags: %v", err)
	}

	// Initialize the logger if logging is enabled
	if config.Log && config.LogFile != "" {
		err = Log.SetLogFile(config.LogFile)
		if err != nil {
			Log.Fatal("failed to initialize logger: %v", err)
		}
		defer Log.Close()
	}

	// Run the main application logic and handle any errors
	if err := run(config); err != nil {
		Log.Fatal("%v", err)
	}
}

// run executes the main application logic
// Don't use os.Exit() here, as it will prevent deferred functions from running.
func run(config *Config) error {
	// Log the application start message
	Log.Info("%s %s started", appName, appVersion)

	// Create the PlexDatabase instance
	plexDB, err := NewPlexDatabase(config.DBPath, config.BusyTimeoutSeconds*1000) // Convert seconds to milliseconds
	if err != nil {
		return fmt.Errorf("failed to create Plex database instance: %w", err)
	}

	// Connect to the Plex database
	err = plexDB.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to Plex database: %w", err)
	}
	defer plexDB.Close()

	// Prepare the list of jobs to run based on the configuration
	jobs, err := prepareJobs(config, plexDB)
	if err != nil {
		return err
	}

	// Validate that there are jobs to run
	if len(jobs) == 0 {
		return fmt.Errorf("no jobs to run")
	}

	// Run the jobs based on whether daemon mode is enabled
	if config.Daemon {
		return runDaemon(jobs)
	}
	return runJobs(jobs)
}

func prepareJobs(config *Config, plexDB *PlexDatabase) ([]*Job, error) {
	if config.JobsConfig == "" {
		// If no jobs config file is defined in the config, create a single job based on the command-line arguments
		jobConfig := &PlexSyncJobConfig{
			JobName:       fmt.Sprintf("%s %s %s%s", config.SourceUser, directionIcon(config.BiDirectional), config.TargetUser, isBulkMode(config.BulkMode)),
			SourceUser:    config.SourceUser,
			TargetUser:    config.TargetUser,
			BiDirectional: config.BiDirectional,
			BulkMode:      config.BulkMode,
			DryRun:        config.DryRun,
		}
		plexJob, err := NewPlexSyncJob(jobConfig, plexDB)
		if err != nil {
			return nil, fmt.Errorf("failed to create sync job '%s': %w", jobConfig.JobName, err)
		}
		return []*Job{{plexJob: plexJob, intervalMinutes: config.IntervalMinutes, maxErrors: config.MaxErrors}}, nil
	}

	// If a jobs config file is defined in the config, create a job for each entry
	plexJobs := []*Job{}
	jobs, err := loadJobs(config.JobsConfig)
	if err != nil {
		return nil, err
	}
	for _, jobConfig := range jobs {
		plexJobConfig := &PlexSyncJobConfig{
			JobName:       fmt.Sprintf("%s %s %s%s", jobConfig.SourceUser, directionIcon(jobConfig.BiDirectional), jobConfig.TargetUser, isBulkMode(config.BulkMode)),
			SourceUser:    jobConfig.SourceUser,
			TargetUser:    jobConfig.TargetUser,
			BiDirectional: jobConfig.BiDirectional,
			BulkMode:      config.BulkMode,
			DryRun:        config.DryRun,
		}
		plexJob, err := NewPlexSyncJob(plexJobConfig, plexDB)
		if err != nil {
			return nil, fmt.Errorf("failed to create sync job '%s': %w", plexJobConfig.JobName, err)
		}
		plexJobs = append(plexJobs, &Job{plexJob: plexJob, intervalMinutes: jobConfig.IntervalMinutes, maxErrors: config.MaxErrors})
	}
	return plexJobs, nil
}

// runDaemon executes the provided jobs in daemon mode, running each job at its specified interval.
func runDaemon(jobs []*Job) error {

	var daemonWG sync.WaitGroup

	// Start a goroutine for each job to run it in daemon mode
	for _, job := range jobs {
		daemonWG.Add(1)
		Log.Info("%sstarting daemon for sync job '%s'", isDryRun(job.plexJob.DryRun), job.plexJob.JobName)

		go func(job *Job) {
			defer daemonWG.Done()
			errorsCount := 0
			time.Sleep(10 * time.Second) // Initial delay to allow other daemons to start

			// Run the job in a loop, executing it at the specified interval
			for {
				err := runJob(job.plexJob)
				if err != nil {
					errorsCount++
					if errorsCount >= job.maxErrors {
						Log.Error("max errors reached for sync job '%s', stopping daemon", job.plexJob.JobName)
						return
					}
				}
				errorsCount = 0 // Reset error count on successful run
				Log.Info("%swaiting %d minutes before next run of sync job '%s'", isDryRun(job.plexJob.DryRun), job.intervalMinutes, job.plexJob.JobName)
				time.Sleep(time.Duration(job.intervalMinutes) * time.Minute)
			}
		}(job)
	}

	// Wait for all daemon jobs to complete (which will only happen if they stop due to errors)
	// This will block indefinitely unless all jobs stop due to reaching max errors
	daemonWG.Wait()
	return fmt.Errorf("all daemon jobs have stopped due to reaching max errors")
}

// runJobs executes multiple PlexSyncJob instances and returns an error if any job fails
func runJobs(jobs []*Job) error {
	// Track failed jobs for reporting
	failedJobs := 0

	// Execute each job and collect any failures
	for _, job := range jobs {
		err := runJob(job.plexJob)
		if err != nil {
			failedJobs++
		}
	}
	if failedJobs > 0 {
		return fmt.Errorf("%d of %d sync job%s failed", failedJobs, len(jobs), pluralSuffix(int64(len(jobs))))
	}
	return nil
}

// runJob executes a single PlexSyncJob and logs the results, including handling dry run scenarios.
func runJob(job *PlexSyncJob) error {
	Log.Info("%srunning sync job '%s'", isDryRun(job.DryRun), job.JobName)

	// Execute the sync job and capture the statistics and any errors
	stats, err := job.Run()
	if err != nil {
		Log.Error("%ssync job '%s' failed: %v", isDryRun(job.DryRun), job.JobName, err)
		return err
	}
	Log.Info("%ssync job '%s' completed", isDryRun(job.DryRun), job.JobName)

	// Handle dry run results by logging the number of items that would have been synced without making any changes
	if job.DryRun {
		Log.Info("DRY RUN: %d watch item%s would have been synced to target user '%s'", stats.ToSyncToTarget, pluralSuffix(stats.ToSyncToTarget), job.TargetUser)
		if job.BiDirectional {
			Log.Info("DRY RUN: %d watch item%s would have been synced to source user '%s'", stats.ToSyncToSource, pluralSuffix(stats.ToSyncToSource), job.SourceUser)
		}
		Log.Info("DRY RUN: no changes were made")
		return nil
	}

	// Log the sync results for the job
	Log.Info("%d of %d watch items synced to target user '%s'", stats.SyncedToTarget, stats.ToSyncToTarget, job.TargetUser)
	if job.BiDirectional {
		Log.Info("%d of %d watch items synced to source user '%s'", stats.SyncedToSource, stats.ToSyncToSource, job.SourceUser)
	}
	return nil
}

// Helper functions

// directionIcon returns a string representation of the sync direction based on the biDirectional flag.
func directionIcon(biDirectional bool) string {
	if biDirectional {
		return "<->"
	}
	return "->"
}

func isDryRun(dryRun bool) string {
	if dryRun {
		return "DRY RUN: "
	}
	return ""
}

func isBulkMode(bulkMode bool) string {
	if bulkMode {
		return " (bulk copy)"
	}
	return ""
}
func pluralSuffix(count int64) string {
	if count == 1 {
		return ""
	}
	return "s"
}
