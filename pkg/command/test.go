package command

import (
	"context"
	"fmt"
	"os"

	"github.com/blazity/enterprise-cli/pkg/github"
	"github.com/blazity/enterprise-cli/pkg/logging"
	"github.com/blazity/enterprise-cli/pkg/resources"
	"github.com/spf13/cobra"
)

func NewTestCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Clone boilerplate and test resource mappings",
		Long:  "Clones the next-enterprise-terraform boilerplate into a temp directory and tests the resources.CopyAllMappings implementation",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := logging.GetLogger()

			tempDir, err := os.MkdirTemp("", "enterprise-boilerplate-*")
			if err != nil {
				logger.Error(fmt.Sprintf("Failed to create temporary directory: %v", err))
				return err
			}
			defer func() {
				if cleanErr := os.RemoveAll(tempDir); cleanErr != nil {
					logger.Warning(fmt.Sprintf("Failed to clean up temp directory: %v", cleanErr))
				}
			}()

			cloneOpts := github.CloneOptions{
				Repository:  "blazity/next-enterprise-terraform",
				Destination: tempDir,
				Depth:       1,
			}
			logger.Info(fmt.Sprintf("Cloning repository to %s...", tempDir))
			if err := github.CloneRepository(cloneOpts); err != nil {
				logger.Error(fmt.Sprintf("Failed to clone repository: %v", err))
				return err
			}
			logger.Info("Repository cloned successfully")

			rm, err := resources.NewResourceManager(tempDir)
			if err != nil {
				logger.Error(fmt.Sprintf("Failed to initialize ResourceManager: %v", err))
				return err
			}

			logger.Info("Copying resource mappings...")
			if err := rm.CopyAllMappings(); err != nil {
				logger.Error(fmt.Sprintf("Failed to copy resource mappings: %v", err))
				return err
			}

			logger.Info("Resource mappings copied successfully")
			return nil
		},
	}

	return cmd
}
