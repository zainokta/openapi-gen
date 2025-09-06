package common

import (
	"os"
	"path/filepath"
	"strings"
)

// FileSystemUtilities provides common filesystem operations for source code analysis
type FileSystemUtilities struct{}

// NewFileSystemUtilities creates a new FileSystemUtilities instance
func NewFileSystemUtilities() *FileSystemUtilities {
	return &FileSystemUtilities{}
}

// FindGoModPath finds the go.mod file path by searching up from startDir
func (fs *FileSystemUtilities) FindGoModPath(startDir string) string {
	dir := startDir
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return goModPath
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// GetModuleNameFromGoMod extracts module name from go.mod file
func (fs *FileSystemUtilities) GetModuleNameFromGoMod(goModPath string) string {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimPrefix(line, "module ")
		}
	}

	return ""
}

// IsDirectory checks if a path is a directory
func (fs *FileSystemUtilities) IsDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// HasGoFiles checks if a directory contains any Go files
func (fs *FileSystemUtilities) HasGoFiles(dir string) bool {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".go") {
			return true
		}
	}

	return false
}
