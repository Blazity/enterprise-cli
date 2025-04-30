package logging

import (
	"os"

	"github.com/charmbracelet/log"
)

// Logger defines the interface for logging operations.
// It includes standard levels and a custom Success level.
type Logger interface {
	Info(message string, keyvals ...interface{})       // Added keyvals for structured logging
	Warning(message string, keyvals ...interface{})    // Added keyvals
	Error(message string, keyvals ...interface{})      // Added keyvals
	Debug(message interface{}, keyvals ...interface{}) // Added keyvals
	IsVerbose() bool
	SetVerbose(verbose bool) // New method for dynamic verbosity
}

type logger struct {
	verbose bool
	log     *log.Logger
}

// NewLogger creates a new logger instance.
func NewLogger(verbose bool) Logger {
	l := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    false, // Keep this false unless debugging the logger itself
		ReportTimestamp: false,
		Level:           log.InfoLevel,
		Prefix:          "", // Remove default prefix if any
	})

	// Set debug level if verbose mode is enabled
	if verbose {
		l.SetLevel(log.DebugLevel)
		l.SetReportTimestamp(true)
	}

	// Customize styles
	styles := log.DefaultStyles()
	l.SetStyles(styles)

	return &logger{
		verbose: verbose,
		log:     l,
	}
}

// Keeping the function for backward compatibility, but it just calls NewLogger
func NewLoggerWithVerbose(verbose bool) Logger {
	return NewLogger(verbose)
}

// Info logs an informational message.
func (l *logger) Info(message string, keyvals ...interface{}) {
	l.log.Info(message, keyvals...)
}

// Warning logs a warning message.
func (l *logger) Warning(message string, keyvals ...interface{}) {
	l.log.Warn(message, keyvals...)
}

// Error logs an error message.
func (l *logger) Error(message string, keyvals ...interface{}) {
	l.log.Error(message, keyvals...)
}

// Debug logs a debug message only if verbose mode is enabled.
func (l *logger) Debug(message interface{}, keyvals ...interface{}) {
	// The level set in NewLogger handles whether this is printed.
	// No need for the `if l.verbose` check here.
	l.log.Debug(message, keyvals...)
}

// IsVerbose returns true if verbose logging is enabled.
func (l *logger) IsVerbose() bool {
	return l.verbose
}

func (l *logger) SetVerbose(verbose bool) {
	l.verbose = verbose
	if verbose {
		l.log.SetLevel(log.DebugLevel)
		l.log.SetReportCaller(true)
	} else {
		l.log.SetLevel(log.InfoLevel)
		l.log.SetReportCaller(false)
	}
}
