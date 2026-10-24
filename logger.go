package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Logger struct {
	logFile *os.File
	logger  *log.Logger
}

// fatal logger function
func (l *Logger) Fatal(logText string, vars ...any) {
	l.logEntry("fatal", logText, vars...)
	l.Close()
	os.Exit(1)
}

// error logger function
func (l *Logger) Error(logText string, vars ...any) {
	l.logEntry("error", logText, vars...)
}

// warning logger function
func (l *Logger) Warn(logText string, vars ...any) {
	l.logEntry("warn", logText, vars...)
}

// info logger function
func (l *Logger) Info(logText string, vars ...any) {
	l.logEntry("info", logText, vars...)
}

// success logger function
func (l *Logger) Success(logText string, vars ...any) {
	l.logEntry("success", logText, vars...)
}

// debug logger function
func (l *Logger) Debug(logText string, vars ...any) {
	l.logEntry("debug", logText, vars...)
}

// logEntry is a helper function to log messages with a specific log type (error, warn, info, success, debug).
func (l *Logger) logEntry(logType string, logText string, vars ...any) {
	msg := strings.Trim(fmt.Sprintf(logText, vars...), " \n")
	if l.logger != nil {
		l.logger.Printf("%s: %s\n", strings.ToUpper(logType), msg)
	}
	fmt.Printf("%s: %s\n", strings.ToUpper(logType), msg)
}

// Close closes the logger and releases any resources associated with it.
func (l *Logger) Close() {
	// clean up
	if l.logFile != nil {
		l.logFile.Close()
		l.logFile = nil
	}
	l.logger = nil
}

// setLogFile sets the log file path for the logger and initializes the logger to write to that file.
func (l *Logger) setLogFile(path string) error {
	// Validate the log file path
	if path == "" {
		return fmt.Errorf("log file path cannot be empty")
	}

	// Create the log directory if it doesn't exist
	err := os.MkdirAll(filepath.Dir(path), os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create log directory '%s': %s", path, err)
	}

	// Open the log file for writing (append mode)
	l.logFile, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file '%s': %s", path, err)
	}

	// Create a new logger that writes to the log file
	l.logger = log.New(l.logFile, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	return nil
}

// NewLogger creates a new logger instance with default settings (no log file).
func NewLogger() *Logger {
	// Return a new logger instance with default settings (no log file).
	return &Logger{}
}

// NewLogger creates a new logger instance that writes log messages to the specified log file.
func NewLoggerWithLogFile(path string) (*Logger, error) {
	// Initialize the logger
	Log := &Logger{}

	err := Log.setLogFile(path)
	if err != nil {
		return nil, err
	}

	return Log, nil
}

// SetLogFile sets the log file path for the logger and initializes the logger to write to that file.
func (l *Logger) SetLogFile(path string) error {
	return l.setLogFile(path)
}
