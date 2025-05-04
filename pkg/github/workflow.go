package github

import (
	"fmt"
	"strings"
	"time"

	"github.com/blazity/enterprise-cli/pkg/logging"
	"github.com/cli/go-gh"
)

type WorkflowRun struct {
	ID         string
	Name       string
	Status     string
	Conclusion string
	URL        string
	CreatedAt  string
	UpdatedAt  string
}

func GetWorkflowRuns(repo string, branch string) ([]WorkflowRun, error) {
	logger := logging.GetLogger()
	logger.Debug(fmt.Sprintf("Getting workflow runs for %s branch %s", repo, branch))

	args := []string{"run", "list", "--repo", repo}

	if branch != "" {
		args = append(args, "--branch", branch)
	}

	args = append(args, "--json", "databaseId,name,status,conclusion,url,createdAt,updatedAt")

	stdout, stderr, err := gh.Exec(args...)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get workflow runs: %s", err))
		logger.Error(stderr.String())
		return nil, err
	}

	output := stdout.String()

	var runs []WorkflowRun
	lines := strings.Split(output, "}")
	for _, line := range lines {
		if !strings.Contains(line, "databaseId") {
			continue
		}

		run := WorkflowRun{}

		if idParts := strings.Split(line, "\"databaseId\": "); len(idParts) > 1 {
			idStr := strings.Split(idParts[1], ",")[0]
			run.ID = strings.TrimSpace(idStr)
		}

		if nameParts := strings.Split(line, "\"name\": \""); len(nameParts) > 1 {
			name := strings.Split(nameParts[1], "\"")[0]
			run.Name = name
		}

		if statusParts := strings.Split(line, "\"status\": \""); len(statusParts) > 1 {
			status := strings.Split(statusParts[1], "\"")[0]
			run.Status = status
		}

		if conclParts := strings.Split(line, "\"conclusion\": \""); len(conclParts) > 1 {
			concl := strings.Split(conclParts[1], "\"")[0]
			run.Conclusion = concl
		}

		if urlParts := strings.Split(line, "\"url\": \""); len(urlParts) > 1 {
			url := strings.Split(urlParts[1], "\"")[0]
			run.URL = url
		}

		if dateParts := strings.Split(line, "\"createdAt\": \""); len(dateParts) > 1 {
			date := strings.Split(dateParts[1], "\"")[0]
			run.CreatedAt = date
		}

		if dateParts := strings.Split(line, "\"updatedAt\": \""); len(dateParts) > 1 {
			date := strings.Split(dateParts[1], "\"")[0]
			run.UpdatedAt = date
		}

		if run.ID != "" {
			runs = append(runs, run)
		}
	}

	logger.Debug(fmt.Sprintf("Found %d workflow runs", len(runs)))

	return runs, nil
}

func GetLatestWorkflowRun(repo string, branch string) (*WorkflowRun, error) {
	logger := logging.GetLogger()
	runs, err := GetWorkflowRuns(repo, branch)
	if err != nil {
		return nil, err
	}

	if len(runs) == 0 {
		logger.Info("No workflow runs found")
		return nil, nil
	}

	latestRun := runs[0]

	logger.Debug(fmt.Sprintf("Latest workflow run: %s (Status: %s, Conclusion: %s)",
		latestRun.Name, latestRun.Status, latestRun.Conclusion))

	return &latestRun, nil
}

func WaitForWorkflowRun(repo string, branch string, timeout time.Duration) (*WorkflowRun, error) {
	logger := logging.GetLogger()
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		run, err := GetLatestWorkflowRun(repo, branch)
		if err != nil {
			return nil, err
		}

		if run == nil {
			logger.Debug("No workflow runs found yet, waiting...")
			time.Sleep(10 * time.Second)
			continue
		}

		if run.Status == "completed" {
			logger.Info(fmt.Sprintf("Workflow run completed with conclusion: %s", run.Conclusion))
			return run, nil
		}

		logger.Debug(fmt.Sprintf("Workflow run status: %s, waiting...", run.Status))
		time.Sleep(30 * time.Second)
	}

	return nil, fmt.Errorf("timeout waiting for workflow run to complete")
}

func CancelWorkflowRun(runID string, repo string) error {
	logger := logging.GetLogger()
	logger.Debug(fmt.Sprintf("Cancelling workflow run %s", runID))

	args := []string{"run", "cancel", runID, "--repo", repo}
	_, stderr, err := gh.Exec(args...)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to cancel workflow run: %s", err))
		logger.Error(stderr.String())
		return err
	}

	logger.Info("Workflow run cancellation requested")

	return nil
}

func GetWorkflowRunLogs(runID string, repo string) (string, error) {
	logger := logging.GetLogger()
	logger.Debug(fmt.Sprintf("Getting logs for workflow run %s", runID))

	args := []string{"run", "view", runID, "--repo", repo, "--log"}
	stdout, stderr, err := gh.Exec(args...)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get workflow logs: %s", err))
		logger.Error(stderr.String())
		return "", err
	}

	return stdout.String(), nil
}

func ExtractRepoFromURL(url string) (string, error) {
	parts := strings.Split(url, "github.com/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid GitHub URL format")
	}

	urlPath := parts[1]
	pathParts := strings.Split(urlPath, "/")

	if len(pathParts) < 2 {
		return "", fmt.Errorf("URL does not contain owner/repo parts")
	}

	owner := pathParts[0]
	repo := pathParts[1]

	repo = strings.TrimSuffix(repo, ".git")

	return fmt.Sprintf("%s/%s", owner, repo), nil
}

func GetWorkflowRunByID(runID string, repo string) (*WorkflowRun, error) {
	logger := logging.GetLogger()
	logger.Debug(fmt.Sprintf("Getting workflow run %s by ID", runID))

	args := []string{"run", "view", runID, "--repo", repo, "--json", "databaseId,name,status,conclusion,url,createdAt,updatedAt"}
	stdout, stderr, err := gh.Exec(args...)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to get workflow run: %s", err))
		logger.Error(stderr.String())
		return nil, err
	}

	output := stdout.String()

	run := WorkflowRun{}

	if idParts := strings.Split(output, "\"databaseId\": "); len(idParts) > 1 {
		idStr := strings.Split(idParts[1], ",")[0]
		run.ID = strings.TrimSpace(idStr)
	}

	if nameParts := strings.Split(output, "\"name\": \""); len(nameParts) > 1 {
		name := strings.Split(nameParts[1], "\"")[0]
		run.Name = name
	}

	if statusParts := strings.Split(output, "\"status\": \""); len(statusParts) > 1 {
		status := strings.Split(statusParts[1], "\"")[0]
		run.Status = status
	}

	if conclParts := strings.Split(output, "\"conclusion\": \""); len(conclParts) > 1 {
		concl := strings.Split(conclParts[1], "\"")[0]
		run.Conclusion = concl
	}

	if urlParts := strings.Split(output, "\"url\": \""); len(urlParts) > 1 {
		url := strings.Split(urlParts[1], "\"")[0]
		run.URL = url
	}

	return &run, nil
}
