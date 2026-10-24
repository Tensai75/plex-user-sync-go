package main

import (
	"database/sql"
	"fmt"
	"sync"
)

var syncMutex = sync.Mutex{} // Mutex to prevent concurrent database access

// PlexSyncJob handles watch history synchronization
type PlexSyncJob struct {
	JobName       string        // Name of the sync job
	SourceUser    string        // Source user for sync
	TargetUser    string        // Target user for sync
	SourceID      int64         // Resolved source account ID
	TargetID      int64         // Resolved target account ID
	BiDirectional bool          // Flag for bi-directional sync
	BulkMode      bool          // Flag for bulk copy sync
	DryRun        bool          // Flag for dry run mode (no changes made)
	Stats         SyncStats     // Statistics for the sync operation
	PlexDB        *PlexDatabase // Plex database connection
}

// SyncStats holds statistics for a sync operation
type SyncStats struct {
	TotalSource        int64 // Total items in source
	TotalTarget        int64 // Total items in target
	ToSyncToTarget     int64 // Items to be synced to target
	ToSyncToSource     int64 // Items to be synced to source
	SyncedToTarget     int64 // Items successfully synced to target
	SyncedToSource     int64 // Items successfully synced to source
	FailedSyncToTarget int64 // Items failed to sync to target
	FailedSyncToSource int64 // Items failed to sync to source
}

// PlexSyncJobConfig holds configuration for a PlexSyncJob
type PlexSyncJobConfig struct {
	JobName       string // Name of the sync job
	SourceUser    string // Source user for sync
	TargetUser    string // Target user for sync
	BiDirectional bool   // Flag for bi-directional sync
	BulkMode      bool   // Flag for bulk copy sync
	DryRun        bool   // Flag for dry run mode (no changes made)
}

// SyncTask represents a single sync operation
type SyncTask struct {
	GUID     string // The GUID of the watch history item to be synced
	SourceID int64  // The source account ID for the item
	TargetID int64  // The target account ID to which the item will be synced
}

// WatchItem represents a watched item with all metadata
type WatchItem struct {
	// Metadata Item Settings fields
	ItemID        int             `db:"item_id"`
	GUID          string          `db:"guid"`
	Rating        sql.NullFloat64 `db:"rating"`
	ViewOffset    sql.NullInt64   `db:"view_offset"`
	ViewCount     sql.NullInt64   `db:"view_count"`
	LastViewedAt  sql.NullInt64   `db:"last_viewed_at"`
	CreatedAt     sql.NullInt64   `db:"created_at"`
	UpdatedAt     sql.NullInt64   `db:"updated_at"`
	SkipCount     sql.NullInt64   `db:"skip_count"`
	LastSkippedAt sql.NullInt64   `db:"last_skipped_at"`
	ChangedAt     sql.NullInt64   `db:"changed_at"`
	ExtraData     sql.NullString  `db:"extra_data"`
	LastRatedAt   sql.NullInt64   `db:"last_rated_at"`
	// Metadata Item fields
	MetadataID       sql.NullInt64  `db:"metadata_id"`
	Title            sql.NullString `db:"title"`
	MetadataType     sql.NullInt64  `db:"metadata_type"`
	Year             sql.NullInt64  `db:"year"`
	LibrarySectionID sql.NullInt64  `db:"library_section_id"`
	Duration         sql.NullInt64  `db:"duration"`
}

// NewPlexSyncJob creates a new PlexSyncJob instance with the provided configuration and database connection
func NewPlexSyncJob(config *PlexSyncJobConfig, db *PlexDatabase) (*PlexSyncJob, error) {
	job := &PlexSyncJob{
		JobName:       config.JobName,
		PlexDB:        db,
		SourceUser:    config.SourceUser,
		TargetUser:    config.TargetUser,
		BiDirectional: config.BiDirectional,
		BulkMode:      config.BulkMode,
		DryRun:        config.DryRun,
	}

	// Resolve account IDs for source and target users
	err := job.getAccountIds()
	if err != nil {
		return nil, err
	}

	// Return the initialized PlexSyncJob instance
	return job, nil
}

// Run executes the sync job, performing the appropriate sync operation based on the configuration
func (p *PlexSyncJob) Run() (SyncStats, error) {
	err := p.run()
	return p.Stats, err
}

// getAccountIds resolves the account IDs for the source and target users
func (p *PlexSyncJob) getAccountIds() error {
	// Lock to prevent concurrent DB access
	syncMutex.Lock()
	defer syncMutex.Unlock()

	// Retrieve accounts from the Plex database
	accounts, err := p.PlexDB.GetAccounts()
	if err != nil {
		return fmt.Errorf("failed to get accounts: %w", err)
	}

	// Check if source and target users exist in the accounts map
	// and assign their corresponding account IDs to the PlexSyncJob instance
	var ok bool
	p.SourceID, ok = accounts[p.SourceUser]
	if !ok {
		return fmt.Errorf("source '%s' not found. Available: %v",
			p.SourceUser, getKeys(accounts))
	}
	p.TargetID, ok = accounts[p.TargetUser]
	if !ok {
		return fmt.Errorf("target '%s' not found. Available: %v",
			p.TargetUser, getKeys(accounts))
	}

	return nil
}

// run executes the full sync operation
func (p *PlexSyncJob) run() error {
	// Lock to prevent concurrent DB access
	syncMutex.Lock()
	defer syncMutex.Unlock()

	// Reset statistics for the sync operation
	p.Stats = SyncStats{}

	// Execute appropriate sync mode
	if p.BulkMode {
		return p.bulkCopy()
	}
	return p.standardSync()
}

// bulkCopy performs a bulk copy sync from source to target
func (p *PlexSyncJob) bulkCopy() error {
	// Get source data count
	var err error
	p.Stats.TotalSource, err = p.PlexDB.CountWatched(p.SourceID)
	p.Stats.ToSyncToTarget = p.Stats.TotalSource
	if err != nil {
		return err
	}

	// If dryrun, return without making changes
	if p.DryRun {
		return nil
	}

	// Perform bulk copy
	p.Stats.SyncedToTarget, err = p.PlexDB.BulkCopyWatchItems(p.SourceID, p.TargetID)
	if err != nil {
		return err
	}

	return nil
}

// standardSync performs a standard sync operation between source and target
func (p *PlexSyncJob) standardSync() error {
	// Get source data
	sourceItems, err := p.PlexDB.GetWatched(p.SourceID)
	if err != nil {
		return err
	}
	p.Stats.TotalSource = int64(len(sourceItems))

	// Get target data
	var targetItems []WatchItem
	targetItems, err = p.PlexDB.GetWatched(p.TargetID)
	if err != nil {
		return err
	}
	p.Stats.TotalTarget = int64(len(targetItems))

	// Build lookup maps
	sourceByGUID := make(map[string]WatchItem)
	for _, item := range sourceItems {
		if item.GUID != "" {
			sourceByGUID[item.GUID] = item
		}
	}
	targetByGUID := make(map[string]WatchItem)
	for _, item := range targetItems {
		if item.GUID != "" {
			targetByGUID[item.GUID] = item
		}
	}

	// Determine sync tasks for both directions
	toSyncToTarget, toSyncToSource := p.determineSyncTasks(sourceByGUID, targetByGUID)
	p.Stats.ToSyncToTarget = int64(len(toSyncToTarget))
	p.Stats.ToSyncToSource = int64(len(toSyncToSource))

	// If dryrun, return without making changes
	if p.DryRun {
		return nil
	}

	// Perform sync to target
	if len(toSyncToTarget) > 0 {
		written, failed, err := p.performSync(toSyncToTarget)
		p.Stats.SyncedToTarget = written
		p.Stats.FailedSyncToTarget = failed
		if err != nil {
			return err
		}
	}

	// Perform sync to source if bi-directional
	if p.BiDirectional && len(toSyncToSource) > 0 {
		written, failed, err := p.performSync(toSyncToSource)
		p.Stats.SyncedToSource = written
		p.Stats.FailedSyncToSource = failed
		if err != nil {
			return err
		}
	}

	return nil

}

// determineSyncTasks determines the sync tasks for both directions based on source and target items
func (p *PlexSyncJob) determineSyncTasks(sourceByGUID, targetByGUID map[string]WatchItem) ([]SyncTask, []SyncTask) {
	toSyncToTarget := make([]SyncTask, 0)
	for guid, item := range sourceByGUID {
		targetItem, exists := targetByGUID[guid]
		if shouldSyncWatchItem(item, targetItem, exists) {
			toSyncToTarget = append(toSyncToTarget, SyncTask{
				GUID:     guid,
				SourceID: p.SourceID,
				TargetID: p.TargetID,
			})
		}
	}

	toSyncToSource := make([]SyncTask, 0)
	if p.BiDirectional {
		for guid, item := range targetByGUID {
			sourceItem, exists := sourceByGUID[guid]
			if shouldSyncWatchItem(item, sourceItem, exists) {
				toSyncToSource = append(toSyncToSource, SyncTask{
					GUID:     guid,
					SourceID: p.TargetID,
					TargetID: p.SourceID,
				})
			}
		}
	}

	return toSyncToTarget, toSyncToSource
}

// performSync executes the actual sync tasks to a target and returns the number of successful and failed syncs
func (p *PlexSyncJob) performSync(tasks []SyncTask) (int64, int64, error) {
	// Initialize counters for successful and failed syncs
	written := int64(0)
	failed := int64(0)

	// Variable to hold the last error encountered during sync
	var lastError error

	// Process each task
	for _, task := range tasks {
		err := p.PlexDB.UpdateOrInsertWatchItem(task.SourceID, task.TargetID, task.GUID)
		if err == nil {
			written++
		} else {
			failed++
			lastError = err
		}
	}

	return written, failed, lastError
}

// Helper functions

// scanWatchItem scans a single row from the database into a WatchItem struct
func scanWatchItem(rows *sql.Rows) (WatchItem, error) {
	var item WatchItem
	if err := rows.Scan(
		&item.ItemID,
		&item.GUID,
		&item.Rating,
		&item.ViewOffset,
		&item.ViewCount,
		&item.LastViewedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.SkipCount,
		&item.LastSkippedAt,
		&item.ChangedAt,
		&item.ExtraData,
		&item.LastRatedAt,
		&item.MetadataID,
		&item.Title,
		&item.MetadataType,
		&item.Year,
		&item.LibrarySectionID,
		&item.Duration,
	); err != nil {
		return WatchItem{}, err
	}
	return item, nil
}

// shouldSyncWatchItem determines if a watch item should be synced
// based on the source and target items and their existence
func shouldSyncWatchItem(sourceItem, targetItem WatchItem, exists bool) bool {
	if !exists {
		return true
	}

	sourceLast := GetInt(sourceItem.LastViewedAt, 0)
	targetLast := GetInt(targetItem.LastViewedAt, 0)
	sourceOffset := GetInt(sourceItem.ViewOffset, 0)
	targetOffset := GetInt(targetItem.ViewOffset, 0)
	sourceCount := GetInt(sourceItem.ViewCount, 0)

	if sourceLast > targetLast {
		return true
	}
	return sourceOffset != targetOffset && sourceCount == 0
}

// getKeys returns the keys of a map as a slice of strings
func getKeys(m map[string]int64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// GetInt safely converts sql.NullInt64 to int with default.
// This is intended for comparison logic, not for preserving NULL values in DB writes.
func GetInt(n sql.NullInt64, defaultVal int) int {
	if n.Valid {
		return int(n.Int64)
	}
	return defaultVal
}
