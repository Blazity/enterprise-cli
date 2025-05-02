package codemod

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Config struct {
	InputPath      string
	CodemodName    string
	CodemodDir     string
	Parser         string
	DryRun         bool
	Verbose        bool
	Extensions     string
	CodemodAbsPath string
	TransformPath  string
}

func NewDefaultConfig() *Config {
	return &Config{
		CodemodDir: "codemods",
		Parser:     "tsx",
		Extensions: "js,jsx,ts,tsx",
		DryRun:     false,
		Verbose:    false,
	}
}

func RunFromFlags() error {
	cfg, err := parseFlags()
	if err != nil {
		return err
	}
	return RunCodemod(cfg)
}

func RunCodemod(cfg *Config) error {
	if cfg.CodemodDir == "" {
		cfg.CodemodDir = "codemods"
	}
	if cfg.Parser == "" {
		cfg.Parser = "tsx"
	}
	if cfg.Extensions == "" {
		cfg.Extensions = "js,jsx,ts,tsx"
	}

	if cfg.InputPath == "" {
		return fmt.Errorf("missing InputPath in codemod config")
	}
	if cfg.CodemodName == "" {
		return fmt.Errorf("missing CodemodName in codemod config")
	}

	if err := resolvePathsAndValidate(cfg); err != nil {
		return fmt.Errorf("path validation failed: %w", err)
	}

	cmd, err := prepareCommand(cfg)
	if err != nil {
		return fmt.Errorf("failed to prepare jscodeshift command: %w", err)
	}

	if err := runCommand(cmd, cfg); err != nil {
		return fmt.Errorf("failed to run jscodeshift: %w", err)
	}

	return nil
}

func parseFlags() (*Config, error) {
	cfg := &Config{}

	fs := flag.NewFlagSet("codemod", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.StringVar(&cfg.InputPath, "input", "", "Path to the file or directory to transform (required)")
	fs.StringVar(&cfg.CodemodName, "transform", "", "Name of the transform to use (required)")
	fs.StringVar(&cfg.CodemodDir, "dir", "codemods", "Directory containing transform files")
	fs.StringVar(&cfg.Parser, "parser", "tsx", "Parser to use (babel, babylon, ts, tsx, flow, etc.)")
	fs.BoolVar(&cfg.DryRun, "dry", false, "Dry run (don't modify files)")
	fs.BoolVar(&cfg.Verbose, "verbose", false, "Show more information about the transform process")
	fs.StringVar(&cfg.Extensions, "extensions", "js,jsx,ts,tsx", "Comma-separated list of file extensions to process")

	err := fs.Parse(os.Args[1:])
	if err != nil {
		var usage strings.Builder
		fmt.Fprintf(&usage, "Usage of %s:\n", os.Args[0])
		fs.SetOutput(&usage)
		fs.PrintDefaults()
		fs.SetOutput(io.Discard)
		return nil, fmt.Errorf("error parsing flags: %w\n%s", err, usage.String())
	}

	if cfg.InputPath == "" {
		return nil, fmt.Errorf("missing required flag: --input")
	}
	if cfg.CodemodName == "" {
		return nil, fmt.Errorf("missing required flag: --transform")
	}

	return cfg, nil
}

func resolveCodemodAbsPath(dirFlag string) (string, error) {
	if filepath.IsAbs(dirFlag) {
		if _, err := os.Stat(dirFlag); os.IsNotExist(err) {
			return "", fmt.Errorf("specified absolute codemod directory not found: %s", dirFlag)
		}
		return dirFlag, nil
	}

	cwd, err := os.Getwd()
	if err == nil {
		pathRelCwd := filepath.Join(cwd, dirFlag)
		if _, err := os.Stat(pathRelCwd); err == nil {
			return pathRelCwd, nil
		}
	}

	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("codemod directory '%s' not found relative to CWD, and could not get executable path to check further: %w", dirFlag, err)
	}
	execDir := filepath.Dir(execPath)
	pathRelExec := filepath.Join(execDir, dirFlag)
	if _, err := os.Stat(pathRelExec); err == nil {
		return pathRelExec, nil
	}

	return "", fmt.Errorf("codemod directory '%s' not found relative to current directory or executable directory ('%s')", dirFlag, execDir)
}

func resolvePathsAndValidate(cfg *Config) error {
	inputPathAbs, err := filepath.Abs(cfg.InputPath)
	if err != nil {
		return fmt.Errorf("could not get absolute path for input '%s': %w", cfg.InputPath, err)
	}
	if _, err := os.Stat(inputPathAbs); os.IsNotExist(err) {
		return fmt.Errorf("input path not found: %s (resolved to: %s)", cfg.InputPath, inputPathAbs)
	}

	codemodAbsPath, err := resolveCodemodAbsPath(cfg.CodemodDir)
	if err != nil {
		return err
	}
	cfg.CodemodAbsPath = codemodAbsPath

	transformPath := filepath.Join(cfg.CodemodAbsPath, cfg.CodemodName)
	if !strings.HasSuffix(strings.ToLower(transformPath), ".js") {
		transformPath += ".js"
	}
	cfg.TransformPath = transformPath

	if _, err := os.Stat(cfg.TransformPath); os.IsNotExist(err) {
		availableTransformsStr := listAvailableTransforms(cfg.CodemodAbsPath)
		errMsg := fmt.Sprintf("transform file not found: %s", cfg.TransformPath)
		if availableTransformsStr != "" {
			errMsg += "\nAvailable transforms in directory:\n" + availableTransformsStr
		} else {
			errMsg += fmt.Sprintf("\n(Could not list available transforms or none found in %s)", cfg.CodemodAbsPath)
		}
		return fmt.Errorf(errMsg)
	}

	return nil
}

func listAvailableTransforms(codemodAbsPath string) string {
	files, err := os.ReadDir(codemodAbsPath)
	if err != nil {
		return ""
	}

	var availableNames []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(strings.ToLower(file.Name()), ".js") {
			transformName := strings.TrimSuffix(file.Name(), ".js")
			transformName = strings.TrimSuffix(transformName, ".JS")
			availableNames = append(availableNames, transformName)
		}
	}

	if len(availableNames) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, name := range availableNames {
		builder.WriteString(fmt.Sprintf("  - %s\n", name))
	}
	return builder.String()
}

// prepareCommand constructs the *exec.Cmd for jscodeshift to be executed via npx.
func prepareCommand(cfg *Config) (*exec.Cmd, error) {
	npxPath, err := exec.LookPath("npx")
	if err != nil {
		return nil, fmt.Errorf("command 'npx' not found in PATH. Is Node.js (which includes npx) installed and configured correctly? (%w)", err)
	}

	args := []string{
		"jscodeshift",
		"-t", cfg.TransformPath,
		"--parser", cfg.Parser,
		"--extensions", cfg.Extensions,
	}

	if cfg.DryRun {
		args = append(args, "--dry")
	}
	if cfg.Verbose {
		args = append(args, "--verbose")
	}

	args = append(args, cfg.InputPath)

	cmd := exec.Command(npxPath, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd, nil
}

func runCommand(cmd *exec.Cmd, cfg *Config) error {
	if cfg.Verbose {
		fmt.Printf("Executing codemod command: %s\n", strings.Join(cmd.Args, " "))
	}

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("jscodeshift process execution failed: %w", err)
	}

	return nil
}
