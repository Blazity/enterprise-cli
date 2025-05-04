package resources

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/blazity/enterprise-cli/pkg/logging"
	"gopkg.in/yaml.v3"
)

type Mapping struct {
	LegibleName string `yaml:"legible-name"`
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
}

type Config struct {
	Mappings []Mapping `yaml:"mappings"`
}

type ResourceManager struct {
	config  *Config
	rootDir string
}

func findUniqueMapYML(rootDir string) (string, string, error) {
	var found []string

	walkErr := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {

			if errors.Is(err, fs.ErrPermission) {
				return nil
			}
			return err
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
		return "", "", fmt.Errorf("error walking directory %s: %w", rootDir, walkErr)
	}
	if len(found) == 0 {
		return "", "", errors.New("no _map.yml file found in the current directory or subdirectories")
	}
	if len(found) > 1 {

		return "", "", fmt.Errorf("multiple _map.yml files found: %v; must be unique", found)
	}
	mapPath := found[0]
	configDir := filepath.Dir(mapPath)
	return mapPath, configDir, nil
}

func NewResourceManager(rootDir string) (*ResourceManager, error) {
	logger := logging.GetLogger()
	logger.Info("Finding _map.yml file... in", "directory", rootDir)
	mapPath, configDir, err := findUniqueMapYML(rootDir)
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

	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode yaml from '%s': %w", mapPath, err)
	}

	for i := range cfg.Mappings {
		if !filepath.IsAbs(cfg.Mappings[i].Source) {
			cfg.Mappings[i].Source = filepath.Join(configDir, cfg.Mappings[i].Source)
		}

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

// normalizeDestination processes destination paths by removing special variables
// and handling empty paths. It returns the absolute path to the destination directory.
func (rm *ResourceManager) normalizeDestination(destination string) (string, error) {
	dest := destination
	dest = strings.Replace(dest, "${next-enterprise}/", "", 1)
	dest = strings.Replace(dest, "${next-enterprise}", "", 1)

	logging.GetLogger().Debug("Normalized destination path", "path", dest)

	if dest == "" {
		dest = "."
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	return filepath.Join(cwd, dest), nil
}

func (rm *ResourceManager) copyMapping(mapping Mapping) error {

	src := mapping.Source
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("source file '%s' for mapping '%s' does not exist", src, mapping.LegibleName)
		}
		return fmt.Errorf("failed to stat source file '%s': %w", src, err)
	}
	if info.IsDir() {
		return fmt.Errorf("source path '%s' for mapping '%s' is a directory", src, mapping.LegibleName)
	}

	dest := mapping.Destination

	destDir, err := rm.normalizeDestination(dest)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory '%s': %w", destDir, err)
	}

	dst := filepath.Join(destDir, filepath.Base(src))
	if err := copyFile(src, dst); err != nil {
		return err
	}

	logging.GetLogger().Debug("Copied mapping", "from", src, "to", dst)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file '%s': %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file '%s': %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		os.Remove(dst)
		return fmt.Errorf("failed to copy file from '%s' to '%s': %w", src, dst, err)
	}

	return nil
}

func (rm *ResourceManager) CopyAllMappings() ([]string, error) {
	if rm.config == nil {
		return nil, errors.New("ResourceManager not properly initialized or configuration is missing")
	}

	destinationPaths := []string{}

	for _, mapping := range rm.config.Mappings {
		if err := rm.copyMapping(mapping); err != nil {
			return nil, fmt.Errorf("failed processing mapping '%s': %w", mapping.LegibleName, err)
		}

		destDir, err := rm.normalizeDestination(mapping.Destination)
		if err != nil {
			return nil, err
		}

		destPath := filepath.Join(destDir, filepath.Base(mapping.Source))
		destinationPaths = append(destinationPaths, destPath)
	}

	return destinationPaths, nil
}
