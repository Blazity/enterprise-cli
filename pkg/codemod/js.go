package codemod

import (
	"embed"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed codemods/*.js
var codemodsFS embed.FS

// extractEmbeddedCodemods unpacks embedded JS codemod files into a temporary dir.
func extractEmbeddedCodemods() (string, error) {
	tmpDir, err := os.MkdirTemp("", "cli-codemods-*")
	if err != nil {
		return "", fmt.Errorf("creating temp codemod dir: %w", err)
	}
	entries, err := codemodsFS.ReadDir("codemods")
	if err != nil {
		return "", fmt.Errorf("reading embedded codemods: %w", err)
	}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(strings.ToLower(ent.Name()), ".js") {
			continue
		}
		data, err := codemodsFS.ReadFile("codemods/" + ent.Name())
		if err != nil {
			return "", fmt.Errorf("reading embedded file %q: %w", ent.Name(), err)
		}
		target := filepath.Join(tmpDir, ent.Name())
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return "", fmt.Errorf("writing embedded codemod %q to temp dir: %w", ent.Name(), err)
		}
	}
	return tmpDir, nil
}

type JsCodemodConfig struct {
	InputPath      string
	JsCodemodName  string
	JsCodemodDir   string
	Parser         string
	DryRun         bool
	Verbose        bool
	Extensions     string
	CodemodAbsPath string
	TransformPath  string
}

func NewDefaultJsCodemodConfig() *JsCodemodConfig {
	return &JsCodemodConfig{
		JsCodemodDir: "codemods",
		Parser:       "tsx",
		Extensions:   "js,jsx,ts,tsx",
		DryRun:       false,
		Verbose:      false,
	}
}

func RunJsCodemodFromFlags() error {
	cfg, err := parseFlags()
	if err != nil {
		return err
	}
	return RunJsCodemod(cfg)
}

func RunJsCodemod(cfg *JsCodemodConfig) error {
	// unpack the embedded codemods into a temp directory
	tmpDir, err := extractEmbeddedCodemods()
	if err != nil {
		return fmt.Errorf("could not extract embedded codemods: %w", err)
	}
	cfg.JsCodemodDir = tmpDir

	// fall back only if extraction failed to provide a dir (unlikely)
	if cfg.JsCodemodDir == "" {
		cfg.JsCodemodDir = "codemods"
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
	if cfg.JsCodemodName == "" {
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

func parseFlags() (*JsCodemodConfig, error) {
	cfg := &JsCodemodConfig{}

	fs := flag.NewFlagSet("codemod", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	fs.StringVar(&cfg.InputPath, "input", "", "Path to the file or directory to transform (required)")
	fs.StringVar(&cfg.JsCodemodName, "transform", "", "Name of the transform to use (required)")
	fs.StringVar(&cfg.JsCodemodDir, "dir", "codemods", "Directory containing transform files")
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
	if cfg.JsCodemodName == "" {
		return nil, fmt.Errorf("missing required flag: --transform")
	}

	return cfg, nil
}

func resolveJsCodemodAbsPath(dirFlag string) (string, error) {
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

func resolvePathsAndValidate(cfg *JsCodemodConfig) error {
	inputPathAbs, err := filepath.Abs(cfg.InputPath)
	if err != nil {
		return fmt.Errorf("could not get absolute path for input '%s': %w", cfg.InputPath, err)
	}
	if _, err := os.Stat(inputPathAbs); os.IsNotExist(err) {
		return fmt.Errorf("input path not found: %s (resolved to: %s)", cfg.InputPath, inputPathAbs)
	}

	codemodAbsPath, err := resolveJsCodemodAbsPath(cfg.JsCodemodDir)
	if err != nil {
		return err
	}
	cfg.CodemodAbsPath = codemodAbsPath

	transformPath := filepath.Join(cfg.CodemodAbsPath, cfg.JsCodemodName)
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

func listAvailableTransforms(jsCodemodAbsPath string) string {
	files, err := os.ReadDir(jsCodemodAbsPath)
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

func prepareCommand(cfg *JsCodemodConfig) (*exec.Cmd, error) {
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

	return cmd, nil
}

func runCommand(cmd *exec.Cmd, cfg *JsCodemodConfig) error {
	if cfg.Verbose {
		fmt.Printf("Executing codemod command: %s\n", strings.Join(cmd.Args, " "))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jscodeshift process execution failed: %w\nOutput:\n%s", err, string(output))
	}

	if cfg.Verbose && len(output) > 0 {
		fmt.Printf("Codemod Output:\n%s\n", string(output))
	}

	return nil
}
