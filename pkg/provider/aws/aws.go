package aws

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	if len(organizations) > 8 {
		organizationsLength = 8
	} else {
		organizationsLength = len(organizations)
	}

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
	logging.GetLogger().Info(fmt.Sprintf("Prepared branch: %s", actualBranchName))

	logging.GetLogger().Info("Copying files from boilerplate...")

	if err := copyFile(
		filepath.Join(p.tempDir, "README.md"),
		filepath.Join(".", "README.md"),
	); err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to copy README.md: %s", err))
		cleanup(p)
		return err
	}

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

	logging.GetLogger().Info(fmt.Sprintf("Creating %s repository: %s on GitHub...",
		map[bool]string{true: "private", false: "public"}[p.isPrivate],
		repoNameForCreation))

	stdout, stderr, err := gh.Exec(createArgs...)
	if err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to create repository: %s", err))
		logging.GetLogger().Error(stderr.String())
		cleanup(p)
		return err
	}

	logging.GetLogger().Info(fmt.Sprintf("Repository created: %s", strings.TrimSpace(stdout.String())))

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

	logging.GetLogger().Info("Setting AWS credentials as GitHub secrets...")

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

	logging.GetLogger().Info("AWS credentials set as GitHub secrets")

	if err := github.CommitChanges(".", "Add files from Enterprise boilerplate", []string{"README.md"}); err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to commit changes: %s", err))
		cleanup(p)
		return err
	}

	logging.GetLogger().Info("Commited changes")

	checkRemoteCmd := exec.Command("git", "-C", ".", "remote")
	remoteOutput, _ := checkRemoteCmd.CombinedOutput()

	remoteName := "enterprise-aws"

	if strings.Contains(string(remoteOutput), remoteName) {
		removeRemoteCmd := exec.Command("git", "-C", ".", "remote", "remove", remoteName)
		if output, err := removeRemoteCmd.CombinedOutput(); err != nil {
			logging.GetLogger().Warning(fmt.Sprintf("Remote already exists but couldn't be removed: %s", string(output)))
			remoteName = "enterprise-aws-new"
		}
	}

	logging.GetLogger().Info(fmt.Sprintf("%s deployment prepared in repository: %s on branch %s", ui.LegibleProviderName(p.GetName()), repoFullName, actualBranchName))

	logging.GetLogger().Info("Applying next.config.ts codemod...")
	codemodCfg := codemod.NewDefaultConfig()
	codemodCfg.InputPath = "next.config.ts"
	codemodCfg.CodemodName = "next-config"

	if err := codemod.RunCodemod(codemodCfg); err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to apply next-config codemod: %v", err))
		cleanup(p)
		return fmt.Errorf("preparation succeeded, but failed to apply next-config codemod: %w", err)
	}
	logging.GetLogger().Info("Successfully applied next.config.ts codemod.")

	resourceManager, err := resources.NewResourceManager(p.tempDir)
	if err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to create resource manager: %s", err))
		cleanup(p)
		return err
	}

	if err := resourceManager.CopyAllMappings(); err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to copy mappings: %s", err))
		cleanup(p)
		return err
	}

	addRemoteCmd := exec.Command("git", "-C", ".", "remote", "add", remoteName, fmt.Sprintf("https://github.com/%s.git", repoFullName))

	if output, err := addRemoteCmd.CombinedOutput(); err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to add remote: %s", err))
		logging.GetLogger().Error(string(output))
		cleanup(p)
		return err
	}

	if err := github.PushBranch(".", actualBranchName, remoteName); err != nil {
		logging.GetLogger().Error(fmt.Sprintf("Failed to push changes: %s", err))
		cleanup(p)
		return err
	}

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
	if p.tempDir != "" {
		logging.GetLogger().Debug(fmt.Sprintf("Cleaning up temporary directory: %s", p.tempDir))
		if err := os.RemoveAll(p.tempDir); err != nil {
			logging.GetLogger().Warning(fmt.Sprintf("Failed to clean up temporary directory: %s", err))
		}
	}
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return nil
}
