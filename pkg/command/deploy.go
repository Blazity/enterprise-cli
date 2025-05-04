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

			if awsP, ok := p.(*aws.AwsProvider); ok {
				awsP.SetCancelFunc(cancel)
				logger.Debug("Cancel function set on AWS provider")
			}

			done := make(chan struct{})
			var deployErr error

			go func() {
				deployErr = p.Deploy()
				close(done)
			}()

			select {
			case <-done:
				if deployErr != nil {
					logger.Error("Failed to deploy: " + deployErr.Error())
					return
				}
				logger.Info("Deployment completed successfully")
			case <-cmd.Context().Done():
				logger.Info("Deployment cancelled, cleaning up...")
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
