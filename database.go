package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/ncruces/go-sqlite3/driver"
)

// SQLiteDB is a wrapper around sql.DB for SQLite operations
type SQLiteDB struct {
	db            *sql.DB
	path          string
	busyTimeoutMs int
}

// NewSQLiteDB creates a new SQLiteDB instance with the specified path and busy timeout
func NewSQLiteDB(path string, busyTimeoutMs int) (*SQLiteDB, error) {
	// Convert DBPath to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path of database: %w", err)
	}
	path = absPath

	// Validate DBPath
	_, err = os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("database file does not exist: %s", path)
		}
		return nil, fmt.Errorf("failed to stat database file: %w", err)
	}

	// Validate busyTimeoutMs
	if busyTimeoutMs < 0 {
		return nil, fmt.Errorf("busy timeout must be zero or greater")
	}

	return &SQLiteDB{
		db:            nil,
		path:          path,
		busyTimeoutMs: busyTimeoutMs,
	}, nil
}

// Connect establishes database connection with proper settings
func (s *SQLiteDB) Connect() error {
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return fmt.Errorf("failed to close existing database connection: %w", err)
		}
		s.db = nil
	}

	// Open SQLite database with busy timeout if specified
	var dbPath string
	if s.busyTimeoutMs > 0 {
		dbPath = fmt.Sprintf("file:%s?_busy_timeout=%d", s.path, s.busyTimeoutMs)
	} else {
		dbPath = fmt.Sprintf("file:%s", s.path)
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Set WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		_ = db.Close()
		return fmt.Errorf("failed to set WAL mode: %w", err)
	}

	s.db = db

	return nil
}

// Close closes the database connection
func (s *SQLiteDB) Close() error {
	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return err
		}
		s.db = nil
	}
	return nil
}

// Begin starts a new transaction
func (s *SQLiteDB) Begin() (*sql.Tx, error) {
	if err := s.ensureConnected(); err != nil {
		return nil, err
	}
	return s.db.Begin()
}

// BeginTx starts a new transaction with the specified options
func (s *SQLiteDB) BeginTx(opts *sql.TxOptions) (*sql.Tx, error) {
	if err := s.ensureConnected(); err != nil {
		return nil, err
	}
	return s.db.BeginTx(context.Background(), opts)
}

// Query executes SQL using SQLite's busy timeout handling.
func (s *SQLiteDB) Query(query string, args ...any) (*sql.Rows, error) {
	if err := s.ensureConnected(); err != nil {
		return nil, err
	}
	return s.db.Query(query, args...)
}

// QueryRowScan executes a single-row query and returns the scan result.
func (s *SQLiteDB) QueryRowScan(query string, scanFn func(*sql.Row) error, args ...any) error {
	if err := s.ensureConnected(); err != nil {
		return err
	}

	row := s.db.QueryRow(query, args...)
	return scanFn(row)
}

// Exec executes SQL that doesn't return rows.
func (s *SQLiteDB) Exec(query string, args ...any) (sql.Result, error) {
	if err := s.ensureConnected(); err != nil {
		return nil, err
	}
	return s.db.Exec(query, args...)
}

// Transaction executes operations in a single transaction.
func (s *SQLiteDB) Transaction(operations []struct {
	Query string
	Args  []any
}) error {
	if err := s.ensureConnected(); err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	for _, op := range operations {
		if _, err := tx.Exec(op.Query, op.Args...); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("transaction failed: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ensureConnected checks if the database connection is established, and connects if not
func (s *SQLiteDB) ensureConnected() error {
	if s.db == nil {
		return s.Connect()
	}
	return nil
}
