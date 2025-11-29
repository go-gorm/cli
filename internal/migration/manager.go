package migration

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"gorm.io/cli/gorm/internal/project"
	"gorm.io/cli/gorm/internal/utils"
)

const (
	defaultModelsDirName     = "models"
	defaultMigrationsDirName = "migrations"
	defaultRunnerFileName    = "main.go"
)

// ErrNotInitialized is returned when the migrations folder has not been bootstrapped yet.
var ErrNotInitialized = errors.New("migration project is not initialized; run 'gorm migrate init'")

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
	flags := utils.ConfirmSkipIfMissing
	if autoApprove {
		flags |= utils.ConfirmAuto
	}
	ok, err := utils.ConfirmWrite(path, flags)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintf(os.Stdout, "Skipped %s\n", path)
		return nil
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

var runnerTemplate = template.Must(template.New("runner").Parse(defaultRunnerTemplate))

const defaultRunnerTemplate = `package main

import (
	"gorm.io/cli/gorm/migration"
	"gorm.io/gorm"
)

var (
	DB         *gorm.DB
	migrations []migration.Migration
	// tablesRules defines the configuration for model reflection.
	// Each rule matches tables using shell-style patterns.
	tablesRules = []migration.TableRule{
		// {
		// 	 Pattern: "users",
		// 	 Config: migration.TableConfig{
		// 		OutputPath: "internal/models/user.go",
		// 		FieldRules: []migration.FieldRule{
		// 			{Pattern: "name", FieldName: "FullName", Tags: map[string]string{"json": "{{"{{"}}.DBName{{"}}"}}"}},
		// 		},
		// 	 },
		// },
		// {
		// 	Pattern: "audit_*",
		// 	Exclude: true,
		// },
	}
)

func register(m migration.Migration) {
	migrations = append(migrations, m)
}

func main() {
	migration.New(migration.Config{
		ModelsDir:     {{printf "%q" .ModelsDir}},
		MigrationsDir: {{printf "%q" .MigrationsDir}},
		TableRules:    tablesRules,
	}, migration.WithDBAdapter(DB)).Run(migrations)
}
`
