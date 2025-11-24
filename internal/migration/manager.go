package migration

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"gorm.io/cli/gorm/internal/project"
)

const (
	defaultModelsDirName     = "models"
	defaultMigrationsDirName = "migrations"
	defaultRunnerFileName    = "main.go"
	diffHelperFile           = "gorm_generated_diff_models_helper.go"
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
	Force bool
}

func (mgr Manager) Init(opts InitOptions) error {
	migrationsDir := project.ResolveRootPath(mgr.MigrationsDir)
	runnerFile := filepath.Join(migrationsDir, defaultRunnerFileName)

	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", migrationsDir, err)
	}

	if err := writeRunnerFile(runnerFile, opts.Force, mgr); err != nil {
		return err
	}

	return nil
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

func (mgr Manager) generateDiffFile() (string, error) {
	migrationsDir := project.ResolveRootPath(mgr.MigrationsDir)
	cmd := exec.Command(mgr.GoCmd, "run", ".", "diff", "--generated-file")
	cmd.Dir = migrationsDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("generate diff helper: %v\n%s", err, stderr.String())
	}
	path := filepath.Join(migrationsDir, diffHelperFile)
	if err := os.WriteFile(path, stdout.Bytes(), 0o644); err != nil {
		return "", fmt.Errorf("write diff helper: %w", err)
	}
	return path, nil
}
