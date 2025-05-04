package logging

import (
	"os"
	"sync"

	"github.com/charmbracelet/log"
)

type Logger interface {
	Info(message string, keyvals ...interface{})
	Warning(message string, keyvals ...interface{})
	Error(message string, keyvals ...interface{})
	Debug(message interface{}, keyvals ...interface{})
	IsVerbose() bool
	SetVerbose(verbose bool)
}

type logger struct {
	verbose bool
	log     *log.Logger
}

var (
	instance Logger
	once     sync.Once
)

func GetLogger() Logger {
	once.Do(func() {
		instance = newLogger(false)
	})
	return instance
}

func InitLogger(verbose bool) {
	once.Do(func() {
		instance = newLogger(verbose)
	})
}

func newLogger(verbose bool) Logger {
	l := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    false,
		ReportTimestamp: false,
		Level:           log.InfoLevel,
		Prefix:          "",
	})

	if verbose {
		l.SetLevel(log.DebugLevel)
		l.SetReportTimestamp(true)
	}

	styles := log.DefaultStyles()
	l.SetStyles(styles)

	return &logger{
		verbose: verbose,
		log:     l,
	}
}

func (l *logger) Info(message string, keyvals ...interface{}) {
	l.log.Helper()
	l.log.Info(message, keyvals...)
}

func (l *logger) Warning(message string, keyvals ...interface{}) {
	l.log.Helper()
	l.log.Warn(message, keyvals...)
}

func (l *logger) Error(message string, keyvals ...interface{}) {
	l.log.Helper()
	l.log.Error(message, keyvals...)
}

func (l *logger) Debug(message interface{}, keyvals ...interface{}) {
	l.log.Helper()
	l.log.Debug(message, keyvals...)
}

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
