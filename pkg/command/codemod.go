package command

import (
	"context"
	"fmt"

	"github.com/blazity/enterprise-cli/pkg/codemod"
	"github.com/blazity/enterprise-cli/pkg/logging"
	"github.com/spf13/cobra"
)

// NewCodemodCommand creates the codemod command
func NewCodemodCommand(ctx context.Context) *cobra.Command {
	logger := logging.GetLogger()
	cfg := codemod.NewDefaultConfig()

	codemodCmd := &cobra.Command{
		Use:   "codemod",
		Short: "Run a jscodeshift codemod transformation",
		Long:  `Applies a specified jscodeshift codemod to a target file or directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info(fmt.Sprintf("Running codemod '%s' on '%s'...", cfg.CodemodName, cfg.InputPath))

			// The config struct is already populated by Viper/Pflag binding
			if err := codemod.RunCodemod(cfg); err != nil {
				logger.Error(fmt.Sprintf("Codemod execution failed: %v", err))
				// Return the error so Cobra displays it
				return err
			}

			// Decide on success message based on DryRun flag in cfg
			if cfg.DryRun {
				logger.Info("Codemod dry run completed successfully.")
			} else {
				logger.Info("Codemod applied successfully.")
			}
			return nil
		},
	}

	// Define flags and bind them to the cfg struct fields
	codemodCmd.Flags().StringVarP(&cfg.InputPath, "input", "i", "", "Path to the file or directory to transform (required)")
	codemodCmd.Flags().StringVarP(&cfg.CodemodName, "transform", "t", "", "Name of the transform to use (required)")
	codemodCmd.Flags().StringVarP(&cfg.CodemodDir, "dir", "d", cfg.CodemodDir, "Directory containing transform files")
	codemodCmd.Flags().StringVar(&cfg.Parser, "parser", cfg.Parser, "Parser to use (e.g., tsx, babel)")
	codemodCmd.Flags().StringVar(&cfg.Extensions, "extensions", cfg.Extensions, "Comma-separated list of file extensions to process")
	codemodCmd.Flags().BoolVar(&cfg.DryRun, "dry", cfg.DryRun, "Perform a dry run without modifying files")
	// Note: The global --verbose flag controls logger verbosity. We don't need a separate flag here
	// unless we want jscodeshift's specific verbose output independent of the CLI's logging level.
	// For now, we'll rely on the global flag affecting the logger.
	// codemodCmd.Flags().BoolVarP(&cfg.Verbose, "verbose", "v", cfg.Verbose, "Show more jscodeshift process information")

	// Mark required flags
	_ = codemodCmd.MarkFlagRequired("input")
	_ = codemodCmd.MarkFlagRequired("transform")

	return codemodCmd
}
