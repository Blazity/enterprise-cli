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

func CloneRepository(opts CloneOptions) error {
	logger := logging.GetLogger()


	args := []string{"repo", "clone"}

	repoName := opts.Repository
	if !strings.Contains(repoName, "/") && !strings.HasPrefix(repoName, "http") && !strings.HasPrefix(repoName, "git@") {
		logger.Error("Invalid repository format. Use 'owner/repo' format.")
		return fmt.Errorf("invalid repository format")
	}

	args = append(args, repoName)

	if opts.Destination != "" {
		if err := os.MkdirAll(filepath.Dir(opts.Destination), 0755); err != nil {
			logger.Error(fmt.Sprintf("Failed to create destination directory: %s", err))
			return err
		}
		args = append(args, opts.Destination)
	}

	logger.Debug("Assembling git arguments for clone...")
	extraArgs := []string{}

	if opts.Branch != "" {
		extraArgs = append(extraArgs, "-b", opts.Branch)
	}

	if opts.Depth > 0 {
		extraArgs = append(extraArgs, "--depth", fmt.Sprintf("%d", opts.Depth))
	}

	if len(extraArgs) > 0 {
		args = append(args, "--")
		args = append(args, extraArgs...)
	}

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
