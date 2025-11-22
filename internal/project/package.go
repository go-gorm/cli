package project

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// DetectPackage returns the name of the first Go package found in dir.
func DetectPackage(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly)
		if err == nil && file.Name != nil {
			return file.Name.Name
		}
	}
	return ""
}
