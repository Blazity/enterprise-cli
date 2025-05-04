package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SafelyDeleteDir recursively deletes a directory ensuring it's within the current working directory.
// It aborts deletion if the target directory is outside or at the same level as the current working directory.
func SafelyDeleteDir(dirPath string) error {
	// Get absolute paths
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	
	// Clean paths to ensure consistent format
	absPath = filepath.Clean(absPath)
	cwd = filepath.Clean(cwd)
	
	// Check if trying to delete the current working directory
	if absPath == cwd {
		return fmt.Errorf("cannot delete the current working directory")
	}
	
	// Check if the path is within the current working directory
	relPath, err := filepath.Rel(cwd, absPath)
	if err != nil {
		return fmt.Errorf("failed to determine relative path: %w", err)
	}
	
	// If relative path starts with "..", it's outside the cwd
	if strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("cannot delete directory outside the current working directory")
	}
	
	// Execute the deletion
	return os.RemoveAll(absPath)
}
