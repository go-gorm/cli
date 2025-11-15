package migration

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

const (
	defaultModelsDirName     = "models"
	defaultMigrationsDirName = "migrations"
	defaultRunnerFileName    = "main.go"
)

var (
	// ErrNotInitialized is returned when the migrations folder has not been bootstrapped yet.
	ErrNotInitialized = errors.New("migration project is not initialized; run 'gorm migrate init'")
)

// Manager owns the operations performed by the migration CLI.
type Manager struct {
	ModelsDir     string
	MigrationsDir string
}

// InitOptions configures the init command.
type InitOptions struct {
	Force bool
}

func (mgr Manager) Init(opts InitOptions) error {
	migrationsDir := normalizePath(mgr.MigrationsDir)
	runnerFile := filepath.Join(migrationsDir, defaultRunnerFileName)

	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", migrationsDir, err)
	}

	if err := writeRunnerFile(runnerFile, opts.Force, mgr); err != nil {
		return err
	}

	return nil
}

func normalizePath(value string) string {
	if value == "" {
		return "."
	}
	return filepath.Clean(value)
}

func writeRunnerFile(path string, force bool, data Manager) error {
	if err := preventOverwrite(path, force); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("prepare runner dir: %w", err)
	}
	var buf bytes.Buffer
	if err := runnerTemplate.Execute(&buf, data); err != nil {
		return fmt.Errorf("render runner template: %w", err)
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func preventOverwrite(path string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", path)
		}
	}
	return nil
}

var runnerTemplate = template.Must(template.New("runner").Parse(defaultRunnerTemplate))

const defaultRunnerTemplate = `package main

import (
	"gorm.io/cli/gorm/migration"
	"gorm.io/gorm"
)

var migrations []migration.Migration

func register(m migration.Migration) {
	migrations = append(migrations, m)
}

func main() {
	// FIXME initialize your gorm DB connection here
	var DB *gorm.DB

	migration.New(migration.Config{
		ModelsDir:     {{printf "%q" .ModelsDir}},
		MigrationsDir: {{printf "%q" .MigrationsDir}},
	}, migration.WithDBAdaptor(DB)).Run(migrations)
}
`
