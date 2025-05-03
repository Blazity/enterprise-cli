package command

import (
	"context"
	"strings"

	"github.com/blazity/enterprise-cli/pkg/logging"
	"github.com/blazity/enterprise-cli/pkg/provider"
	"github.com/blazity/enterprise-cli/pkg/provider/aws"
	"github.com/blazity/enterprise-cli/pkg/ui"
	"github.com/spf13/cobra"
)

func NewDeployCommand(ctx context.Context, cancel context.CancelFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy [provider]",
		Short: "Deploy enterprise infrastructure",
		Long:  "Deploy enterprise infrastructure to the selected provider",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			logger := logging.GetLogger()
			// Get the command context, which is already connected to the global context
			select {
			case <-cmd.Context().Done():
				logger.Info("Operation cancelled before it started")
				return
			default:
			}

			providerName := args[0]
			logger.Info("Preparing the deployment for " + ui.LegibleProviderName(providerName))

			p, exists := provider.Get(providerName)
			if !exists {
				availableProviders := provider.ListAvailableProviders()
				logger.Error("Provider not supported: " + providerName)
				logger.Info("Available providers: " + strings.Join(availableProviders, ", "))
				return
			}

			// Set the cancel function on the provider if it's AWS
			if awsP, ok := p.(*aws.AwsProvider); ok {
				awsP.SetCancelFunc(cancel)
				logger.Debug("Cancel function set on AWS provider")
			}

			// Create a channel for completion notification
			done := make(chan struct{})
			var deployErr error

			// Run the deployment in a goroutine so we can monitor for cancellation
			go func() {
				deployErr = p.Deploy()
				close(done)
			}()

			// Wait for either deployment to complete or context to be cancelled
			select {
			case <-done:
				// Deployment completed
				if deployErr != nil {
					logger.Error("Failed to deploy: " + deployErr.Error())
					return
				}
				logger.Info("Deployment completed successfully")
			case <-cmd.Context().Done():
				// Context cancelled (CTRL+C was pressed)
				logger.Info("Deployment cancelled, cleaning up...")
				// Wait for the deployment to actually finish its cleanup
				<-done
				if deployErr != nil && deployErr.Error() != "operation cancelled by user" {
					logger.Error("Error during cancellation: " + deployErr.Error())
				}
				logger.Info("Cleanup completed")
			}
		},
	}

	return cmd
}
