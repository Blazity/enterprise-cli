package logging

import (
	"os"

	"github.com/charmbracelet/log"
)

type Logger interface {
	Info(message string)
	Warning(message string)
	Error(message string)
	Debug(message interface{})
	IsVerbose() bool
}

type logger struct {
	verbose bool
	log     *log.Logger
}

func NewLogger(verbose bool) Logger {
	l := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    verbose,
		ReportTimestamp: true,
		Level:           log.InfoLevel,
	})

	// Set debug level if verbose mode is enabled
	if verbose {
		l.SetLevel(log.DebugLevel)
	}

	return &logger{
		verbose: verbose,
		log:     l,
	}
}

// Keeping the function for backward compatibility, but it just calls NewLogger
func NewLoggerWithVerbose(verbose bool) Logger {
	return NewLogger(verbose)
}

func (l *logger) Info(message string) {
	l.log.Info(message)
}

func (l *logger) Warning(message string) {
	l.log.Warn(message)
}

func (l *logger) Error(message string) {
	l.log.Error(message)
}

func (l *logger) Debug(message interface{}) {
	if l.verbose {
		l.log.Debug(message)
	}
}

func (l *logger) IsVerbose() bool {
	return l.verbose
}
