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
	BranchName string // Base name, timestamp might be added
	BaseBranch string
	SkipPull   bool
}

// CreateBranch ensures the desired branch exists and is checked out.
// It appends a timestamp to BranchName if it doesn't already contain one.
// Returns the actual final branch name used (potentially with timestamp) and an error if any.
func CreateBranch(opts BranchOptions, logger logging.Logger) (string, error) {
	// Calculate final branch name (append timestamp if needed)
	timestamp := time.Now().Unix()
	finalBranchName := opts.BranchName
	if !containsTimestamp(opts.BranchName) {
		finalBranchName = fmt.Sprintf("%s-%d", opts.BranchName, timestamp)
	}

	logger.Debug(fmt.Sprintf("Ensuring branch %s exists based on %s (local: %t)", finalBranchName, opts.BaseBranch, opts.SkipPull))

	// 1. Checkout base branch
	checkoutBaseCmd := exec.Command("git", "-C", opts.Path, "checkout", opts.BaseBranch)
	output, err := checkoutBaseCmd.CombinedOutput()
	if err != nil {
		// Ignore "Already on '...'" message, warn otherwise
		errMsg := string(output)
		// Check for common success messages that might go to stderr
		isSuccessMsg := strings.Contains(errMsg, fmt.Sprintf("Already on '%s'", opts.BaseBranch)) ||
			strings.Contains(errMsg, fmt.Sprintf("Switched to branch '%s'", opts.BaseBranch)) ||
			strings.Contains(errMsg, fmt.Sprintf("Switched to a new branch '%s'", opts.BaseBranch)) // Handle if base branch itself was just created

		if !isSuccessMsg {
			logger.Warning(fmt.Sprintf("Could not checkout base branch %s cleanly: %s", opts.BaseBranch, err))
			logger.Warning(errMsg)
			// Don't return error here, pull/create might still work if base exists remotely or locally
		}
	}

	// 2. Pull latest changes for base branch (if not skipped)
	if !opts.SkipPull {
		logger.Debug(fmt.Sprintf("Pulling latest changes for %s", opts.BaseBranch))
		pullCmd := exec.Command("git", "-C", opts.Path, "pull", "origin", opts.BaseBranch)
		if pullOutput, pullErr := pullCmd.CombinedOutput(); pullErr != nil {
			logger.Error(fmt.Sprintf("Failed to pull latest changes for %s: %s", opts.BaseBranch, pullErr))
			logger.Error(string(pullOutput))
			// Proceed even if pull fails? Or return error? Let's return error for now.
			return "", fmt.Errorf("failed to pull base branch %s: %w", opts.BaseBranch, pullErr)
		}
	} else {
		logger.Debug("Skipping pull step; operations remain local.")
	}

	// 3. Try creating the new branch from the current HEAD (which should be BaseBranch)
	createBranchCmd := exec.Command("git", "-C", opts.Path, "checkout", "-b", finalBranchName)
	createOutput, createErr := createBranchCmd.CombinedOutput()

	if createErr == nil {
		// Success! Branch created and checked out.
		logger.Debug(fmt.Sprintf("Branch %s created and checked out successfully.", finalBranchName))
		return finalBranchName, nil
	}

	// Check if the error is because the branch already exists
	createErrMsg := string(createOutput)
	if strings.Contains(createErrMsg, fmt.Sprintf("fatal: A branch named '%s' already exists.", finalBranchName)) ||
		strings.Contains(createErrMsg, fmt.Sprintf("fatal: branch '%s' already exists.", finalBranchName)) { // Git versions might differ
		logger.Debug(fmt.Sprintf("Branch %s already exists, switching to it.", finalBranchName))

		// 4. If creation failed because it exists, just check it out
		switchCmd := exec.Command("git", "-C", opts.Path, "checkout", finalBranchName)
		switchOutput, switchErr := switchCmd.CombinedOutput()
		switchMsg := string(switchOutput)

		// Check if the switch command actually worked (it might print to stderr on success e.g. "Already on...")
		isSuccessMsg := strings.Contains(switchMsg, fmt.Sprintf("Already on '%s'", finalBranchName)) ||
			strings.Contains(switchMsg, fmt.Sprintf("Switched to branch '%s'", finalBranchName))

		if switchErr == nil || isSuccessMsg {
			logger.Debug(fmt.Sprintf("Switched to existing branch %s.", finalBranchName))
			return finalBranchName, nil
		} else {
			// Unknown state after checkout attempt
			logger.Debug(fmt.Sprintf("Failed to switch to existing branch %s: %s", finalBranchName, switchErr))
			logger.Error(switchMsg)
			return "", fmt.Errorf("branch '%s' exists but could not be checked out: %w", finalBranchName, switchErr)
		}

	}

	// 5. If it was some other error during creation
	logger.Error(fmt.Sprintf("Failed to create or checkout branch %s: %s", finalBranchName, createErr))
	logger.Error(createErrMsg)
	return "", fmt.Errorf("failed to create branch '%s': %w", finalBranchName, createErr)
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

	return nil
}

func PushBranch(path string, branch string, remote string, logger logging.Logger) error {
	if remote == "" {
		remote = "origin"
	}

	logger.Debug(fmt.Sprintf("Pushing branch %s to %s", branch, remote))

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

	logger.Debug("Branch pushed successfully")

	return nil
}

// Helper function to check if branch name already contains a timestamp-like suffix
func containsTimestamp(branchName string) bool {
	parts := strings.Split(branchName, "-")
	if len(parts) <= 1 {
		return false
	}

	// Check if last part is numeric and long enough to likely be a timestamp
	lastPart := parts[len(parts)-1]
	// Heuristic: avoid matching things like "v1" or "issue-123" by checking length
	// Unix timestamp in seconds is usually 10 digits. Let's use >= 8 as a loose check.
	if len(lastPart) < 8 {
		return false
	}
	_, err := strconv.ParseInt(lastPart, 10, 64) // Use ParseInt for robustness (handles potential leading zeros if any)
	return err == nil
}
