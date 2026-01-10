package migration

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"gorm.io/cli/gorm/internal/migration/templates"
	"gorm.io/cli/gorm/internal/project"
	"gorm.io/cli/gorm/internal/utils"
)

const (
	defaultModelsDirName     = "models"
	defaultMigrationsDirName = "migrations"
	defaultRunnerFileName    = "main.go"
)

// Manager owns the operations performed by the migration CLI.
type Manager struct {
	ModelsDir     string
	MigrationsDir string
	GoCmd         string
}

// InitOptions configures the init command.
type InitOptions struct {
	AutoApprove bool
}

func (mgr Manager) Init(opts InitOptions) error {
	migrationsDir := project.ResolveRootPath(mgr.MigrationsDir)
	runnerFile := filepath.Join(migrationsDir, defaultRunnerFileName)

	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", migrationsDir, err)
	}

	if err := writeRunnerFile(runnerFile, opts.AutoApprove, mgr); err != nil {
		return err
	}

	return nil
}

func writeRunnerFile(path string, autoApprove bool, data Manager) error {
	// Generate the new content first to allow for diffing.
	var buf bytes.Buffer
	if err := templates.RunnerTemplate.Execute(&buf, data); err != nil {
		return fmt.Errorf("render runner template: %w", err)
	}
	newContent := buf.Bytes()

	// Read original content, ignoring error if it doesn't exist.
	originalContent, _ := os.ReadFile(path)

	// Use the centralized utility for writing the file.
	// dryRun is false because this command doesn't support it directly.
	// skipIfMissing is true to maintain the original behavior.
	return utils.WriteFileWithConfirmation(path, originalContent, newContent, false, autoApprove, true)
}
