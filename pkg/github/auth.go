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
		status.Error = err
		return status
	}
	status.CliInstalled = true

	stdout, stderr, err := gh.Exec("auth", "status")
	if err != nil {
		status.Error = fmt.Errorf("auth error: %s: %s", err, stderr.String())
		return status
	}

	output := stdout.String()

	if strings.Contains(output, "Logged in") {
		status.IsAuthenticated = true

		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "Logged in to") && strings.Contains(line, "account") {
				parts := strings.Split(line, "account")
				if len(parts) >= 2 {
					username := strings.TrimSpace(parts[1])
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
