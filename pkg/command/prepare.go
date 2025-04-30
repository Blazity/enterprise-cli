package command

import (
	"context"
	"strings"

	"github.com/blazity/enterprise-cli/pkg/logging"
	"github.com/blazity/enterprise-cli/pkg/provider"
	"github.com/blazity/enterprise-cli/pkg/ui"
	"github.com/spf13/cobra"
)

func NewPrepareCommand(logger logging.Logger, ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prepare [provider]",
		Short: "Prepare infrastructure for enterprise deployment",
		Long:  "Prepare infrastructure and collect configuration for enterprise deployment",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Get the command context, which is already connected to the global context
			cmdCtx := cmd.Context()

			// Check if the context is already done (e.g., CTRL+C was pressed before we even started)
			select {
			case <-cmdCtx.Done():
				logger.Info("Operation cancelled before it started")
				return
			default:
				// Continue with preparation
			}

			providerName := args[0]
			logger.Info("Preparing the " + ui.LegibleProviderName(providerName) + " provider")

			p, exists := provider.Get(providerName, logger)
			if !exists {
				availableProviders := provider.ListAvailableProviders()
				logger.Error("Provider not supported: " + providerName)
				logger.Info("Available providers: " + strings.Join(availableProviders, ", "))
				return
			}

			// Create a channel for completion notification
			done := make(chan struct{})
			var prepErr error

			// Run the preparation in a goroutine so we can monitor for cancellation
			go func() {
				prepErr = p.Prepare()
				close(done)
			}()

			// Wait for either preparation to complete or context to be cancelled
			select {
			case <-done:
				// Preparation completed
				if prepErr != nil {
					logger.Error("Failed to prepare: " + prepErr.Error())
					return
				}
				logger.Info("Preparation completed successfully")
			case <-cmdCtx.Done():
				// Context cancelled (CTRL+C was pressed)
				logger.Info("Preparation cancelled, cleaning up...")
				// Wait for the preparation to actually finish its cleanup
				<-done
				if prepErr != nil && prepErr.Error() != "operation cancelled by user" {
					logger.Error("Error during cancellation: " + prepErr.Error())
				}
				logger.Info("Cleanup completed")
			}
		},
	}

	return cmd
}
