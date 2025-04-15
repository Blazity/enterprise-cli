package logging

import (
	"os"

	"github.com/charmbracelet/log"
)

type Logger interface {
	Info(message string)
	Warning(message string)
	Error(message string)
	Debug(message string)
	IsVerbose() bool
}

type logger struct {
	verbose bool
	log     *log.Logger
}

func NewLogger() Logger {
	l := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    false,
		ReportTimestamp: false,
		Level:           log.InfoLevel,
	})

	return &logger{
		verbose: false,
		log:     l,
	}
}

func NewLoggerWithVerbose(verbose bool) Logger {
	l := NewLogger().(*logger)
	l.verbose = verbose
	if verbose {
		l.log.SetLevel(log.DebugLevel)
	}
	return l
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

func (l *logger) Debug(message string) {
	if l.verbose {
		l.log.Debug(message)
	}
}

func (l *logger) IsVerbose() bool {
	return l.verbose
}