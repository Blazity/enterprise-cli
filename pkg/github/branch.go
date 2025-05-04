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

func CreateBranch(opts BranchOptions) (string, error) {
	logger := logging.GetLogger()

	timestamp := time.Now().Unix()
	finalBranchName := opts.BranchName
	if !containsTimestamp(opts.BranchName) {
		finalBranchName = fmt.Sprintf("%s-%d", opts.BranchName, timestamp)
	}

	logger.Debug(fmt.Sprintf("Ensuring branch %s exists based on %s (local: %t)", finalBranchName, opts.BaseBranch, opts.SkipPull))

	checkoutBaseCmd := exec.Command("git", "-C", opts.Path, "checkout", opts.BaseBranch)
	output, err := checkoutBaseCmd.CombinedOutput()
	if err != nil {
		errMsg := string(output)
		isSuccessMsg := strings.Contains(errMsg, fmt.Sprintf("Already on '%s'", opts.BaseBranch)) ||
			strings.Contains(errMsg, fmt.Sprintf("Switched to branch '%s'", opts.BaseBranch)) ||
			strings.Contains(errMsg, fmt.Sprintf("Switched to a new branch '%s'", opts.BaseBranch))

		if !isSuccessMsg {
			logger.Warning(fmt.Sprintf("Could not checkout base branch %s cleanly: %s", opts.BaseBranch, err))
			logger.Warning(errMsg)
		}
	}

	if !opts.SkipPull {
		logger.Debug(fmt.Sprintf("Pulling latest changes for %s", opts.BaseBranch))
		pullCmd := exec.Command("git", "-C", opts.Path, "pull", "origin", opts.BaseBranch)
		if pullOutput, pullErr := pullCmd.CombinedOutput(); pullErr != nil {
			logger.Error(fmt.Sprintf("Failed to pull latest changes for %s: %s", opts.BaseBranch, pullErr))
			logger.Error(string(pullOutput))
			return "", fmt.Errorf("failed to pull base branch %s: %w", opts.BaseBranch, pullErr)
		}
	} else {
		logger.Debug("Skipping pull step; operations remain local.")
	}

	createBranchCmd := exec.Command("git", "-C", opts.Path, "checkout", "-b", finalBranchName)
	createOutput, createErr := createBranchCmd.CombinedOutput()

	if createErr == nil {
		logger.Debug(fmt.Sprintf("Branch %s created and checked out successfully.", finalBranchName))
		return finalBranchName, nil
	}

	createErrMsg := string(createOutput)
	if strings.Contains(createErrMsg, fmt.Sprintf("fatal: A branch named '%s' already exists.", finalBranchName)) ||
		strings.Contains(createErrMsg, fmt.Sprintf("fatal: branch '%s' already exists.", finalBranchName)) {
		logger.Debug(fmt.Sprintf("Branch %s already exists, switching to it.", finalBranchName))

		switchCmd := exec.Command("git", "-C", opts.Path, "checkout", finalBranchName)
		switchOutput, switchErr := switchCmd.CombinedOutput()
		switchMsg := string(switchOutput)

		isSuccessMsg := strings.Contains(switchMsg, fmt.Sprintf("Already on '%s'", finalBranchName)) ||
			strings.Contains(switchMsg, fmt.Sprintf("Switched to branch '%s'", finalBranchName))

		if switchErr == nil || isSuccessMsg {
			logger.Debug(fmt.Sprintf("Switched to existing branch %s.", finalBranchName))
			return finalBranchName, nil
		} else {
			logger.Debug(fmt.Sprintf("Failed to switch to existing branch %s: %s", finalBranchName, switchErr))
			logger.Error(switchMsg)
			return "", fmt.Errorf("branch '%s' exists but could not be checked out: %w", finalBranchName, switchErr)
		}

	}

	logger.Error(fmt.Sprintf("Failed to create or checkout branch %s: %s", finalBranchName, createErr))
	logger.Error(createErrMsg)
	return "", fmt.Errorf("failed to create branch '%s': %w", finalBranchName, createErr)
}

func GetCurrentBranch(path string) (string, error) {
	logger := logging.GetLogger()

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

func CommitChanges(path string, message string, files []string) error {
	logger := logging.GetLogger()

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

	commitCmd := exec.Command("git", "-C", path, "commit", "-m", message)
	output, err := commitCmd.CombinedOutput()
	if err != nil {
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

func PushBranch(path string, branch string, remote string) error {
	logger := logging.GetLogger()

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

func containsTimestamp(branchName string) bool {
	parts := strings.Split(branchName, "-")
	if len(parts) <= 1 {
		return false
	}

	lastPart := parts[len(parts)-1]
	if len(lastPart) < 8 {
		return false
	}
	_, err := strconv.ParseInt(lastPart, 10, 64)
	return err == nil
}
