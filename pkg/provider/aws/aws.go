package aws

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blazity/enterprise-cli/pkg/codemod"
	"github.com/blazity/enterprise-cli/pkg/github"
	"github.com/blazity/enterprise-cli/pkg/logging"
	"github.com/blazity/enterprise-cli/pkg/provider"
	"github.com/blazity/enterprise-cli/pkg/resources"
	"github.com/blazity/enterprise-cli/pkg/ui"
	"github.com/blazity/enterprise-cli/pkg/utils/filesystem"
	"github.com/charmbracelet/huh"
	"github.com/cli/go-gh"
)

func init() {
	provider.Register("aws", &AwsProviderFactory{})
}

type AwsProviderFactory struct{}

func (f *AwsProviderFactory) Create() provider.Provider {
	return &AwsProvider{}
}

type AwsProvider struct {
	cancel          context.CancelFunc
	region          string
	accessKeyID     string
	secretAccessKey string
	repositoryName  string
	organization    string
	isPrivate       bool
	tempDir         string
	cancelled       bool
	activeBranch    string
}

func (p *AwsProvider) SetCancelFunc(cancel context.CancelFunc) {
	p.cancel = cancel
}

func (p *AwsProvider) GetName() string {
	return "aws"
}

func (p *AwsProvider) Prepare() error {
	return p.PrepareWithContext(context.Background())
}

func (p *AwsProvider) PrepareWithContext(ctx context.Context) error {
	if ctx == nil {
		logging.GetLogger().Error("Context is nil, using background context as fallback")
		ctx = context.Background()
	}

	cancelled := ctx.Done()

	checkCancelled := func() bool {
		select {
		case <-cancelled:
			logging.GetLogger().Info("Operation cancelled via context")
			return true
		default:
			return false
		}
	}

	if checkCancelled() {
		logging.GetLogger().Info("Operation was cancelled before starting AWS preparation")
		return fmt.Errorf("operation cancelled by user")
	}

	logging.GetLogger().Info("Collecting information...")
	logging.GetLogger().Debug("Fetching available organizations...")
	organizations, err := github.GetOrganizations()
	if err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to fetch organizations: %s", err))
		return err
	}

	if len(organizations) == 0 {
		logging.GetLogger().Error("No organizations found, and couldn't determine the username")
		return fmt.Errorf("no organizations found")
	}

	p.organization = organizations[0]
	p.isPrivate = true

	organizationsLength := 0
	organizationsLength = min(len(organizations), 8)

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("AWS Region").
				Description("The AWS region to deploy").
				Options(
					huh.NewOption("US East (N. Virginia)", "us-east-1"),
					huh.NewOption("US East (Ohio)", "us-east-2"),
					huh.NewOption("US West (N. California)", "us-west-1"),
					huh.NewOption("US West (Oregon)", "us-west-2"),
					huh.NewOption("Canada (Central)", "ca-central-1"),
					huh.NewOption("Canada West (Calgary)", "ca-west-1"),
					huh.NewOption("AWS GovCloud (US-East)", "us-gov-east-1"),
					huh.NewOption("AWS GovCloud (US-West)", "us-gov-west-1"),
					huh.NewOption("Europe (Ireland)", "eu-west-1"),
					huh.NewOption("Europe (London)", "eu-west-2"),
					huh.NewOption("Europe (Paris)", "eu-west-3"),
					huh.NewOption("Europe (Frankfurt)", "eu-central-1"),
					huh.NewOption("Europe (Zurich)", "eu-central-2"),
					huh.NewOption("Europe (Stockholm)", "eu-north-1"),
					huh.NewOption("Europe (Milan)", "eu-south-1"),
					huh.NewOption("Europe (Spain)", "eu-south-2"),
					huh.NewOption("Africa (Cape Town)", "af-south-1"),
					huh.NewOption("Asia Pacific (Hong Kong)", "ap-east-1"),
					huh.NewOption("Asia Pacific (Tokyo)", "ap-northeast-1"),
					huh.NewOption("Asia Pacific (Seoul)", "ap-northeast-2"),
					huh.NewOption("Asia Pacific (Osaka)", "ap-northeast-3"),
					huh.NewOption("Asia Pacific (Mumbai)", "ap-south-1"),
					huh.NewOption("Asia Pacific (Hyderabad)", "ap-south-2"),
					huh.NewOption("Asia Pacific (Singapore)", "ap-southeast-1"),
					huh.NewOption("Asia Pacific (Sydney)", "ap-southeast-2"),
					huh.NewOption("Asia Pacific (Jakarta)", "ap-southeast-3"),
					huh.NewOption("Asia Pacific (Melbourne)", "ap-southeast-4"),
					huh.NewOption("Asia Pacific (Malaysia)", "ap-southeast-5"),
					huh.NewOption("Asia Pacific (Thailand)", "ap-southeast-7"),
					huh.NewOption("China (Beijing)", "cn-north-1"),
					huh.NewOption("China (Ningxia)", "cn-northwest-1"),
					huh.NewOption("Israel (Tel Aviv)", "il-central-1"),
					huh.NewOption("Middle East (UAE)", "me-central-1"),
					huh.NewOption("Middle East (Bahrain)", "me-south-1"),
					huh.NewOption("Mexico (Central)", "mx-central-1"),
					huh.NewOption("South America (Sao Paulo)", "sa-east-1"),
				).
				Height(8).
				Value(&p.region),
			huh.NewInput().
				Title("Repository Name").
				Description("Name for the new GitHub repository").
				Placeholder("my-aws-project").
				Value(&p.repositoryName),
			huh.NewSelect[string]().
				Title("Repository Owner").
				Description("GitHub organization or username for the new repository\n").
				Options(
					huh.NewOptions(organizations...)...,
				).
				Height(organizationsLength).
				Value(&p.organization),
			huh.NewConfirm().
				Title("Private Repository?").
				Description("Should the repository be private?").
				Affirmative("Yes").
				Negative("No").
				Value(&p.isPrivate),
		),
	)

	if err := ui.RunForm(form, p.cancel); err != nil {
		if errors.Is(err, ui.ErrFormCancelled) {
			p.cancelled = true
			logging.GetLogger().Info("Operation cancelled by user during configuration, aborting preparation.")
			return err
		}
		logging.GetLogger().Error("Failed to collect configuration information")
		return err
	}

	logging.GetLogger().Debug("Cloning the next-enterprise boilerplate repository...")

	if checkCancelled() {
		return fmt.Errorf("operation cancelled before cloning")
	}

	p.tempDir, err = os.MkdirTemp("", "enterprise-boilerplate-*")
	if err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to create temporary directory: %s", err))
		return err
	}

	cloneOpts := github.CloneOptions{
		Repository:  "blazity/next-enterprise-terraform",
		Destination: p.tempDir,
		Depth:       1,
	}

	if err := github.CloneRepository(cloneOpts); err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to clone terraform repository: %s", err))
		cleanup(p)
		return err
	}

	branchName := "enterprise-aws-setup"

	logging.GetLogger().Debug("Creating branch in the current repository...")

	branchOpts := github.BranchOptions{
		Path:       ".",
		BranchName: branchName,
		BaseBranch: "main",
		SkipPull:   true,
	}

	actualBranchName, err := github.CreateBranch(branchOpts)
	if err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to create or checkout branch: %s", err))
		cleanup(p)
		return err
	}
	p.activeBranch = actualBranchName
	logging.GetLogger().Info("Prepared branch", "name", actualBranchName)

	logging.GetLogger().Debug("Copying terraform files", "source", filepath.Join(p.tempDir, "terraform"), "dest", filepath.Join(".", "terraform"))

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	targetGitHubActionsDir := filepath.Join(cwd, ".github/")

	if err := filesystem.SafelyDeleteDir(targetGitHubActionsDir); err != nil {
		logging.GetLogger().Error("Failed to delete CI/CD (GitHub Actions) files", "error", err)
		cleanup(p)
		return err
	}

	if err := filesystem.CopyDir(filepath.Join(p.tempDir, ".github/"), targetGitHubActionsDir); err != nil {
		logging.GetLogger().Error("Failed to copy CI/CD (GitHub Actions) files", "error", err)
		cleanup(p)
		return err
	}

	if err := github.CommitChanges(".", "chore(ci): configure github actions for aws", []string{".github"}); err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to commit changes: %s", err))
		cleanup(p)
		return err
	}

	logging.GetLogger().Info("Overwritten the CI/CD files (GitHub Actions) in the local git repository")

	targetTerraformDir := filepath.Join(cwd, "terraform/")

	if err := filesystem.CopyDir(
		filepath.Join(p.tempDir, "terraform/"),
		targetTerraformDir,
	); err != nil {
		logging.GetLogger().Error("Failed to copy terraform files", "error", err)
		cleanup(p)
		return err
	}

	logging.GetLogger().Info("Copied terraform files to the local git repository")

	if err := github.CommitChanges(".", "chore(aws): add terraform files", []string{"terraform"}); err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to commit changes: %s", err))
		cleanup(p)
		return err
	}

	hclCodemodCfg := codemod.NewDefaultHclCodemodConfig()
	hclCodemodCfg.SourceDir = targetTerraformDir
	hclCodemodCfg.Region = p.region

	if err := codemod.RunHclCodemod(hclCodemodCfg); err != nil {
		logging.GetLogger().Error("Failed to apply HCL codemod", "error", err)
		cleanup(p)
		return fmt.Errorf("failed to apply HCL codemod: %w", err)
	}

	if err := github.CommitChanges(".", "chore(aws): modify hcl to reflect user input", []string{"terraform"}); err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to commit changes: %s", err))
		cleanup(p)
		return err
	}

	logging.GetLogger().Info("Modified HCL according to the user input")

	jsCodemodCfg := codemod.NewDefaultJsCodemodConfig()
	jsCodemodCfg.InputPath = "next.config.ts"
	jsCodemodCfg.JsCodemodName = "next-config"

	if err := codemod.RunJsCodemod(jsCodemodCfg); err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to apply next-config codemod: %v", err))
		cleanup(p)
		return fmt.Errorf("preparation succeeded, but failed to apply next-config codemod: %w", err)
	}

	logging.GetLogger().Info("Applied next.config.ts codemod in the local git repository")

	if err := github.CommitChanges(".", "chore(aws): add next.config.ts codemod", []string{"next.config.ts"}); err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to commit changes: %s", err))
		cleanup(p)
		return err
	}

	resourceManager, err := resources.NewResourceManager(p.tempDir)
	if err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to create resource manager: %s", err))
		cleanup(p)
		return err
	}

	destinationPaths, err := resourceManager.CopyAllMappings()
	if err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to copy mappings: %s", err))
		cleanup(p)
		return err
	}

	logging.GetLogger().Info("Copied remaining resources to the local git repository")

	if err := github.CommitChanges(".", "chore(aws): add all remaining resources", destinationPaths); err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to commit changes: %s", err))
		cleanup(p)
		return err
	}

	if err := filesystem.MoveToSubDir("frontend", []string{".github", "terraform", "README.md", "LICENSE", ".gitignore", ".git"}); err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to pack old repository files to the frontend/ subdirectory: %s", err))
		cleanup(p)
		return err
	}

	if err := github.CommitChanges(".", "chore(aws): move old repository to frontend/ sub dir", []string{}); err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to commit changes: %s", err))
		cleanup(p)
		return err
	}

	logging.GetLogger().Info("Moved Next.js application source to subdirectory", "path", "frontend/")

	logging.GetLogger().Info("Done all local git commits")

	repoNameForCreation := ""
	repoFullName :=
		fmt.Sprintf("%s/%s", p.organization, p.repositoryName)
	if p.organization == github.CheckAuthStatus().Username {
		repoNameForCreation = p.repositoryName
	} else {
		repoNameForCreation = repoFullName
	}

	createArgs := []string{"repo", "create", repoNameForCreation}

	if p.isPrivate {
		createArgs = append(createArgs, "--private")
	} else {
		createArgs = append(createArgs, "--public")
	}

	logging.GetLogger().Debug("Creating repository", "name", repoNameForCreation, "type", map[bool]string{true: "private", false: "public"}[p.isPrivate])

	stdout, stderr, err := gh.Exec(createArgs...)
	if err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to create repository: %s", err))
		logging.GetLogger().Error(stderr.String())
		cleanup(p)
		return err
	}

	logging.GetLogger().Info("Created remote repository on GitHub", "url", strings.TrimSpace(stdout.String()))

	remoteName := "origin"
	remoteURL := fmt.Sprintf("https://github.com/%s.git", repoFullName)
	if err := github.SetRemote(".", remoteName, remoteURL); err != nil {
		cleanup(p)
		return err
	}

	logging.GetLogger().Info("Added git remote info to the local repository", "name", remoteName)

	awsForm := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("AWS Access Key ID").
				Description("Your AWS access key").
				Placeholder("AKIAIOSFODNN7EXAMPLE").
				Value(&p.accessKeyID),
			huh.NewInput().
				Title("AWS Secret Access Key").
				Description("Your AWS secret key").
				EchoMode(huh.EchoModePassword).
				Value(&p.secretAccessKey),
		),
	)

	if err := ui.RunForm(awsForm, p.cancel); err != nil {
		if errors.Is(err, ui.ErrFormCancelled) {
			p.cancelled = true
			cleanup(p)
			return err
		}
		logging.GetLogger().Error("Failed to collect AWS credentials")
		cleanup(p)
		return err
	}

	setAccessKeyArgs := []string{"secret", "set", "AWS_ACCESS_KEY_ID", "--body", p.accessKeyID, "--repo", repoFullName}
	_, stderr, err = gh.Exec(setAccessKeyArgs...)
	if err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to set AWS_ACCESS_KEY_ID: %s", err))
		logging.GetLogger().Error(stderr.String())
		cleanup(p)
		return err
	}

	setSecretKeyArgs := []string{"secret", "set", "AWS_SECRET_ACCESS_KEY", "--body", p.secretAccessKey, "--repo", repoFullName}
	_, stderr, err = gh.Exec(setSecretKeyArgs...)
	if err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to set AWS_SECRET_ACCESS_KEY: %s", err))
		logging.GetLogger().Error(stderr.String())
		cleanup(p)
		return err
	}

	setRegionArgs := []string{"secret", "set", "AWS_REGION", "--body", p.region, "--repo", repoFullName}
	_, stderr, err = gh.Exec(setRegionArgs...)
	if err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to set AWS_REGION: %s", err))
		logging.GetLogger().Error(stderr.String())
		cleanup(p)
		return err
	}

	logging.GetLogger().Info(fmt.Sprintf("Set %s secrets as GitHub Secrets", ui.LegibleProviderName("aws")), "secrets", []string{"AWS_REGION", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"})

	setRedisUrlArgs := []string{"variable", "set", "REDIS_URL", "--body", "redis://next-enterprise-terraform-dev-redis-cluster.rwzcut.0001.euw2.cache.amazonaws.com:6379", "--repo", repoFullName}

	_, stderr, err = gh.Exec(setRedisUrlArgs...)
	if err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to set REDIS_URL: %s", err))
		logging.GetLogger().Error(stderr.String())
		cleanup(p)
		return err
	}

	setS3StorybookBucketName := []string{"variable", "set", "S3_STORYBOOK_BUCKET_NAME", "--body", "next-enterprise-terraform-storybook", "--repo", repoFullName}

	_, stderr, err = gh.Exec(setS3StorybookBucketName...)
	if err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to set S3_STORYBOOK_BUCKET_NAME: %s", err))
		logging.GetLogger().Error(stderr.String())
		cleanup(p)
		return err
	}

	logging.GetLogger().Info("Set variables as GitHub Env Vars", "variables", []string{"S3_STORYBOOK_BUCKET_NAME", "REDIS_URL"})

	enableActionsArgs := []string{
		"api",
		"-X", "PUT",
		fmt.Sprintf("repos/%s/actions/permissions", repoFullName),
		"-H", "Accept: application/vnd.github+json",
		"-H", "X-GitHub-Api-Version: 2022-11-28",
		"-F", "enabled=true",
		"-F", "allowed_actions=all",
	}

	_, stderr, err = gh.Exec(enableActionsArgs...)
	if err != nil {
		logging.GetLogger().Error("Failed to enable GitHub Actions", "error", err)
		cleanup(p)
		return err
	}

	logging.GetLogger().Info("Enabled GitHub Actions in the newly created remote repository")

	logging.GetLogger().Info("Deployment prepared in repository", "name", repoFullName, "branch", actualBranchName)

	// Push timestamp branch to remote main
	logger := logging.GetLogger()
	logger.Debug("Pushing local branch to remote main", "localBranch", p.activeBranch, "remote", remoteName, "remoteBranch", "main")
	if out, err := exec.Command("git", "-C", ".", "push", "-u", remoteName, fmt.Sprintf("%s:main", p.activeBranch)).CombinedOutput(); err != nil {
		logger.Error("Failed to push local branch to remote main", "error", err)
		logger.Debug(string(out))
		cleanup(p)
		return err
	}
	logger.Info("Pushed local branch to remote main", "remote", remoteName, "branch", "main")

	// Artificial Merge: delete local timestamp branch
	logger.Info("Deleting local timestamp branch", "branch", p.activeBranch)
	if out, err := exec.Command("git", "-C", ".", "branch", "-D", p.activeBranch).CombinedOutput(); err != nil {
		logger.Warning("Failed to delete local timestamp branch", "branch", p.activeBranch, "error", err)
		logger.Debug(string(out))
	}

	// Delete any existing main branch locally
	logger.Debug("Deleting pre-existing local main branch", "branch", "main")
	if out, err := exec.Command("git", "-C", ".", "branch", "-D", "main").CombinedOutput(); err != nil {
		logger.Debug("Could not delete local main branch (may not exist)", "error", err)
		logger.Debug(string(out))
	}

	// Fetch and check out remote main
	logger.Info("Fetching remote main branch", "remote", remoteName)
	if out, err := exec.Command("git", "-C", ".", "fetch", remoteName, "main").CombinedOutput(); err != nil {
		logger.Error("Failed to fetch remote main", "error", err)
		logger.Debug(string(out))
		cleanup(p)
		return err
	}

	logger.Info("Checking out remote main as local main", "remoteBranch", remoteName+"/main")
	if out, err := exec.Command("git", "-C", ".", "checkout", "--track", remoteName+"/main").CombinedOutput(); err != nil {
		logger.Error("Failed to checkout remote main", "error", err)
		logger.Debug(string(out))
		cleanup(p)
		return err
	}

	// Clear activeBranch so cleanup won't try branch operations again
	p.activeBranch = ""

	cleanup(p)

	return nil
}

func (p *AwsProvider) Deploy() error {
	return p.DeployWithContext(context.Background())
}

func (p *AwsProvider) DeployWithContext(ctx context.Context) error {
	logging.GetLogger().Info("Deploying to AWS...")

	cancelled := ctx.Done()

	checkCancelled := func() bool {
		select {
		case <-cancelled:
			logging.GetLogger().Info("Operation cancelled via context during deployment")
			return true
		default:
			return false
		}
	}

	if checkCancelled() {
		return fmt.Errorf("operation cancelled by user before deployment started")
	}

	logging.GetLogger().Info("AWS deployment completed successfully")

	return nil
}

func cleanup(p *AwsProvider) {
	logger := logging.GetLogger()
	// If a timestamp branch was created, switch back to main and delete it
	if p.activeBranch != "" {
		logger.Debug("Switching back to main branch", "from", p.activeBranch)
		if out, err := exec.Command("git", "-C", ".", "checkout", "main").CombinedOutput(); err != nil {
			logger.Warning("Failed to checkout main during cleanup", "error", err)
			logger.Debug(string(out))
		}
		logger.Debug("Deleting local timestamp branch", "branch", p.activeBranch)
		if out, err := exec.Command("git", "-C", ".", "branch", "-D", p.activeBranch).CombinedOutput(); err != nil {
			logger.Warning("Failed to delete local timestamp branch during cleanup", "branch", p.activeBranch, "error", err)
			logger.Debug(string(out))
		}
	}
	// Clean up temp dir if exists
	if p.tempDir != "" {
		logger.Debug("Cleaning up temporary directory", "path", p.tempDir)
		if err := os.RemoveAll(p.tempDir); err != nil {
			logger.Warning("Failed to clean up temporary directory", "path", p.tempDir, "error", err)
		}
	}
}
