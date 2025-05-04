package command

import (
	"context"
	"fmt"

	"github.com/blazity/enterprise-cli/pkg/codemod"
	"github.com/blazity/enterprise-cli/pkg/logging"
	"github.com/spf13/cobra"
)

func NewCodemodCommand(ctx context.Context) *cobra.Command {
	logger := logging.GetLogger()
	cfg := codemod.NewDefaultJsCodemodConfig()

	codemodCmd := &cobra.Command{
		Use:   "codemod",
		Short: "Run a jscodeshift codemod transformation",
		Long:  `Applies a specified jscodeshift codemod to a target file or directory.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info(fmt.Sprintf("Running codemod '%s' on '%s'...", cfg.JsCodemodName, cfg.InputPath))

			if err := codemod.RunJsCodemod(cfg); err != nil {
				logger.Error(fmt.Sprintf("Codemod execution failed: %v", err))
				return err
			}

			if cfg.DryRun {
				logger.Info("Codemod dry run completed successfully.")
			} else {
				logger.Info("Codemod applied successfully.")
			}
			return nil
		},
	}

	codemodCmd.Flags().StringVarP(&cfg.InputPath, "input", "i", "", "Path to the file or directory to transform (required)")
	codemodCmd.Flags().StringVarP(&cfg.JsCodemodName, "transform", "t", "", "Name of the transform to use (required)")
	codemodCmd.Flags().StringVarP(&cfg.JsCodemodDir, "dir", "d", cfg.JsCodemodDir, "Directory containing transform files")
	codemodCmd.Flags().StringVar(&cfg.Parser, "parser", cfg.Parser, "Parser to use (e.g., tsx, babel)")
	codemodCmd.Flags().StringVar(&cfg.Extensions, "extensions", cfg.Extensions, "Comma-separated list of file extensions to process")
	codemodCmd.Flags().BoolVar(&cfg.DryRun, "dry", cfg.DryRun, "Perform a dry run without modifying files")

	_ = codemodCmd.MarkFlagRequired("input")
	_ = codemodCmd.MarkFlagRequired("transform")

	return codemodCmd
}
