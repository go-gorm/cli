package migration

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"gorm.io/cli/gorm/migration/runtime"
	"gorm.io/gorm"
)

// Config configures the migration runner embedded in generated projects.
type Config struct {
	DB    *gorm.DB
	Token string

	ModelsDir     string
	MigrationsDir string

	Args   []string
	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

// Option mutates a Runner configuration during construction.
type Option func(*Runner)

// Runner executes migration commands using the provided configuration.
type Runner struct {
	cfg Config
}

// New creates a Runner with sane defaults for missing configuration fields.
func New(cfg Config, opts ...Option) *Runner {
	r := &Runner{cfg: cfg}
	r.applyDefaults()
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

func (r *Runner) applyDefaults() {
	if r.cfg.Stdout == nil {
		r.cfg.Stdout = os.Stdout
	}
	if r.cfg.Stderr == nil {
		r.cfg.Stderr = os.Stderr
	}
	if r.cfg.Stdin == nil {
		r.cfg.Stdin = os.Stdin
	}
	if r.cfg.Args == nil {
		r.cfg.Args = os.Args[1:]
	}
	if r.cfg.ModelsDir == "" {
		r.cfg.ModelsDir = "models"
	}
	if r.cfg.MigrationsDir == "" {
		r.cfg.MigrationsDir = "migrations"
	}
}

// WithDBAdaptor injects the DB connection used to build a runtime adaptor.
func WithDBAdaptor(db *gorm.DB) Option {
	return func(r *Runner) {
		r.cfg.DB = db
	}
}

// WithDBAdapter is an alias for WithDBAdaptor.
func WithDBAdapter(db *gorm.DB) Option {
	return WithDBAdaptor(db)
}

// Run executes the migration command, registering the provided migrations and exiting on error.
func (r *Runner) Run(migrations []runtime.Migration) {
	if err := r.RunE(migrations); err != nil {
		fmt.Fprintln(r.cfg.Stderr, err)
		os.Exit(1)
	}
}

// RunE executes the migration command with the provided migrations and returns any error encountered.
func (r *Runner) RunE(migrations []runtime.Migration) error {
	if r.cfg.DB == nil {
		return errors.New("migration: DB is required")
	}
	r.registerAll(migrations)
	return r.run(r.cfg.Args)
}

// SetDB updates the database connection used by the runner.
func (r *Runner) SetDB(db *gorm.DB) *Runner {
	r.cfg.DB = db
	return r
}

func (r *Runner) registerAll(migrations []runtime.Migration) {
	for _, migration := range migrations {
		runtime.RegisterMigration(migration)
	}
}

func (r *Runner) run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing command (up/down/status/diff/gen)")
	}
	adapter, err := r.newAdapter()
	if err != nil {
		return err
	}
	cmd := args[0]
	rest := args[1:]
	switch cmd {
	case "up":
		return r.runUp(adapter, rest)
	case "down":
		return r.runDown(adapter, rest)
	case "status":
		return adapter.Status()
	case "diff":
		return adapter.Diff()
	case "gen":
		return r.runGen(adapter, rest)
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func (r *Runner) runUp(adapter runtime.Adapter, args []string) error {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	fs.SetOutput(r.cfg.Stderr)
	limit := fs.Int("limit", 0, "number of migrations to apply")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return adapter.Up(*limit)
}

func (r *Runner) runDown(adapter runtime.Adapter, args []string) error {
	fs := flag.NewFlagSet("down", flag.ContinueOnError)
	fs.SetOutput(r.cfg.Stderr)
	steps := fs.Int("steps", 1, "number of migrations to rollback")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return adapter.Down(*steps)
}

func (r *Runner) runGen(adapter runtime.Adapter, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing gen subcommand (model/migration)")
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "model":
		return r.runGenModel(adapter, rest)
	case "migration":
		return r.runGenMigration(adapter, rest)
	default:
		return fmt.Errorf("unknown gen subcommand: %s", sub)
	}
}

func (r *Runner) runGenModel(adapter runtime.Adapter, args []string) error {
	fs := flag.NewFlagSet("gen-model", flag.ContinueOnError)
	fs.SetOutput(r.cfg.Stderr)
	pkg := fs.String("package", "models", "Package name for generated files")
	schema := fs.String("schema", "", "Optional schema note to embed")
	dryRun := fs.Bool("dry-run", false, "Preview generated code without writing to disk")
	auto := fs.Bool("yes", false, "Skip confirmation prompts")
	var tables stringList
	fs.Var(&tables, "table", "Table to include (repeat flag for multiple)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return adapter.GenerateModel(runtime.GenerateModelOptions{
		PackageName: *pkg,
		SchemaPath:  *schema,
		DryRun:      *dryRun,
		AutoApprove: *auto,
		Tables:      tables,
	})
}

func (r *Runner) runGenMigration(adapter runtime.Adapter, args []string) error {
	fs := flag.NewFlagSet("gen-migration", flag.ContinueOnError)
	fs.SetOutput(r.cfg.Stderr)
	name := fs.String("name", "", "Descriptive migration name (e.g. add_users_table)")
	dryRun := fs.Bool("dry-run", false, "Preview migration contents without creating a file")
	auto := fs.Bool("yes", false, "Skip confirmation prompts")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("--name is required")
	}
	return adapter.GenerateMigration(runtime.GenerateMigrationOptions{
		Name:        *name,
		DryRun:      *dryRun,
		AutoApprove: *auto,
	})
}

func (r *Runner) newAdapter() (runtime.Adapter, error) {
	return runtime.NewDBAdapter(r.cfg.DB, runtime.Config{
		RootDir:       ".",
		ModelsDir:     r.cfg.ModelsDir,
		MigrationsDir: r.cfg.MigrationsDir,
		Stdout:        r.cfg.Stdout,
		Stderr:        r.cfg.Stderr,
		Stdin:         r.cfg.Stdin,
	})
}

// Definition describes a migration that can be registered with the runner.
type Definition = runtime.Migration

// Register registers a migration definition so it can be picked up by commands.
func (r *Runner) Register(m Definition) {
	runtime.RegisterMigration(runtime.Migration(m))
}

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}
