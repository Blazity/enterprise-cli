package github

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/cli/go-gh"
)

type AuthStatus struct {
	CliInstalled    bool
	IsAuthenticated bool
	Username        string
	Error           error
}

func CheckAuthStatus() AuthStatus {
	status := AuthStatus{
		CliInstalled:    false,
		IsAuthenticated: false,
	}

	// Check if GitHub CLI is installed
	_, err := exec.LookPath("gh")
	if err != nil {
		status.Error = err
		return status
	}
	status.CliInstalled = true

	// Check if user is authenticated using 'gh auth status'
	stdout, stderr, err := gh.Exec("auth", "status")
	if err != nil {
		status.Error = fmt.Errorf("auth error: %s: %s", err, stderr.String())
		return status
	}

	output := stdout.String()

	// Parse output to check authentication
	if strings.Contains(output, "Logged in") {
		status.IsAuthenticated = true

		// Extract username from the output
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			// Handle the format: "✓ Logged in to github.com account neg4n (keyring)"
			if strings.Contains(line, "Logged in to") && strings.Contains(line, "account") {
				// Split by "account" and get the second part
				parts := strings.Split(line, "account")
				if len(parts) >= 2 {
					// Extract username, which is between "account" and potential "(keyring)"
					username := strings.TrimSpace(parts[1])
					// Remove anything in parentheses if present
					if idx := strings.Index(username, "("); idx > 0 {
						username = strings.TrimSpace(username[:idx])
					}
					status.Username = username
					break
				}
			}
		}
	}

	return status
}
