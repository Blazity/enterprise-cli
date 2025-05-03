package github

import (
	"fmt"
	"strings"
	"time"

	"github.com/blazity/enterprise-cli/pkg/logging"
	"github.com/cli/go-gh"
)

type PROptions struct {
	SourceRepo          string
	TargetRepo          string
	BranchName          string
	BaseBranch          string
	Title               string
	Body                string
	CommitMessage       string
	ChangesToCommit     []string // List of files to add before committing
	ModifiedDestination string   // Path to the cloned & modified repository
}

type PRStatus struct {
	URL       string
	Number    int
	State     string
	Completed bool
	Passing   bool
}

func PreparePullRequest(opts PROptions) (string, error) {
	// Retrieve global logger singleton
	logger := logging.GetLogger()
	logger.Debug("Preparing pull request")

	// Commit changes if needed
	if len(opts.ChangesToCommit) > 0 {
		if err := CommitChanges(opts.ModifiedDestination, opts.CommitMessage, opts.ChangesToCommit); err != nil {
			return "", err
		}
	}

	// Push branch if needed
	currentBranch, err := GetCurrentBranch(opts.ModifiedDestination)
	if err != nil {
		return "", err
	}

	if err := PushBranch(opts.ModifiedDestination, currentBranch, "origin"); err != nil {
		return "", err
	}

	// Create PR using gh CLI
	logger.Debug("Creating pull request")

	// Build PR command
	prArgs := []string{"pr", "create"}

	// Add base and target repos if needed
	if opts.TargetRepo != "" && opts.TargetRepo != opts.SourceRepo {
		// Format as owner/repo:branch
		baseDest := opts.BaseBranch
		if opts.TargetRepo != "" {
			baseDest = fmt.Sprintf("%s:%s", opts.TargetRepo, opts.BaseBranch)
		}
		prArgs = append(prArgs, "--base", baseDest)
	}

	// Add title and body
	prArgs = append(prArgs, "--title", opts.Title)

	// Body can be multiple lines, so we handle it carefully
	if opts.Body != "" {
		prArgs = append(prArgs, "--body", opts.Body)
	}

	// Execute from the repo directory
	if opts.ModifiedDestination != "" {
		prArgs = append([]string{"--cwd", opts.ModifiedDestination}, prArgs...)
	}

	// Execute the PR creation command
	stdout, stderr, err := gh.Exec(prArgs...)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to create PR: %s", err))
		logger.Error(stderr.String())
		return "", err
	}

	prURL := strings.TrimSpace(stdout.String())
	logger.Info(fmt.Sprintf("Pull request created: %s", prURL))

	return prURL, nil
}

func CheckPRStatus(prURL string) (PRStatus, error) {
	// Retrieve global logger singleton
	logger := logging.GetLogger()
	status := PRStatus{
		URL: prURL,
	}

	// Extract PR number from URL
	parts := strings.Split(prURL, "/")
	if len(parts) < 7 {
		return status, fmt.Errorf("invalid PR URL format")
	}

	// View PR details using gh CLI
	viewArgs := []string{"pr", "view", prURL, "--json", "number,state,statusCheckRollup"}
	stdout, stderr, err := gh.Exec(viewArgs...)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to check PR status: %s", err))
		logger.Error(stderr.String())
		return status, err
	}

	output := stdout.String()

	// Parse the JSON output to extract status
	// For simplicity, we'll do basic parsing instead of unmarshaling JSON
	if strings.Contains(output, "\"state\": \"open\"") {
		status.State = "open"
	} else if strings.Contains(output, "\"state\": \"closed\"") {
		status.State = "closed"
	} else if strings.Contains(output, "\"state\": \"merged\"") {
		status.State = "merged"
	}

	// Check CI status
	status.Completed = !strings.Contains(output, "\"status\": \"pending\"") &&
		!strings.Contains(output, "\"status\": \"queued\"") &&
		!strings.Contains(output, "\"status\": \"in_progress\"")

	status.Passing = strings.Contains(output, "\"status\": \"success\"") &&
		!strings.Contains(output, "\"status\": \"failure\"") &&
		!strings.Contains(output, "\"status\": \"cancelled\"")

	return status, nil
}

// Wait for PR checks to complete with timeout
func WaitForPRChecks(prURL string, timeout time.Duration) (PRStatus, error) {
	// Retrieve global logger singleton
	logger := logging.GetLogger()
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		status, err := CheckPRStatus(prURL)
		if err != nil {
			return status, err
		}

		if status.Completed {
			logger.Info("PR checks completed")
			return status, nil
		}

		logger.Debug("Waiting for PR checks to complete...")
		time.Sleep(30 * time.Second)
	}

	return PRStatus{}, fmt.Errorf("timeout waiting for PR checks to complete")
}
