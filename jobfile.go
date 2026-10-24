package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// JobsFile represents the YAML file for jobs.
type JobsFile struct {
	Jobs []SyncJob `yaml:"jobs"`
}

// SyncJob defines a single sync job.
type SyncJob struct {
	SourceUser      string `yaml:"source_user"`
	TargetUser      string `yaml:"target_user"`
	BiDirectional   bool   `yaml:"bi_directional"`
	IntervalMinutes int    `yaml:"interval_minutes"`
}

// loadJobs loads and validates jobs from the specified YAML file path.
func loadJobs(path string) ([]SyncJob, error) {
	jobs, err := loadAndValidateJobs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load jobs from config '%s': %w", path, err)
	}
	return jobs, nil
}

// loadAndValidateJobs loads jobs from the specified YAML file path and validates them.
func loadAndValidateJobs(path string) ([]SyncJob, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config JobsFile
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	if len(config.Jobs) == 0 {
		return nil, fmt.Errorf("no jobs found")
	}

	for i, job := range config.Jobs {
		if job.SourceUser == "" || job.TargetUser == "" {
			return nil, fmt.Errorf("job %d: source and target usernames are required", i+1)
		}
		if job.SourceUser == job.TargetUser {
			return nil, fmt.Errorf("job %d: source and target usernames cannot be the same", i+1)
		}
		if job.IntervalMinutes <= 0 {
			return nil, fmt.Errorf("job %d: interval must be greater than 0 minutes", i+1)
		}
	}

	return config.Jobs, nil
}
