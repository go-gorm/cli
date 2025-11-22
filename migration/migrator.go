package migration

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"gorm.io/cli/gorm/internal/migration/adapter"
	"gorm.io/cli/gorm/internal/project"
	"gorm.io/gorm"
)

type (
	Migration   = adapter.Migration
	FieldRule   = adapter.FieldRule
	TableConfig = adapter.TableConfig
	TableRule   = adapter.TableRule
)

// Config configures the migration migrator embedded in generated projects.
type Config struct {
	ModelsDir     string
	MigrationsDir string
	TableRules    []TableRule
}

// Option mutates a Migrator configuration during construction.
type Option func(*Migrator)

// Migrator executes migration commands using the provided configuration.
type Migrator struct {
	cfg      Config
	args     []string
	adapters []adapter.Adapter
}

// New creates a Migrator with sane defaults for missing configuration fields.
func New(cfg Config, opts ...Option) *Migrator {
	cfg.ModelsDir = project.ResolveRootPath(cfg.ModelsDir)
	cfg.MigrationsDir = project.ResolveRootPath(cfg.MigrationsDir)
	r := &Migrator{cfg: cfg}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

// WithDBAdapter injects the DB connection used to build a runtime adapter.
func WithDBAdapter(db *gorm.DB) Option {
	return func(r *Migrator) {
		if db == nil {
			return
		}
		adp, err := adapter.NewDBAdapter(db, adapter.Config{
			ModelsDir:     r.cfg.ModelsDir,
			MigrationsDir: r.cfg.MigrationsDir,
			TableRules:    r.cfg.TableRules,
		})
		if err != nil {
			log.Print(err)
			return
		}
		r.adapters = append(r.adapters, adp)
	}
}

// Run executes the migration command, registering the provided migrations and exiting on error.
func (r *Migrator) Run(migrations []Migration) {
	for _, adp := range r.adapters {
		adp.RegisterMigrations(migrations)

		if err := r.run(adp, r.args); err != nil {
			log.Print(err)
			os.Exit(1)
		}
	}
}

func (r *Migrator) run(adp adapter.Adapter, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing command (up/down/status/diff/reflect/create)")
	}
	cmd := args[0]
	rest := args[1:]

	if adp == nil && cmd != "create" {
		return fmt.Errorf("adapter is required for %s", cmd)
	}

	switch cmd {
	case "up":
		return r.runUp(adp, rest)
	case "down":
		return r.runDown(adp, rest)
	case "status":
		return adp.Status(adapter.StatusOptions{})
	case "diff":
		return adp.Diff(adapter.DiffOptions{})
	case "reflect":
		return r.runReflect(adp, rest)
	case "create":
		return r.runCreate(adp, rest)
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func (r *Migrator) runReflect(adp adapter.Adapter, args []string) error {
	fs := flag.NewFlagSet("reflect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dryRun := fs.Bool("dry-run", false, "Preview generated code without writing to disk")
	auto := fs.Bool("yes", false, "Skip confirmation prompts")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return adp.GenerateModel(adapter.GenerateModelOptions{
		DryRun:      *dryRun,
		AutoApprove: *auto,
	})
}

func (r *Migrator) runCreate(adp adapter.Adapter, args []string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dryRun := fs.Bool("dry-run", false, "Preview migration contents without creating a file")
	yes := fs.Bool("yes", false, "Skip confirmation prompts")
	auto := fs.Bool("auto", false, "Auto-generate from model/DB diff (requires DB adapter)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return fmt.Errorf("migration name is required")
	}
	name := fs.Arg(0)
	if *auto {
		if adp == nil {
			return fmt.Errorf("--auto requires a DB adapter")
		}
		return adp.GenerateMigration(adapter.GenerateMigrationOptions{
			Name:        name,
			DryRun:      *dryRun,
			AutoApprove: *yes,
		})
	}
	return r.writeEmptyMigration(name, *dryRun, *yes)
}

func (r *Migrator) writeEmptyMigration(name string, dryRun, yes bool) error {
	timestamp := time.Now().UTC().Format("20060102150405")
	slug := project.Slugify(name)
	filename := fmt.Sprintf("%s_%s.go", timestamp, slug)
	path := filepath.Join(r.cfg.MigrationsDir, filename)
	content := renderEmptyMigration(strings.TrimSuffix(filename, ".go"))

	if dryRun {
		fmt.Fprintf(os.Stdout, "--- migration preview (%s) ---%c%s\n--- end ---\n", path, '\n', content)
		return nil
	}
	if info, err := os.Stat(path); err == nil && !yes && info.Mode().IsRegular() {
		return fmt.Errorf("%s already exists (use --yes to overwrite)", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Migration created: %s\n", path)
	return nil
}

func renderEmptyMigration(name string) string {
	var buf bytes.Buffer
	_ = emptyMigrationTemplate.Execute(&buf, struct{ Name string }{Name: name})
	return buf.String()
}

func (r *Migrator) runUp(adp adapter.Adapter, args []string) error {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	limit := fs.Int("limit", 0, "number of migrations to apply")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return adp.Up(adapter.UpOptions{Limit: *limit})
}

func (r *Migrator) runDown(adp adapter.Adapter, args []string) error {
	fs := flag.NewFlagSet("down", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	steps := fs.Int("steps", 1, "number of migrations to rollback")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return adp.Down(adapter.DownOptions{Steps: *steps})
}

var emptyMigrationTemplate = template.Must(template.New("migration").Parse(`package main

import (
    "gorm.io/cli/gorm/migration"
    "gorm.io/gorm"
)

func init() {
    register(migration.Migration{
        Name: "{{.Name}}",
        Up: func(tx *gorm.DB) error {
            // TODO: implement forward migration logic
            return nil
        },
        Down: func(tx *gorm.DB) error {
            // TODO: implement rollback logic
            return nil
        },
    })
}
`))
