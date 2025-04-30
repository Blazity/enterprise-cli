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
		status.Error = fmt.Errorf("GitHub CLI ('gh') not found in PATH. Please install it: https://cli.github.com/")
		return status
	}
	status.CliInstalled = true

	stdout, stderr, err := gh.Exec("auth", "status")
	if err != nil {
		errMsg := stderr.String()
		if strings.Contains(errMsg, "could not determine authentication status") || strings.Contains(errMsg, "No accounts logged in") {
			status.IsAuthenticated = false
			return status
		}
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

		if !strings.HasPrefix(trimmedLine, "-") && strings.Contains(trimmedLine, ".") && !strings.Contains(trimmedLine, "Logged in") {
			currentHost = trimmedLine
			continue
		}

		if strings.Contains(trimmedLine, "Logged in to") && strings.Contains(trimmedLine, "account") {
			parts := strings.SplitN(trimmedLine, "account", 2)
			if len(parts) == 2 {
				usernamePart := strings.TrimSpace(parts[1])
				if idx := strings.Index(usernamePart, "("); idx > 0 {
					currentUsername = strings.TrimSpace(usernamePart[:idx])
				} else {
					currentUsername = usernamePart
				}
			}
		}

		if currentHost != "" && currentUsername != "" && strings.Contains(trimmedLine, "Active account: true") {
			activeUsername = currentUsername
			status.IsAuthenticated = true
			break
		}
	}

	if activeUsername != "" {
		status.Username = activeUsername
	} else if currentUsername != "" && !status.IsAuthenticated {
		status.Username = currentUsername
		status.IsAuthenticated = true
	}

	if !status.IsAuthenticated && status.Error == nil {
		status.Error = fmt.Errorf("not logged in to GitHub. Run 'gh auth login'")
	}

	return status
}
