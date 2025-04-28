package github

import (
	"fmt"
	"strings"

	"github.com/blazity/enterprise-cli/pkg/logging"
	"github.com/cli/go-gh"
)

// GetOrganizations returns a list of organizations the user belongs to
func GetOrganizations(logger logging.Logger) ([]string, error) {
	logger.Debug("Fetching organizations...")

	// Execute gh org list
	stdout, stderr, err := gh.Exec("org", "list")
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to fetch organizations: %s", err))
		logger.Error(stderr.String())
		return nil, err
	}

	output := stdout.String()

	var orgs []string

	lines := strings.Split(output, "\n")

	authStatus := CheckAuthStatus()
	if authStatus.IsAuthenticated && authStatus.Username != "" {
		// Add the user's personal account as the first option
		username := authStatus.Username
		logger.Debug(fmt.Sprintf("Adding current user %s to organization options", username))
		orgs = append(orgs, username)
	}

	// Parse organization names
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "Showing") {
			orgs = append(orgs, line)
		}
	}

	logger.Debug(fmt.Sprintf("Found %d organizations", len(orgs)))

	return orgs, nil
}
