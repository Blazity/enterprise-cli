package codemod

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// HclCodemodConfig holds configuration for the HCL codemod
// It specifies the source directory containing Terraform files and the new region to set.
type HclCodemodConfig struct {
	SourceDir string
	Region    string
}

// NewDefaultHclCodemodConfig returns a default HclCodemodConfig
func NewDefaultHclCodemodConfig() *HclCodemodConfig {
	return &HclCodemodConfig{}
}

// RunHclCodemodFromFlags parses flags and runs the HCL codemod
func RunHclCodemodFromFlags() error {
	cfg, err := parseHclFlags()
	if err != nil {
		return err
	}
	return RunHclCodemod(cfg)
}

// RunHclCodemod validates the config and modifies backend.tf accordingly
func RunHclCodemod(cfg *HclCodemodConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := cfg.ModifyBackend(); err != nil {
		return fmt.Errorf("failed to modify backend.tf: %w", err)
	}
	return nil
}

// parseHclFlags parses command-line flags for the HCL codemod
func parseHclFlags() (*HclCodemodConfig, error) {
	cfg := &HclCodemodConfig{}
	fs := flag.NewFlagSet("hcl-codemod", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.SourceDir, "source", "", "Path to Terraform source directory (required)")
	fs.StringVar(&cfg.Region, "region", "", "AWS region to set in backend.tf (required)")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return nil, fmt.Errorf("error parsing flags: %w", err)
	}
	if cfg.SourceDir == "" {
		return nil, fmt.Errorf("missing required flag: --source")
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("missing required flag: --region")
	}
	return cfg, nil
}

// Validate ensures the Terraform backend file exists at the expected path
func (cfg *HclCodemodConfig) Validate() error {
	backendPath := filepath.Join(cfg.SourceDir, "dev", "backend.tf")
	if _, err := os.Stat(backendPath); os.IsNotExist(err) {
		return fmt.Errorf("backend.tf not found at %s", backendPath)
	}
	return nil
}

// ModifyBackend reads backend.tf, updates the region attribute, and writes the file
func (cfg *HclCodemodConfig) ModifyBackend() error {
	backendPath := filepath.Join(cfg.SourceDir, "dev", "backend.tf")
	src, err := ioutil.ReadFile(backendPath)
	if err != nil {
		return fmt.Errorf("error reading backend.tf: %w", err)
	}

	file, diags := hclwrite.ParseConfig(src, backendPath, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return fmt.Errorf("error parsing backend.tf: %s", diags.Error())
	}

	backendBlock, err := findBackendBlock(file)
	if err != nil {
		return err
	}

	updateRegionAttribute(backendBlock, cfg.Region)

	if err := writeBackendFile(backendPath, file); err != nil {
		return err
	}
	return nil
}

// findBackendBlock locates the terraform backend "s3" block in the HCL file
func findBackendBlock(f *hclwrite.File) (*hclwrite.Block, error) {
	rootBody := f.Body()
	terraformBlock := rootBody.FirstMatchingBlock("terraform", nil)
	if terraformBlock == nil {
		return nil, fmt.Errorf("terraform block not found in backend.tf")
	}
	backendBlock := terraformBlock.Body().FirstMatchingBlock("backend", []string{"s3"})
	if backendBlock == nil {
		return nil, fmt.Errorf("backend \"s3\" block not found in backend.tf")
	}
	return backendBlock, nil
}

// updateRegionAttribute sets the region attribute on the backend block
func updateRegionAttribute(block *hclwrite.Block, region string) {
	block.Body().SetAttributeValue("region", cty.StringVal(region))
}

// writeBackendFile writes the modified HCL file back to disk
func writeBackendFile(path string, f *hclwrite.File) error {
	if err := ioutil.WriteFile(path, f.Bytes(), 0644); err != nil {
		return fmt.Errorf("error writing updated backend.tf: %w", err)
	}
	return nil
}
