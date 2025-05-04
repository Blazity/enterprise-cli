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
	ChangesToCommit     []string
	ModifiedDestination string
}

type PRStatus struct {
	URL       string
	Number    int
	State     string
	Completed bool
	Passing   bool
}

func PreparePullRequest(opts PROptions) (string, error) {
	logger := logging.GetLogger()
	logger.Debug("Preparing pull request")

	if len(opts.ChangesToCommit) > 0 {
		if err := CommitChanges(opts.ModifiedDestination, opts.CommitMessage, opts.ChangesToCommit); err != nil {
			return "", err
		}
	}

	currentBranch, err := GetCurrentBranch(opts.ModifiedDestination)
	if err != nil {
		return "", err
	}

	if err := PushBranch(opts.ModifiedDestination, currentBranch, "origin"); err != nil {
		return "", err
	}

	logger.Debug("Creating pull request")

	prArgs := []string{"pr", "create"}

	if opts.TargetRepo != "" && opts.TargetRepo != opts.SourceRepo {
		baseDest := opts.BaseBranch
		if opts.TargetRepo != "" {
			baseDest = fmt.Sprintf("%s:%s", opts.TargetRepo, opts.BaseBranch)
		}
		prArgs = append(prArgs, "--base", baseDest)
	}

	prArgs = append(prArgs, "--title", opts.Title)

	if opts.Body != "" {
		prArgs = append(prArgs, "--body", opts.Body)
	}

	if opts.ModifiedDestination != "" {
		prArgs = append([]string{"--cwd", opts.ModifiedDestination}, prArgs...)
	}

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
	logger := logging.GetLogger()
	status := PRStatus{
		URL: prURL,
	}

	parts := strings.Split(prURL, "/")
	if len(parts) < 7 {
		return status, fmt.Errorf("invalid PR URL format")
	}

	viewArgs := []string{"pr", "view", prURL, "--json", "number,state,statusCheckRollup"}
	stdout, stderr, err := gh.Exec(viewArgs...)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to check PR status: %s", err))
		logger.Error(stderr.String())
		return status, err
	}

	output := stdout.String()

	if strings.Contains(output, "\"state\": \"open\"") {
		status.State = "open"
	} else if strings.Contains(output, "\"state\": \"closed\"") {
		status.State = "closed"
	} else if strings.Contains(output, "\"state\": \"merged\"") {
		status.State = "merged"
	}

	status.Completed = !strings.Contains(output, "\"status\": \"pending\"") &&
		!strings.Contains(output, "\"status\": \"queued\"") &&
		!strings.Contains(output, "\"status\": \"in_progress\"")

	status.Passing = strings.Contains(output, "\"status\": \"success\"") &&
		!strings.Contains(output, "\"status\": \"failure\"") &&
		!strings.Contains(output, "\"status\": \"cancelled\"")

	return status, nil
}

func WaitForPRChecks(prURL string, timeout time.Duration) (PRStatus, error) {
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
