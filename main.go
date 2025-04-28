package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/blazity/enterprise-cli/pkg/enterprise"
	"github.com/blazity/enterprise-cli/pkg/logging"
)

var GlobalCtx context.Context
var GlobalCancel context.CancelFunc

func main() {
	GlobalCtx, GlobalCancel = context.WithCancel(context.Background())
	defer GlobalCancel()

	logger := enterprise.InitializeLogger(false)

	setupSignalHandling(logger)

	cmd := enterprise.NewEnterpriseCommand(logger, GlobalCtx, GlobalCancel)

	select {
	case <-GlobalCtx.Done():
		logger.Info("Operation was cancelled before execution")
		os.Exit(1)
	default:
		if err := cmd.ExecuteContext(GlobalCtx); err != nil {
			logger.Error(err.Error())
			os.Exit(1)
		}
	}
}

func setupSignalHandling(logger logging.Logger) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-c

		fmt.Print("\r\033[K")

		logger.Info("Received signal: " + sig.String())
		logger.Info("Cancelling operations...")

		GlobalCancel()

		cleanup := make(chan struct{})
		go func() {
			time.Sleep(100 * time.Millisecond)
			close(cleanup)
		}()

		select {
		case <-cleanup:
			logger.Info("Graceful shutdown completed")
		case <-time.After(2 * time.Second):
			logger.Warning("Cleanup timed out, forcing exit")
		}

		os.Exit(1)
	}()
}
