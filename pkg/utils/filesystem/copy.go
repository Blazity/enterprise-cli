package filesystem

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/blazity/enterprise-cli/pkg/logging"
)

func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}

	// Create destination directory with parent directories if needed
	err = os.MkdirAll(dst, srcInfo.Mode())
	if err != nil {
		logging.GetLogger().Debug("Could not mkdir", "error", err)
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}

		if entryInfo.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			
			// Force behavior - remove existing symlink first
			_ = os.Remove(dstPath)
			
			err = os.Symlink(linkTarget, dstPath)
			if err != nil {
				return err
			}
		} else if entry.IsDir() {
			err = CopyDir(srcPath, dstPath)
			if err != nil {
				return err
			}
		} else {
			err = CopyFile(srcPath, dstPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// CopyFile copies a file from src to dst, implementing cp -f behavior
func CopyFile(src, dst string) error {
	// Open source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Get source file info
	sourceFileInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	// Force behavior - try to remove the destination first
	_ = os.Remove(dst)

	// Create destination file
	destFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, sourceFileInfo.Mode())
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Copy the contents
	if _, err = io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	// Preserve the file metadata (permissions and modification time)
	if err = os.Chmod(dst, sourceFileInfo.Mode()); err != nil {
		return err
	}
	
	return os.Chtimes(dst, time.Now(), sourceFileInfo.ModTime())
}
