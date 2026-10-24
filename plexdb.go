package main

import (
	"database/sql"
	"fmt"

	_ "github.com/ncruces/go-sqlite3/driver"
)

const (
	// SQL queries for Plex database operations

	// Query to get all accounts with non-null names
	// No arguments needed
	getAccountsQuery = `
		SELECT id, name FROM accounts
		WHERE name IS NOT NULL;`

	// Query to get watched items for a specific account
	// Args to be provided: account_id
	getWatchedQuery = `
		SELECT 
			mis.id as item_id,
			mis.guid,
			mis.rating,
			mis.view_offset,
			mis.view_count,
			mis.last_viewed_at,
			mis.created_at,
			mis.updated_at,
			mis.skip_count,
			mis.last_skipped_at,
			mis.changed_at,
			mis.extra_data,
			mis.last_rated_at,
			mi.id as metadata_id,
			mi.title,
			mi.metadata_type,
			mi.year,
			mi.library_section_id,
			mi.duration
		FROM metadata_item_settings mis
		LEFT JOIN metadata_items mi ON mis.guid = mi.guid
		WHERE mis.account_id = ? AND (mis.view_count > 0 OR mis.view_offset > 0)
		ORDER BY mis.last_viewed_at DESC`

	// Query to count watched items for a specific account
	// Args to be provided: account_id
	countWatchedQuery = `
		SELECT COUNT(*) as count
		FROM metadata_item_settings
		WHERE account_id = ? AND (view_count > 0 OR view_offset > 0)`

	// Query to count in-progress items for a specific account
	// Args to be provided: account_id
	countInProgressQuery = `
		SELECT COUNT(*) as count
		FROM metadata_item_settings
		WHERE account_id = ? AND view_offset > 0 AND (view_count IS NULL OR view_count = 0)`

	// Query to clear watch history for a specific account
	// Args to be provided: account_id
	clearWatchHistoryQuery = `
		DELETE FROM metadata_item_settings
		WHERE account_id = ? AND (view_count > 0 OR view_offset > 0)`

	// Query to check if a specific watched item exists for a specific account
	// Args to be provided: account_id, guid
	hasWatchItemQuery = `
		SELECT EXISTS(
			SELECT 1 FROM metadata_item_settings
			WHERE account_id = ? AND guid = ? AND (view_count > 0 OR view_offset > 0)
		)`

	// Query to find a matching watched item row in the target account by GUID
	// Args to be provided: account_id, guid
	findWatchItemByGUIDQuery = `
		SELECT id
		FROM metadata_item_settings
		WHERE account_id = ? AND guid = ?
		LIMIT 1`

	// Query to insert a single watched item from source account to target account
	// Args to be provided: target_account_id, source_account_id, guid
	insertWatchItemQuery = `
		INSERT INTO metadata_item_settings
		(account_id, guid, rating, view_offset, view_count,
			last_viewed_at, created_at, updated_at,
			skip_count, last_skipped_at, changed_at, extra_data, last_rated_at)
		SELECT ?, guid, rating, view_offset, view_count,
				last_viewed_at, created_at, updated_at,
				skip_count, last_skipped_at, changed_at, extra_data, last_rated_at
		FROM metadata_item_settings
		WHERE account_id = ? AND guid = ? AND (view_count > 0 OR view_offset > 0)`

	// Query to update a single watched item on the target account with values from the source account
	// Args to be provided: (source_account_id, guid,) x 11, target_row_id
	updateWatchItemQuery = `
		UPDATE metadata_item_settings
		SET rating = (SELECT rating FROM metadata_item_settings WHERE account_id = ? AND guid = ?),
			view_offset = (SELECT view_offset FROM metadata_item_settings WHERE account_id = ? AND guid = ?),
			view_count = (SELECT view_count FROM metadata_item_settings WHERE account_id = ? AND guid = ?),
			last_viewed_at = (SELECT last_viewed_at FROM metadata_item_settings WHERE account_id = ? AND guid = ?),
			created_at = (SELECT created_at FROM metadata_item_settings WHERE account_id = ? AND guid = ?),
			updated_at = (SELECT updated_at FROM metadata_item_settings WHERE account_id = ? AND guid = ?),
			skip_count = (SELECT skip_count FROM metadata_item_settings WHERE account_id = ? AND guid = ?),
			last_skipped_at = (SELECT last_skipped_at FROM metadata_item_settings WHERE account_id = ? AND guid = ?),
			changed_at = (SELECT changed_at FROM metadata_item_settings WHERE account_id = ? AND guid = ?),
			extra_data = (SELECT extra_data FROM metadata_item_settings WHERE account_id = ? AND guid = ?),
			last_rated_at = (SELECT last_rated_at FROM metadata_item_settings WHERE account_id = ? AND guid = ?)
		WHERE id = ?`

	// Query to insert watched items from source account to target account
	// Args to be provided: target_account_id, source_account_id
	BulkCopyWatchItemsQuery = `
		INSERT INTO metadata_item_settings
		(account_id, guid, rating, view_offset, view_count,
			last_viewed_at, created_at, updated_at,
			skip_count, last_skipped_at, changed_at, extra_data, last_rated_at)
		SELECT ?, guid, rating, view_offset, view_count,
				last_viewed_at, created_at, updated_at,
				skip_count, last_skipped_at, changed_at, extra_data, last_rated_at
		FROM metadata_item_settings
		WHERE account_id = ? AND (view_count > 0 OR view_offset > 0)`
)

// sqlOp represents a single SQL operation with its query and arguments.
type sqlOp struct {
	Query string
	Args  []any
}

// PlexDatabase manages a Plex SQLite connection with WAL settings.
type PlexDatabase struct {
	db *SQLiteDB
}

// NewPlexDatabase creates a new Plex database manager
func NewPlexDatabase(dbPath string, busyTimeoutMs int) (*PlexDatabase, error) {
	db, err := NewSQLiteDB(dbPath, busyTimeoutMs)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQLiteDB: %w", err)
	}
	return &PlexDatabase{db: db}, nil
}

// Connect establishes the underlying SQLite connection.
func (p *PlexDatabase) Connect() error {
	return p.db.Connect()
}

// Close closes the underlying SQLite connection.
func (p *PlexDatabase) Close() error {
	return p.db.Close()
}

// Query executes SQL on the underlying SQLite database.
func (p *PlexDatabase) Query(query string, args ...any) (*sql.Rows, error) {
	return p.db.Query(query, args...)
}

// QueryRowScan executes a single-row query on the underlying SQLite database.
func (p *PlexDatabase) QueryRowScan(query string, scanFn func(*sql.Row) error, args ...any) error {
	return p.db.QueryRowScan(query, scanFn, args...)
}

// Exec executes SQL on the underlying SQLite database.
func (p *PlexDatabase) Exec(query string, args ...any) (sql.Result, error) {
	return p.db.Exec(query, args...)
}

// Transaction executes a transaction on the underlying SQLite database.
func (p *PlexDatabase) Transaction(operations []sqlOp) error {
	wrapped := make([]struct {
		Query string
		Args  []any
	}, 0, len(operations))
	for _, op := range operations {
		wrapped = append(wrapped, struct {
			Query string
			Args  []any
		}{
			Query: op.Query,
			Args:  op.Args,
		})
	}
	return p.db.Transaction(wrapped)
}

// GetAccounts returns username to ID mapping
func (p *PlexDatabase) GetAccounts() (map[string]int64, error) {
	rows, err := p.db.Query(getAccountsQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := make(map[string]int64)
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		accounts[name] = id
	}

	return accounts, rows.Err()
}

// GetWatched retrieves watched items for a user.
func (p *PlexDatabase) GetWatched(accountID int64) ([]WatchItem, error) {
	rows, err := p.db.Query(getWatchedQuery, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]WatchItem, 0)
	for rows.Next() {
		item, err := scanWatchItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

// HasWatchItem checks whether a watched item with the given GUID exists for an account.
func (p *PlexDatabase) HasWatchItem(accountID int64, guid string) (bool, error) {
	var exists bool
	err := p.db.QueryRowScan(hasWatchItemQuery, func(row *sql.Row) error {
		return row.Scan(&exists)
	},
		accountID,
		guid,
	)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// CountWatched returns count of watched items for an account.
func (p *PlexDatabase) CountWatched(accountID int64) (int64, error) {
	var count int64
	err := p.db.QueryRowScan(countWatchedQuery, func(row *sql.Row) error {
		return row.Scan(&count)
	}, accountID)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ClearWatchHistory deletes all watch history for a user.
func (p *PlexDatabase) ClearWatchHistory(accountID int64) (int64, error) {
	count, err := p.CountWatched(accountID)
	if err != nil {
		return 0, err
	}

	if _, err := p.db.Exec(clearWatchHistoryQuery, accountID); err != nil {
		return 0, err
	}

	return count, nil
}

// UpdateOrInsertWatchItem updates a single watched item identified by GUID on the target account,
// or inserts it if the target GUID does not exist yet.
func (p *PlexDatabase) UpdateOrInsertWatchItem(sourceID, targetID int64, guid string) error {
	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction for watch item %q: %w", guid, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var targetRowID int64
	err = tx.QueryRow(findWatchItemByGUIDQuery, targetID, guid).Scan(&targetRowID)
	switch {
	case err == nil:
		_, err = tx.Exec(updateWatchItemQuery,
			sourceID, guid, sourceID, guid, sourceID, guid, sourceID, guid,
			sourceID, guid, sourceID, guid, sourceID, guid, sourceID, guid,
			sourceID, guid, sourceID, guid, sourceID, guid, targetRowID,
		)
		if err != nil {
			return fmt.Errorf("update watch item %q failed: %w", guid, err)
		}
	case err == sql.ErrNoRows:
		_, err = tx.Exec(insertWatchItemQuery, targetID, sourceID, guid)
		if err != nil {
			return fmt.Errorf("insert watch item %q failed: %w", guid, err)
		}
	default:
		return fmt.Errorf("failed to find target row for watch item %q: %w", guid, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit watch item %q failed: %w", guid, err)
	}
	return nil
}

// InsertWatchItem copies a single watch item from source to target.
func (p *PlexDatabase) InsertWatchItem(sourceID, targetID int64, guid string) error {
	ops := []sqlOp{
		{
			Query: insertWatchItemQuery,
			Args:  []any{targetID, sourceID, guid},
		},
	}

	if err := p.Transaction(ops); err != nil {
		return fmt.Errorf("insert watch item %q failed: %w", guid, err)
	}
	return nil
}

// BulkCopyWatchItems performs bulk copy from source to target.
func (p *PlexDatabase) BulkCopyWatchItems(sourceID, targetID int64) (int64, error) {
	count, err := p.CountWatched(sourceID)
	if err != nil {
		return 0, fmt.Errorf("failed to count source items for bulk copy: %w", err)
	}

	ops := []sqlOp{
		{
			Query: clearWatchHistoryQuery,
			Args:  []any{targetID},
		},
		{
			Query: BulkCopyWatchItemsQuery,
			Args:  []any{targetID, sourceID},
		},
	}

	if err := p.Transaction(ops); err != nil {
		return 0, fmt.Errorf("bulk copy failed: %w", err)
	}

	return int64(count), nil
}

// CountInProgress returns count of in-progress items.
func (p *PlexDatabase) CountInProgress(accountID int64) (int64, error) {
	var count int64
	err := p.db.QueryRowScan(countInProgressQuery, func(row *sql.Row) error {
		return row.Scan(&count)
	}, accountID)
	if err != nil {
		return 0, err
	}
	return count, nil
}
