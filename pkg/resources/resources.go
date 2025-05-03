package resources

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Mapping represents a single file mapping from the YAML config
// legible-name: Human-readable identifier for the mapping
// source: Absolute path to the source file (resolved during initialization)
// destination: Target location relative to the rootDir where the file should be copied
type Mapping struct {
	LegibleName string `yaml:"legible-name"`
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
}

// Config represents the structure of the _map.yml file
// mappings: List of Mapping entries
type Config struct {
	Mappings []Mapping `yaml:"mappings"`
}

// ResourceManager holds the loaded configuration and the target root directory.
type ResourceManager struct {
	config  *Config
	rootDir string // Absolute path to the target root directory
}

// findUniqueMapYML searches for a unique _map.yml file in the current working directory and subdirectories.
// Returns the absolute path to the file and its containing directory.
func findUniqueMapYML() (string, string, error) {
	var found []string
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}

	walkErr := filepath.WalkDir(cwd, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Prevent descending into directories that can't be read
			if errors.Is(err, fs.ErrPermission) {
				return nil
			}
			return err // Propagate other errors
		}
		if !d.IsDir() && d.Name() == "_map.yml" {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("failed to get absolute path for %s: %w", path, err)
			}
			found = append(found, absPath)
		}
		return nil
	})

	if walkErr != nil {
		return "", "", fmt.Errorf("error walking directory %s: %w", cwd, walkErr)
	}
	if len(found) == 0 {
		return "", "", errors.New("no _map.yml file found in the current directory or subdirectories")
	}
	if len(found) > 1 {
		// TODO: Consider listing the found files for better debugging
		return "", "", fmt.Errorf("multiple _map.yml files found: %v; must be unique", found)
	}
	mapPath := found[0]
	configDir := filepath.Dir(mapPath)
	return mapPath, configDir, nil
}

// NewResourceManager creates a new ResourceManager instance.
// It finds the unique _map.yml file, loads the configuration,
// resolves source paths to be absolute, and stores the configuration
// along with the absolute path to the provided rootDir.
func NewResourceManager(rootDir string) (*ResourceManager, error) {
	mapPath, configDir, err := findUniqueMapYML()
	if err != nil {
		return nil, fmt.Errorf("failed to find _map.yml: %w", err)
	}

	f, err := os.Open(mapPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file '%s': %w", mapPath, err)
	}
	defer f.Close()

	var cfg Config
	decoder := yaml.NewDecoder(f)
	// Use DisallowUnknownFields for stricter parsing if needed
	// decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode yaml from '%s': %w", mapPath, err)
	}

	// Resolve source paths to be absolute relative to the config file's directory
	for i := range cfg.Mappings {
		if !filepath.IsAbs(cfg.Mappings[i].Source) {
			cfg.Mappings[i].Source = filepath.Join(configDir, cfg.Mappings[i].Source)
		}
		// Optional: Clean the path
		cfg.Mappings[i].Source = filepath.Clean(cfg.Mappings[i].Source)
	}

	absRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for root directory '%s': %w", rootDir, err)
	}

	rm := &ResourceManager{
		config:  &cfg,
		rootDir: absRootDir,
	}

	return rm, nil
}

// copyMapping copies a single mapping's source file to its destination within the ResourceManager's context.
// This is an internal helper method.
func (rm *ResourceManager) copyMapping(mapping Mapping) error {
	srcPath := mapping.Source // Source path is already absolute
	dstPath := filepath.Join(rm.rootDir, mapping.Destination, filepath.Base(srcPath))
	dstDir := filepath.Dir(dstPath)

	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return fmt.Errorf("source file '%s' for mapping '%s' does not exist", srcPath, mapping.LegibleName)
	} else if err != nil {
		return fmt.Errorf("failed to stat source file '%s': %w", srcPath, err)
	}

	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory '%s': %w", dstDir, err)
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file '%s': %w", srcPath, err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		// Attempt to clean up created directory if file creation fails? Maybe too complex.
		return fmt.Errorf("failed to create destination file '%s': %w", dstPath, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		// Attempt to remove partially created dstFile on copy error
		dstFile.Close()    // Close first
		os.Remove(dstPath) // Ignore error on remove
		return fmt.Errorf("failed to copy file from '%s' to '%s': %w", srcPath, dstPath, err)
	}

	return nil
}

// CopyAllMappings copies all files specified in the loaded configuration
// to their destinations relative to the ResourceManager's root directory.
func (rm *ResourceManager) CopyAllMappings() error {
	if rm.config == nil {
		return errors.New("ResourceManager not properly initialized or configuration is missing")
	}
	for _, mapping := range rm.config.Mappings {
		if err := rm.copyMapping(mapping); err != nil {
			// Return the first error encountered
			return fmt.Errorf("failed processing mapping '%s': %w", mapping.LegibleName, err)
		}
		// Optional: Add logging here if needed
	}
	return nil
}
