# Enterprise CLI

A comprehensive CLI tool for preparing and deploying enterprise infrastructure across various cloud providers.

## Features

- Provider-based architecture for easy extensibility
- Interactive forms with beautiful UI
- Consistent styled logging with verbose mode
- GitHub integration for repository management
- Built using modern Go libraries

## Installation

```bash
# Clone the repository
git clone https://github.com/blazity/enterprise-cli.git
cd enterprise-cli

# Build the CLI
go build -o enterprise

# Optionally, move to a directory in your PATH
mv enterprise /usr/local/bin/
```

## Usage

### AWS Deployment

```bash
# Prepare AWS deployment - this will:
# 1. Clone a template repository
# 2. Create a new GitHub repository
# 3. Set up AWS credentials as repository secrets
# 4. Push initial files to the new repository
enterprise prepare aws

# Deploy your AWS infrastructure
enterprise deploy aws

# Enable verbose logging
enterprise --verbose prepare aws
```

### Requirements

- Git must be installed
- GitHub CLI (`gh`) must be installed and authenticated
- For AWS provider: AWS credentials (Access Key ID and Secret Access Key)

## Technologies

- [Cobra](https://github.com/spf13/cobra) - Command structure and CLI framework
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - Terminal UI framework
- [Huh](https://github.com/charmbracelet/huh) - Interactive form components
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [GitHub CLI](https://cli.github.com/) - GitHub integration
- [Terraform Exec](https://github.com/hashicorp/terraform-exec) - Terraform integration

## Adding New Providers

To add a new provider:

1. Create a new package in `pkg/provider/<provider-name>`
2. Implement the `Provider` interface
3. Register the provider in the init function
4. Import the provider package in `cmd/enterprise/enterprise.go`

## License

MIT