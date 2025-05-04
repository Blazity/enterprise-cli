package github

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/blazity/enterprise-cli/pkg/logging"
)

// SetRemote adds a new git remote with the given name and URL to the repository at path,
// or updates the URL if the remote already exists. It logs debug information for each step.
func SetRemote(path, name, url string) error {
	logger := logging.GetLogger()
	// Check if remote exists by trying to get its URL
	logger.Debug("Checking for existing git remote", "name", name, "path", path)
	getCmd := exec.Command("git", "-C", path, "remote", "get-url", name)
	output, err := getCmd.CombinedOutput()
	if err != nil {
		errMsg := string(output)
		// remote does not exist
		if strings.Contains(errMsg, fmt.Sprintf("fatal: No such remote '%s'", name)) ||
			strings.Contains(errMsg, fmt.Sprintf("fatal: no such remote '%s'", name)) {
			logger.Debug("Adding new git remote", "name", name, "url", url)
			addCmd := exec.Command("git", "-C", path, "remote", "add", name, url)
			out, addErr := addCmd.CombinedOutput()
			if addErr != nil {
				logger.Error("Failed to add git remote", "name", name, "url", url, "error", addErr)
				logger.Error(string(out))
				return fmt.Errorf("failed to add git remote '%s': %w", name, addErr)
			}
			logger.Info("Added git remote", "name", name, "url", url)
			if logger.IsVerbose() {
				logger.Debug(string(out))
			}
			return nil
		}
		// other error while checking remote
		logger.Error("Failed to get git remote URL", "name", name, "error", err)
		logger.Error(errMsg)
		return fmt.Errorf("failed to get git remote '%s': %w", name, err)
	}
	// remote exists - update its URL
	oldURL := strings.TrimSpace(string(output))
	logger.Debug("Remote exists, updating URL", "name", name, "old-url", oldURL, "new-url", url)
	setCmd := exec.Command("git", "-C", path, "remote", "set-url", name, url)
	out, setErr := setCmd.CombinedOutput()
	if setErr != nil {
		logger.Error("Failed to set git remote URL", "name", name, "url", url, "error", setErr)
		logger.Error(string(out))
		return fmt.Errorf("failed to set git remote URL for '%s': %w", name, setErr)
	}
	if logger.IsVerbose() {
		logger.Debug(string(out))
	}
	return nil
}
