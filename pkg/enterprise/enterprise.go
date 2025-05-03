package enterprise

import (
	"context"
	"os"
	"os/exec"

	"github.com/blazity/enterprise-cli/pkg/command"
	"github.com/blazity/enterprise-cli/pkg/github"
	"github.com/blazity/enterprise-cli/pkg/logging"
	_ "github.com/blazity/enterprise-cli/pkg/provider/aws"
	"github.com/spf13/cobra"
)

func NewEnterpriseCommand(ctx context.Context, cancel context.CancelFunc) *cobra.Command {
	var verbose bool

	rootCmd := &cobra.Command{
		Use:   "enterprise",
		Short: "Enterprise CLI for infrastructure management",
		Long:  "Enterprise CLI for preparing and deploying infrastructure across various providers",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			logger := logging.GetLogger()
			if verbose {
				if l, ok := logger.(interface{ SetVerbose(bool) }); ok {
					l.SetVerbose(true)
					logger.Debug("Verbose logging enabled")
				} else {
					logger.Warning("Logger does not support dynamic verbosity changes. Continuing in non-verbose mode.")
				}
			} else {
				if l, ok := logger.(interface{ SetVerbose(bool) }); ok {
					l.SetVerbose(false)
				}
			}

			performEarlyChecks()
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	rootCmd.AddCommand(command.NewPrepareCommand(ctx))
	rootCmd.AddCommand(command.NewDeployCommand(ctx, cancel))
	rootCmd.AddCommand(command.NewCodemodCommand(ctx))

	// Customize help template to show only commands
	rootCmd.SetHelpTemplate(`{{.Short}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}

Use "{{.CommandPath}} [command] --help" for more information about a command.
`)

	return rootCmd
}

// InitializeLogger initializes the singleton logger with the specified verbosity level and returns it
func InitializeLogger(verbose bool) logging.Logger {
	logging.InitLogger(verbose)
	return logging.GetLogger()
}

// performEarlyChecks runs validation checks needed before executing commands
func performEarlyChecks() {
	// Check if Git is installed
	_, err := exec.LookPath("git")
	if err != nil {
		logger := logging.GetLogger()
		logger.Error("Git is not installed. Please install Git to use this CLI.")
		os.Exit(1)
	}

	// Check if current directory is a Git repository
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		logger := logging.GetLogger()
		logger.Error("Current directory is not a Git repository. Please run this CLI from a Git repository.")
		os.Exit(1)
	}

	// Check GitHub CLI authentication
	authStatus := github.CheckAuthStatus()
	if !authStatus.CliInstalled {
		logger := logging.GetLogger()
		logger.Error("GitHub CLI is not installed. This tool requires GitHub CLI.")
		logger.Info("To install GitHub CLI, visit: https://cli.github.com/")
		os.Exit(1)
	} else if !authStatus.IsAuthenticated {
		logger := logging.GetLogger()
		logger.Error("Not authenticated with GitHub CLI. This tool requires GitHub authentication.")
		logger.Info("To authenticate, run: gh auth login")
		os.Exit(1)
	} else {
		logger := logging.GetLogger()
		logger.Debug("Authenticated with GitHub as " + authStatus.Username)
	}
}
