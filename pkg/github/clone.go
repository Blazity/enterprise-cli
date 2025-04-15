package github

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blazity/enterprise-cli/pkg/logging"
	"github.com/cli/go-gh"
)

type CloneOptions struct {
	Repository  string
	Branch      string
	Destination string
	Depth       int
}

func CloneRepository(opts CloneOptions, logger logging.Logger) error {
	logger.Info(fmt.Sprintf("Cloning repository %s", opts.Repository))
	
	// Build clone command arguments
	args := []string{"repo", "clone"}
	
	// Format repository name/URL
	repoName := opts.Repository
	if !strings.Contains(repoName, "/") && !strings.HasPrefix(repoName, "http") && !strings.HasPrefix(repoName, "git@") {
		logger.Error("Invalid repository format. Use 'owner/repo' format.")
		return fmt.Errorf("invalid repository format")
	}
	
	// Add repository to arguments
	args = append(args, repoName)
	
	// Add destination if specified
	if opts.Destination != "" {
		// Create destination directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(opts.Destination), 0755); err != nil {
			logger.Error(fmt.Sprintf("Failed to create destination directory: %s", err))
			return err
		}
		args = append(args, opts.Destination)
	}
	
	// For gh cli, we need to pass git flags after --
	extraArgs := []string{}
	
	// Add branch flag as git argument if needed
	if opts.Branch != "" {
		extraArgs = append(extraArgs, "-b", opts.Branch)
	}
	
	// Add depth flag as git argument if needed
	if opts.Depth > 0 {
		extraArgs = append(extraArgs, "--depth", fmt.Sprintf("%d", opts.Depth))
	}
	
	// Add -- separator and git flags if we have any
	if len(extraArgs) > 0 {
		args = append(args, "--")
		args = append(args, extraArgs...)
	}
	
	// Execute gh repo clone command
	stdout, stderr, err := gh.Exec(args...)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to clone repository: %s", err))
		logger.Error(stderr.String())
		return err
	}
	
	if logger.IsVerbose() {
		logger.Debug(stdout.String())
	}
	
	logger.Info(fmt.Sprintf("Repository %s cloned successfully", opts.Repository))
	
	return nil
}