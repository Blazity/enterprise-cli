// move.go
package filesystem

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// MoveToSubDir moves all files and directories from the current working directory
// to a specified subdirectory, optionally ignoring specified paths.
// It creates the subdirectory if it doesn't exist.
func MoveToSubDir(subDirName string, ignorePaths []string) error {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Create full path for the target subdirectory
	targetDir := filepath.Join(cwd, subDirName)

	// Convert paths to absolute and clean them
	targetDirAbs, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for target directory: %w", err)
	}
	targetDirAbs = filepath.Clean(targetDirAbs)
	cwdAbs := filepath.Clean(cwd)

	// Ensure target is a subdirectory of the current working directory
	relPath, err := filepath.Rel(cwdAbs, targetDirAbs)
	if err != nil {
		return fmt.Errorf("failed to determine relative path: %w", err)
	}
	if strings.HasPrefix(relPath, "..") || relPath == "." {
		return fmt.Errorf("target directory must be a subdirectory of the current working directory")
	}

	// Build a map of paths to ignore for efficient lookup
	ignorePathsMap := make(map[string]bool, len(ignorePaths)+1)

	// Always ignore the target directory
	ignorePathsMap[targetDirAbs] = true

	// Add user-specified paths to ignore
	for _, path := range ignorePaths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path for %s: %w", path, err)
		}
		ignorePathsMap[filepath.Clean(absPath)] = true
	}

	// Create target directory if it doesn't exist
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", subDirName, err)
	}

	// Read all entries in the current working directory
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return fmt.Errorf("failed to read current directory: %w", err)
	}

	// Track entries that have been moved for potential rollback
	movedEntries := make([]string, 0, len(entries))

	// Move each entry that isn't in the ignore list
	for _, entry := range entries {
		entryName := entry.Name()
		srcPath := filepath.Join(cwd, entryName)

		// Convert to absolute path for comparison with ignore list
		srcPathAbs, err := filepath.Abs(srcPath)
		if err != nil {
			rollbackMoves(cwd, targetDir, movedEntries)
			return fmt.Errorf("failed to resolve absolute path for %s: %w", entryName, err)
		}
		srcPathAbs = filepath.Clean(srcPathAbs)

		// Skip if this path should be ignored
		if ignorePathsMap[srcPathAbs] {
			continue
		}

		dstPath := filepath.Join(targetDir, entryName)

		// Try to move the entry
		if err := moveEntry(srcPath, dstPath, entry.IsDir()); err != nil {
			// If there's an error, try to roll back already moved entries
			rollbackMoves(cwd, targetDir, movedEntries)
			return fmt.Errorf("failed to move %s: %w", entryName, err)
		}

		movedEntries = append(movedEntries, entryName)
	}

	return nil
}

// moveEntry moves a file or directory using the appropriate method
func moveEntry(src, dst string, isDir bool) error {
	// First try to use rename, which is most efficient
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// If rename fails, fall back to copy and delete
	if isDir {
		return copyDirAndDelete(src, dst)
	}
	return copyFileAndDelete(src, dst)
}

// copyFileAndDelete copies a file and then deletes the original
func copyFileAndDelete(src, dst string) error {
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Get source file info for permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get source file info: %w", err)
	}

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy the content
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		os.Remove(dst) // Clean up the destination file
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Ensure file is written to disk
	if err := dstFile.Sync(); err != nil {
		os.Remove(dst) // Clean up the destination file
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	// Close files before setting permissions
	dstFile.Close()
	srcFile.Close()

	// Set permissions
	if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
		os.Remove(dst) // Clean up the destination file
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Delete the source file
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("failed to remove source file: %w", err)
	}

	return nil
}

// copyDirAndDelete copies a directory recursively and then deletes the original
func copyDirAndDelete(src, dst string) error {
	// Get source directory info for permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to get source directory info: %w", err)
	}

	// Create destination directory with same permissions
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Read directory contents
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDirAndDelete(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFileAndDelete(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	// After all contents are copied, remove the source directory
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("failed to remove source directory: %w", err)
	}

	return nil
}

// rollbackMoves attempts to move entries back to their original location
// This is a best-effort function that does not return errors
func rollbackMoves(cwd, targetDir string, movedEntries []string) {
	for _, entryName := range movedEntries {
		srcPath := filepath.Join(targetDir, entryName)
		dstPath := filepath.Join(cwd, entryName)

		// Try to move using rename first
		_ = os.Rename(srcPath, dstPath) // Ignore errors, best effort
	}
}
