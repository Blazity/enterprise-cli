package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/blazity/enterprise-cli/cmd/enterprise"
	"github.com/blazity/enterprise-cli/pkg/logging"
)

// Global context that can be used to propagate cancellation
var GlobalCtx context.Context
var GlobalCancel context.CancelFunc

func main() {
	// Create a base context with cancellation
	GlobalCtx, GlobalCancel = context.WithCancel(context.Background())
	defer GlobalCancel()

	// Initialize the logger
	logger := enterprise.InitializeLogger(false) // Default to non-verbose; can be updated by flags

	// Handle signals for graceful shutdown
	setupSignalHandling(logger)

	// Create and execute the CLI commands
	cmd := enterprise.NewEnterpriseCommand(logger, GlobalCtx, GlobalCancel)

	// Check if the context is already cancelled before executing
	select {
	case <-GlobalCtx.Done():
		logger.Info("Operation was cancelled before execution")
		os.Exit(1)
	default:
		// Continue with execution
		if err := cmd.ExecuteContext(GlobalCtx); err != nil {
			logger.Error(err.Error())
			os.Exit(1)
		}
	}
}

// setupSignalHandling sets up handlers for CTRL+C and other termination signals
func setupSignalHandling(logger logging.Logger) {
	// Create a channel to receive OS signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		// Wait for SIGINT or SIGTERM
		sig := <-c

		// Clear current line for clean output
		fmt.Print("\r\033[K")

		// Log message
		logger.Info("Received signal: " + sig.String())
		logger.Info("Cancelling operations...")

		// Cancel the context to propagate cancellation to all operations
		GlobalCancel()

		// Give operations time to clean up (up to 2 seconds)
		cleanup := make(chan struct{})
		go func() {
			// You could add actual cleanup checks here if needed
			time.Sleep(100 * time.Millisecond)
			close(cleanup)
		}()

		// Wait for cleanup or timeout
		select {
		case <-cleanup:
			logger.Info("Graceful shutdown completed")
		case <-time.After(2 * time.Second):
			logger.Warning("Cleanup timed out, forcing exit")
		}

		os.Exit(1)
	}()
}
