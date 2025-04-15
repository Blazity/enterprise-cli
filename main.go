package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/blazity/enterprise-cli/cmd/enterprise"
	"github.com/blazity/enterprise-cli/pkg/github"
	"github.com/blazity/enterprise-cli/pkg/logging"
	"github.com/charmbracelet/log"
)

// Global context that can be used to propagate cancellation
var GlobalCtx context.Context
var GlobalCancel context.CancelFunc

func main() {
	// Create a base context with cancellation
	GlobalCtx, GlobalCancel = context.WithCancel(context.Background())
	defer GlobalCancel()

	// Set up the charmbracelet logger
	logger := logging.NewLogger()

	// Handle signals for graceful shutdown
	setupSignalHandling(logger)

	// Check if Git is installed
	_, err := exec.LookPath("git")
	if err != nil {
		logger.Error("Git is not installed. Please install Git to use this CLI.")
		os.Exit(1)
	}

	// Check if current directory is a Git repository
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		logger.Error("Current directory is not a Git repository. Please run this CLI from a Git repository.")
		os.Exit(1)
	}

	// Check GitHub CLI authentication
	authStatus := github.CheckAuthStatus()
	if !authStatus.CliInstalled {
		logger.Error("GitHub CLI is not installed. This tool requires GitHub CLI.")
		logger.Info("To install GitHub CLI, visit: https://cli.github.com/")
		os.Exit(1)
	} else if !authStatus.IsAuthenticated {
		logger.Error("Not authenticated with GitHub CLI. This tool requires GitHub authentication.")
		logger.Info("To authenticate, run: gh auth login")
		os.Exit(1)
	} else {
		logger.Debug("Authenticated with GitHub as " + authStatus.Username)
	}

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
		log.Info("Received signal", "signal", sig.String())
		log.Info("Cancelling operations...")

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
			log.Info("Graceful shutdown completed")
		case <-time.After(2 * time.Second):
			log.Warn("Cleanup timed out, forcing exit")
		}

		os.Exit(1)
	}()
}
