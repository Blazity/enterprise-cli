package command

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/blazity/enterprise-cli/pkg/github"
	"github.com/blazity/enterprise-cli/pkg/logging"
	"github.com/blazity/enterprise-cli/pkg/ui"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

func NewGitHubCommand(logger logging.Logger, ctx context.Context, cancel context.CancelFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "github",
		Short: "GitHub operations for enterprise deployments",
		Long:  "GitHub operations for cloning, preparing and deploying enterprise infrastructure",
	}

	cmd.AddCommand(newGitHubCloneCommand(logger, ctx))
	cmd.AddCommand(newGitHubPrepareCommand(logger, ctx, cancel))
	cmd.AddCommand(newGitHubStatusCommand(logger, ctx))

	return cmd
}

func newGitHubCloneCommand(logger logging.Logger, ctx context.Context) *cobra.Command {
	var depth int
	var destination string
	var branch string

	cmd := &cobra.Command{
		Use:   "clone [repository]",
		Short: "Clone a GitHub repository",
		Long:  "Clone a GitHub repository for deployment preparation",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			repository := args[0]

			// Check GitHub authentication
			authStatus := github.CheckAuthStatus()
			if !authStatus.CliInstalled {
				logger.Error("GitHub CLI is not installed. Please install it first.")
				return
			}

			if !authStatus.IsAuthenticated {
				logger.Error("Not authenticated with GitHub. Please run 'gh auth login' first.")
				return
			}

			logger.Info(fmt.Sprintf("Authenticated as %s", authStatus.Username))

			// Clone repository
			opts := github.CloneOptions{
				Repository:  repository,
				Depth:       depth,
				Destination: destination,
				Branch:      branch,
			}

			if err := github.CloneRepository(opts, logger); err != nil {
				logger.Error(fmt.Sprintf("Failed to clone repository: %s", err))
				return
			}

			logger.Info(fmt.Sprintf("Repository %s cloned successfully", repository))
		},
	}

	cmd.Flags().IntVar(&depth, "depth", 1, "Create a shallow clone with the specified depth")
	cmd.Flags().StringVar(&destination, "destination", "", "Directory to clone into")
	cmd.Flags().StringVar(&branch, "branch", "", "Branch to checkout")

	return cmd
}

func newGitHubPrepareCommand(logger logging.Logger, ctx context.Context, cancel context.CancelFunc) *cobra.Command {
	var targetRepo string
	var baseBranch string
	var repoPath string
	var prTitle string
	var prBody string

	cmd := &cobra.Command{
		Use:   "prepare",
		Short: "Prepare a GitHub repository for deployment",
		Long:  "Create a branch, make changes, and create a PR for deployment",
		Run: func(cmd *cobra.Command, args []string) {
			// Check GitHub authentication
			authStatus := github.CheckAuthStatus()
			if !authStatus.CliInstalled {
				logger.Error("GitHub CLI is not installed. Please install it first.")
				return
			}

			if !authStatus.IsAuthenticated {
				logger.Error("Not authenticated with GitHub. Please run 'gh auth login' first.")
				return
			}

			logger.Info(fmt.Sprintf("Authenticated as %s", authStatus.Username))

			// Form for repository preparation
			var sourceRepo string
			var branchName string
			var selectedFiles string

			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Source Repository").
						Description("The source repository (e.g., owner/repo)").
						Placeholder("owner/repo").
						Value(&sourceRepo),
					huh.NewInput().
						Title("Target Repository").
						Description("The target repository for the PR").
						Placeholder("owner/repo").
						Value(&targetRepo),
					huh.NewInput().
						Title("Repository Path").
						Description("Path to the local repository").
						Placeholder("/path/to/repo").
						Value(&repoPath),
					huh.NewInput().
						Title("Branch Name").
						Description("Name for the new branch").
						Placeholder("feature/my-changes").
						Value(&branchName),
					huh.NewInput().
						Title("Base Branch").
						Description("Base branch to create from").
						Placeholder("main").
						Value(&baseBranch),
					huh.NewInput().
						Title("PR Title").
						Description("Title for the pull request").
						Placeholder("Add new feature").
						Value(&prTitle),
					huh.NewText().
						Title("PR Body").
						Description("Description for the pull request").
						Placeholder("This PR adds...").
						Value(&prBody),
					huh.NewText().
						Title("Files to Commit").
						Description("Comma-separated list of files to commit").
						Placeholder("file1.go,file2.go").
						Value(&selectedFiles),
				),
			).WithWidth(80).WithShowHelp(true)

			// Pass the cancel function to RunForm
			if err := ui.RunForm(form, logger, cancel); err != nil {
				// Handle potential cancellation error from RunForm
				if errors.Is(err, ui.ErrFormCancelled) {
					logger.Info("Operation cancelled by user during GitHub preparation form.")
					return // Exit cleanly on cancellation
				}
				// Handle other form errors
				// Format the error message before logging
				formattedError := fmt.Sprintf("Failed to collect GitHub preparation information: %v", err)
				logger.Error(formattedError)
				return
			}

			// Create branch
			branchOpts := github.BranchOptions{
				Path:       repoPath,
				BranchName: branchName,
				BaseBranch: baseBranch,
			}

			if err := github.CreateBranch(branchOpts, logger); err != nil {
				logger.Error(fmt.Sprintf("Failed to create branch: %s", err))
				return
			}

			// Commit changes
			commitMessage := fmt.Sprintf("Prepare deployment for %s", targetRepo)
			filesList := []string{}

			if selectedFiles != "" {
				filesList = strings.Split(selectedFiles, ",")
				// Trim spaces
				for i, file := range filesList {
					filesList[i] = strings.TrimSpace(file)
				}
			}

			if err := github.CommitChanges(repoPath, commitMessage, filesList, logger); err != nil {
				logger.Error(fmt.Sprintf("Failed to commit changes: %s", err))
				return
			}

			// Push branch
			currentBranch, err := github.GetCurrentBranch(repoPath, logger)
			if err != nil {
				logger.Error(fmt.Sprintf("Failed to get current branch: %s", err))
				return
			}

			if err := github.PushBranch(repoPath, currentBranch, "origin", logger); err != nil {
				logger.Error(fmt.Sprintf("Failed to push branch: %s", err))
				return
			}

			// Create PR
			prOpts := github.PROptions{
				SourceRepo:          sourceRepo,
				TargetRepo:          targetRepo,
				BranchName:          currentBranch,
				BaseBranch:          baseBranch,
				Title:               prTitle,
				Body:                prBody,
				CommitMessage:       commitMessage,
				ChangesToCommit:     filesList,
				ModifiedDestination: repoPath,
			}

			prURL, err := github.PreparePullRequest(prOpts, logger)
			if err != nil {
				logger.Error(fmt.Sprintf("Failed to create PR: %s", err))
				return
			}

			logger.Info(fmt.Sprintf("Pull request created successfully: %s", prURL))
		},
	}

	cmd.Flags().StringVar(&targetRepo, "target", "", "Target repository for the PR")
	cmd.Flags().StringVar(&baseBranch, "base", "main", "Base branch to create from")
	cmd.Flags().StringVar(&repoPath, "path", "", "Path to the local repository")
	cmd.Flags().StringVar(&prTitle, "title", "Prepare deployment", "Title for the pull request")
	cmd.Flags().StringVar(&prBody, "body", "This PR prepares the deployment", "Body for the pull request")

	return cmd
}

func newGitHubStatusCommand(logger logging.Logger, ctx context.Context) *cobra.Command {
	var timeout int
	var repo string
	var branch string

	cmd := &cobra.Command{
		Use:   "status [pr-url]",
		Short: "Check the status of a GitHub PR or workflow",
		Long:  "Check the status of a GitHub PR and its workflow runs, or check workflows for a branch",
		Args:  cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Check GitHub authentication
			authStatus := github.CheckAuthStatus()
			if !authStatus.CliInstalled {
				logger.Error("GitHub CLI is not installed. Please install it first.")
				return
			}

			if !authStatus.IsAuthenticated {
				logger.Error("Not authenticated with GitHub. Please run 'gh auth login' first.")
				return
			}

			logger.Info(fmt.Sprintf("Authenticated as %s", authStatus.Username))

			// Check if we're checking a PR or a workflow
			if len(args) > 0 && strings.Contains(args[0], "pull") {
				// This is a PR URL
				prURL := args[0]
				logger.Info(fmt.Sprintf("Checking PR status for %s", prURL))

				// Check PR status
				status, err := github.CheckPRStatus(prURL, logger)
				if err != nil {
					logger.Error(fmt.Sprintf("Failed to check PR status: %s", err))
					return
				}

				logger.Info(fmt.Sprintf("PR Status: %s", status.State))

				if status.Completed {
					if status.Passing {
						logger.Info("All checks have passed!")
					} else {
						logger.Warning("Some checks have failed.")
					}
				} else {
					logger.Info("Checks are still running...")

					if timeout > 0 {
						logger.Info(fmt.Sprintf("Waiting for checks to complete (timeout: %d minutes)...", timeout))
						timeoutDuration := time.Duration(timeout) * time.Minute

						status, err = github.WaitForPRChecks(prURL, timeoutDuration, logger)
						if err != nil {
							logger.Error(fmt.Sprintf("Failed to wait for PR checks: %s", err))
							return
						}

						if status.Passing {
							logger.Info("All checks have passed!")
						} else {
							logger.Warning("Some checks have failed.")
						}
					}
				}
			} else if repo != "" {
				// We're checking workflow runs for a repository
				logger.Info(fmt.Sprintf("Checking workflow runs for %s", repo))

				// If a branch is specified, filter by branch
				if branch != "" {
					logger.Info(fmt.Sprintf("Filtering by branch: %s", branch))
				}

				// Get workflow runs
				runs, err := github.GetWorkflowRuns(repo, branch, logger)
				if err != nil {
					logger.Error(fmt.Sprintf("Failed to get workflow runs: %s", err))
					return
				}

				if len(runs) == 0 {
					logger.Info("No workflow runs found")
					return
				}

				// Display latest runs
				logger.Info(fmt.Sprintf("Latest workflow runs for %s:", repo))
				for i, run := range runs {
					if i >= 3 { // Only show the 3 most recent runs
						break
					}
					status := run.Status
					if run.Status == "completed" {
						status = run.Conclusion
					}
					logger.Info(fmt.Sprintf("%s: %s (%s)", run.Name, status, run.URL))
				}

				// Wait for latest run if requested
				if timeout > 0 && len(runs) > 0 {
					latestRun := runs[0]
					if latestRun.Status != "completed" {
						logger.Info(fmt.Sprintf("Waiting for latest run to complete (timeout: %d minutes)...", timeout))
						timeoutDuration := time.Duration(timeout) * time.Minute

						run, err := github.WaitForWorkflowRun(repo, branch, timeoutDuration, logger)
						if err != nil {
							logger.Error(fmt.Sprintf("Failed to wait for workflow run: %s", err))
							return
						}

						if run.Conclusion == "success" {
							logger.Info("Workflow run completed successfully!")
						} else {
							logger.Warning(fmt.Sprintf("Workflow run completed with status: %s", run.Conclusion))
						}
					}
				}
			} else {
				logger.Error("Please provide either a PR URL or --repo flag")
			}
		},
	}

	cmd.Flags().IntVar(&timeout, "wait", 0, "Wait for checks to complete (timeout in minutes, 0 = don't wait)")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository to check workflows for (owner/repo format)")
	cmd.Flags().StringVar(&branch, "branch", "", "Branch to check workflows for")

	return cmd
}
