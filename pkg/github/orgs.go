package github

import (
	"fmt"
	"strings"

	"github.com/blazity/enterprise-cli/pkg/logging"
	"github.com/cli/go-gh"
)

func GetOrganizations() ([]string, error) {
	logger := logging.GetLogger()
	logger.Debug("Fetching organizations...")

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
		username := authStatus.Username
		logger.Debug(fmt.Sprintf("Adding current user %s to organization options", username))
		orgs = append(orgs, username)
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "Showing") {
			orgs = append(orgs, line)
		}
	}

	logger.Debug(fmt.Sprintf("Found %d organizations", len(orgs)))

	return orgs, nil
}
