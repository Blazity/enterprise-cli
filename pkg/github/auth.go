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

	_, err := exec.LookPath("gh")
	if err != nil {
		// gh CLI not found
		status.Error = fmt.Errorf("GitHub CLI ('gh') not found in PATH. Please install it: https://cli.github.com/")
		return status
	}
	status.CliInstalled = true

	stdout, stderr, err := gh.Exec("auth", "status")
	if err != nil {
		// Handle cases where gh auth status fails (e.g., not logged in at all)
		// Check if the error message indicates logged out status
		errMsg := stderr.String()
		if strings.Contains(errMsg, "could not determine authentication status") || strings.Contains(errMsg, "No accounts logged in") {
			status.IsAuthenticated = false
			return status // Not an error, just not logged in
		}
		// Otherwise, it's a genuine error
		status.Error = fmt.Errorf("gh auth status error: %s: %s", err, errMsg)
		return status
	}

	output := stdout.String()
	lines := strings.Split(output, "\n")

	var currentHost string
	var currentUsername string
	var activeUsername string

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Check for host line (e.g., "github.com")
		if !strings.HasPrefix(trimmedLine, "-") && strings.Contains(trimmedLine, ".") && !strings.Contains(trimmedLine, "Logged in") {
			currentHost = trimmedLine
			continue // Move to the next line after identifying the host
		}

		// Check for the line indicating a logged-in account
		if strings.Contains(trimmedLine, "Logged in to") && strings.Contains(trimmedLine, "account") {
			parts := strings.SplitN(trimmedLine, "account", 2)
			if len(parts) == 2 {
				usernamePart := strings.TrimSpace(parts[1])
				// Extract username before any parentheses (like '(keyring)')
				if idx := strings.Index(usernamePart, "("); idx > 0 {
					currentUsername = strings.TrimSpace(usernamePart[:idx])
				} else {
					currentUsername = usernamePart
				}
			}
		}

		// Check for the line indicating the active account *for the current host*
		if currentHost != "" && currentUsername != "" && strings.Contains(trimmedLine, "Active account: true") {
			activeUsername = currentUsername // Found the active account for this host
			status.IsAuthenticated = true    // Mark as authenticated if an active account is found
			break                            // Assume only one active account per host, stop searching
		}
	}

	// If an active account was found, set it in the status
	if activeUsername != "" {
		status.Username = activeUsername
	} else if currentUsername != "" && !status.IsAuthenticated {
		// Fallback: If no account is explicitly active, but we found one logged-in account,
		// assume it's the one to use (maintains previous behavior for single-account setups).
		// This might happen if the format changes or only one account is logged in without the 'Active' flag.
		status.Username = currentUsername
		status.IsAuthenticated = true
	}

	// If after parsing, IsAuthenticated is still false, it means no logged-in accounts were found or no active one was identified clearly.
	if !status.IsAuthenticated && status.Error == nil {
		// If no error occurred but not authenticated, explicitly state not logged in.
		// This handles cases where 'gh auth status' runs successfully but reports no logged-in accounts.
		status.Error = fmt.Errorf("not logged in to GitHub. Run 'gh auth login'")
	}

	return status
}
