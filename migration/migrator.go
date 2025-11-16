package migration

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/cli/gorm/migration/adapter"
	"gorm.io/gorm"
)

// Migration is an alias of adapter.Migration for convenience in templates.
type Migration = adapter.Migration

// Config configures the migration migrator embedded in generated projects.
type Config struct {
	ModelsDir     string
	MigrationsDir string

	Args []string
}

// Option mutates a Migrator configuration during construction.
type Option func(*Migrator)

// Migrator executes migration commands using the provided configuration.
type Migrator struct {
	cfg      Config
	adapters []adapter.Adapter
}

// New creates a Migrator with sane defaults for missing configuration fields.
func New(cfg Config, opts ...Option) *Migrator {
	r := &Migrator{cfg: cfg}
	r.applyDefaults()
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

func (r *Migrator) applyDefaults() {
	if r.cfg.Args == nil {
		r.cfg.Args = os.Args[1:]
	}
	if r.cfg.ModelsDir == "" {
		r.cfg.ModelsDir = "models"
	}
	if r.cfg.MigrationsDir == "" {
		r.cfg.MigrationsDir = "migrations"
	}
	if wd, err := os.Getwd(); err == nil {
		if filepath.Base(filepath.Clean(wd)) == r.cfg.MigrationsDir {
			r.cfg.MigrationsDir = "."
		}
	}
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
	for _, m := range migrations {
		for _, adp := range r.adapters {
			adp.RegisterMigration(m)
		}
	}
	var primary adapter.Adapter
	if len(r.adapters) > 0 {
		primary = r.adapters[0]
	}
	if err := r.run(primary, r.cfg.Args); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func (r *Migrator) run(adp adapter.Adapter, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing command (up/down/status/diff/reflect/create)")
	}
	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "up":
		return r.runUp(adp, rest)
	case "down":
		return r.runDown(adp, rest)
	case "status":
		if adp == nil {
			return fmt.Errorf("adapter is required for status")
		}
		return adp.Status(adapter.StatusOptions{})
	case "diff":
		if adp == nil {
			return fmt.Errorf("adapter is required for diff")
		}
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
	if adp == nil {
		return fmt.Errorf("adapter is required for reflect")
	}
	fs := flag.NewFlagSet("reflect", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	dryRun := fs.Bool("dry-run", false, "Preview generated code without writing to disk")
	auto := fs.Bool("yes", false, "Skip confirmation prompts")
	tables := fs.String("table", "", "Comma-separated tables to include")
	if err := fs.Parse(args); err != nil {
		return err
	}
	var tableList []string
	if v := strings.TrimSpace(*tables); v != "" {
		parts := strings.Split(v, ",")
		tableList = make([]string, 0, len(parts))
		for _, p := range parts {
			if name := strings.TrimSpace(p); name != "" {
				tableList = append(tableList, name)
			}
		}
	}
	return adp.GenerateModel(adapter.GenerateModelOptions{
		DryRun:      *dryRun,
		AutoApprove: *auto,
		Tables:      tableList,
	})
}

func (r *Migrator) runCreate(adp adapter.Adapter, args []string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	name := fs.String("name", "", "Migration name")
	dryRun := fs.Bool("dry-run", false, "Preview migration contents without creating a file")
	yes := fs.Bool("yes", false, "Skip confirmation prompts")
	auto := fs.Bool("auto", false, "Auto-generate from model/DB diff (requires DB adapter)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" && fs.NArg() > 0 {
		*name = fs.Arg(0)
	}
	if *name == "" {
		return fmt.Errorf("migration name is required")
	}
	if *auto {
		if adp == nil {
			return fmt.Errorf("--auto requires a DB adapter")
		}
		return adp.GenerateMigration(adapter.GenerateMigrationOptions{
			Name:        *name,
			DryRun:      *dryRun,
			AutoApprove: *yes,
		})
	}
	return r.writeEmptyMigration(*name, *dryRun, *yes)
}

func (r *Migrator) writeEmptyMigration(name string, dryRun, yes bool) error {
	timestamp := time.Now().UTC().Format("20060102150405")
	slug := slugify(name)
	if slug == "" {
		slug = "migration"
	}
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
	return fmt.Sprintf(`package main

import (
    "gorm.io/cli/gorm/migration"
    "gorm.io/gorm"
)

func init() {
    register(migration.Migration{
        Name: "%s",
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
`, name)
}

func slugify(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.Trim(value, "_")
	return value
}

func (r *Migrator) runUp(adp adapter.Adapter, args []string) error {
	if adp == nil {
		return fmt.Errorf("adapter is required for up")
	}
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	limit := fs.Int("limit", 0, "number of migrations to apply")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return adp.Up(adapter.UpOptions{Limit: *limit})
}

func (r *Migrator) runDown(adp adapter.Adapter, args []string) error {
	if adp == nil {
		return fmt.Errorf("adapter is required for down")
	}
	fs := flag.NewFlagSet("down", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	steps := fs.Int("steps", 1, "number of migrations to rollback")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return adp.Down(adapter.DownOptions{Steps: *steps})
}
