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
			cmdCtx := cmd.Context()

			select {
			case <-cmdCtx.Done():
				logger.Info("Operation cancelled before it started")
				return
			default:
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

			done := make(chan struct{})
			var prepErr error

			go func() {
				prepErr = p.Prepare()
				close(done)
			}()

			select {
			case <-done:
				if prepErr != nil {
					logger.Error("Failed to prepare: " + prepErr.Error())
					return
				}
				logger.Info("Preparation completed successfully")
			case <-cmdCtx.Done():
				logger.Info("Preparation cancelled, cleaning up...")
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
