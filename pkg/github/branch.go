package github

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/blazity/enterprise-cli/pkg/logging"
)

type BranchOptions struct {
	Path       string
	BranchName string
	BaseBranch string
	SkipPull   bool
}

func CreateBranch(opts BranchOptions, logger logging.Logger) error {
	if opts.SkipPull {
		logger.Info(fmt.Sprintf("Local-only: Creating branch %s from %s", opts.BranchName, opts.BaseBranch))
	} else {
		logger.Info(fmt.Sprintf("Creating branch %s from %s", opts.BranchName, opts.BaseBranch))
	}

	// First, ensure we're on the base branch and it's up to date
	// Execute git commands directly instead of through gh

	// Check if branch already exists
	checkBranchCmd := exec.Command("git", "-C", opts.Path, "branch")
	checkBranchOutput, err := checkBranchCmd.CombinedOutput()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to check branches: %s", err))
		return err
	}

	// If branch already exists, switch to it and return
	if strings.Contains(string(checkBranchOutput), opts.BranchName) {
		logger.Info(fmt.Sprintf("Branch %s already exists, switching to it", opts.BranchName))
		switchCmd := exec.Command("git", "-C", opts.Path, "checkout", opts.BranchName)
		if output, err := switchCmd.CombinedOutput(); err != nil {
			logger.Error(fmt.Sprintf("Failed to switch to existing branch: %s", err))
			logger.Error(string(output))
			return err
		}
		return nil
	}

	// Checkout base branch
	checkoutCmd := exec.Command("git", "-C", opts.Path, "checkout", opts.BaseBranch)
	if output, err := checkoutCmd.CombinedOutput(); err != nil {
		logger.Error(fmt.Sprintf("Failed to checkout base branch: %s", err))
		logger.Error(string(output))
		return err
	}

	// Pull latest changes (quietly to avoid unnecessary output)
	if !opts.SkipPull {
		logger.Info(fmt.Sprintf("Pulling latest changes from origin/%s", opts.BaseBranch))
		pullCmd := exec.Command("git", "-C", opts.Path, "pull", "-q", "origin", opts.BaseBranch)
		if output, err := pullCmd.CombinedOutput(); err != nil {
			logger.Error(fmt.Sprintf("Failed to pull latest changes: %s", err))
			logger.Error(string(output))
			return err
		}
	} else {
		logger.Info("Skipping pull step; all git operations will remain local")
	}

	// Create new branch with timestamp if needed
	timestamp := time.Now().Unix()
	finalBranchName := opts.BranchName

	// If branch name doesn't include timestamp or unique ID, add timestamp
	if !containsTimestamp(opts.BranchName) {
		finalBranchName = fmt.Sprintf("%s-%d", opts.BranchName, timestamp)
	}

	// Create and checkout the new branch
	createBranchCmd := exec.Command("git", "-C", opts.Path, "checkout", "-b", finalBranchName)
	if output, err := createBranchCmd.CombinedOutput(); err != nil {
		logger.Error(fmt.Sprintf("Failed to create branch: %s", err))
		logger.Error(string(output))
		return err
	}

	logger.Info(fmt.Sprintf("Branch %s created successfully", finalBranchName))

	return nil
}

func GetCurrentBranch(path string, logger logging.Logger) (string, error) {
	cmd := exec.Command("git", "-C", path, "branch", "--show-current")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get current branch: %s", err))
		logger.Error(string(output))
		return "", err
	}

	branch := strings.TrimSpace(string(output))
	return branch, nil
}

func CommitChanges(path string, message string, files []string, logger logging.Logger) error {
	logger.Info("Committing changes")

	// Add specific files if provided, otherwise add all
	var addCmd *exec.Cmd
	if len(files) > 0 {
		args := append([]string{"-C", path, "add"}, files...)
		addCmd = exec.Command("git", args...)
	} else {
		addCmd = exec.Command("git", "-C", path, "add", ".")
	}

	if output, err := addCmd.CombinedOutput(); err != nil {
		logger.Error(fmt.Sprintf("Failed to stage changes: %s", err))
		logger.Error(string(output))
		return err
	}

	// Commit changes
	commitCmd := exec.Command("git", "-C", path, "commit", "-m", message)
	output, err := commitCmd.CombinedOutput()
	if err != nil {
		// Check if error is "nothing to commit"
		if strings.Contains(string(output), "nothing to commit") {
			logger.Info("No changes to commit")
			return nil
		}

		logger.Error(fmt.Sprintf("Failed to commit changes: %s", err))
		logger.Error(string(output))
		return err
	}

	if logger.IsVerbose() {
		logger.Debug(string(output))
	}

	logger.Info("Changes committed successfully")

	return nil
}

func PushBranch(path string, branch string, remote string, logger logging.Logger) error {
	if remote == "" {
		remote = "origin"
	}

	logger.Info(fmt.Sprintf("Pushing branch %s to %s", branch, remote))

	pushCmd := exec.Command("git", "-C", path, "push", "-u", remote, branch)
	output, err := pushCmd.CombinedOutput()
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to push changes: %s", err))
		logger.Error(string(output))
		return err
	}

	if logger.IsVerbose() {
		logger.Debug(string(output))
	}

	logger.Info("Branch pushed successfully")

	return nil
}

// Helper function to check if branch name already contains a timestamp-like suffix
func containsTimestamp(branchName string) bool {
	parts := strings.Split(branchName, "-")
	if len(parts) <= 1 {
		return false
	}

	// Check if last part is numeric
	lastPart := parts[len(parts)-1]
	_, err := strconv.Atoi(lastPart)
	return err == nil
}
