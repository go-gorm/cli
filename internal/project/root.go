package project

import (
	"os"
	"path/filepath"
)

// ResolveRootPath ensures path is anchored to the detected root when relative.
func ResolveRootPath(path string) string {
	if path == "" {
		path = "."
	}
	root := Root()
	if filepath.IsAbs(path) || root == "" {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(root, path))
}

// Root returns the best-effort project root (preferring go.mod over .git).
func Root() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	wd = filepath.Clean(wd)
	if root, ok := findUp(wd, "go.mod", func(info os.FileInfo) bool {
		return !info.IsDir()
	}); ok {
		return root
	}
	if root, ok := findUp(wd, ".git", func(os.FileInfo) bool { return true }); ok {
		return root
	}
	return wd
}

func findUp(start, marker string, accept func(os.FileInfo) bool) (string, bool) {
	dir := start
	for {
		candidate := filepath.Join(dir, marker)
		if info, err := os.Stat(candidate); err == nil && accept(info) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}
