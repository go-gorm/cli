package adapter

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Migration represents a named schema change.
type Migration struct {
	Name string
	Up   func(tx *gorm.DB) error
	Down func(tx *gorm.DB) error
}

// Adapter describes the contract used by migrations/main.go.
type Adapter interface {
	Up(UpOptions) error
	Down(DownOptions) error
	Status(StatusOptions) error
	Diff(DiffOptions) error
	RegisterMigration(Migration)
	GenerateModel(GenerateModelOptions) error
	GenerateMigration(GenerateMigrationOptions) error
}

// UpOptions controls how many migrations to apply.
type UpOptions struct {
	Limit int
}

// DownOptions controls how many migrations to rollback.
type DownOptions struct {
	Steps int
}

// StatusOptions currently holds no fields; defined for future extension.
type StatusOptions struct{}

// DiffOptions currently holds no fields; defined for future extension.
type DiffOptions struct{}

// GenerateModelOptions drives DBAdapter.GenerateModel.
type GenerateModelOptions struct {
	DryRun      bool
	AutoApprove bool
	Tables      []string
}

// GenerateMigrationOptions drives DBAdapter.GenerateMigration.
type GenerateMigrationOptions struct {
	Name        string
	DryRun      bool
	AutoApprove bool
}

// Config configures the DBAdapter.
type Config struct {
	ModelsDir     string
	MigrationsDir string
}

// DBAdapter implements Adapter using a gorm.DB connection.
type DBAdapter struct {
	db         *gorm.DB
	cfg        Config
	migrations map[string]Migration
}

// NewDBAdapter wires a DBAdapter for the provided DB connection.
func NewDBAdapter(db *gorm.DB, cfg Config) (*DBAdapter, error) {
	if db == nil {
		return nil, errors.New("migration runtime: db is required")
	}
	return &DBAdapter{db: db, cfg: cfg, migrations: make(map[string]Migration)}, nil
}

func (a *DBAdapter) RegisterMigration(m Migration) {
	if m.Name == "" {
		panic("migration runtime: migration must have a name")
	}
	a.migrations[m.Name] = m
}

func (a *DBAdapter) ensureSchemaTable() error {
	return a.db.AutoMigrate(&schemaMigration{})
}

// Up applies pending migrations, tracking state in schema_migrations.
func (a *DBAdapter) Up(opts UpOptions) error {
	if err := a.ensureSchemaTable(); err != nil {
		return err
	}
	pending, err := a.pendingMigrations()
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		fmt.Fprintln(os.Stdout, "No pending migrations")
		return nil
	}
	if opts.Limit > 0 && opts.Limit < len(pending) {
		pending = pending[:opts.Limit]
	}
	for _, m := range pending {
		if err := a.db.Transaction(func(tx *gorm.DB) error {
			if err := m.Up(tx); err != nil {
				return err
			}
			return a.recordApplied(m.Name)
		}); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Applied %s\n", m.Name)
	}
	return nil
}

// Down rolls back the latest applied migrations.
func (a *DBAdapter) Down(opts DownOptions) error {
	steps := opts.Steps
	if steps <= 0 {
		steps = 1
	}
	if err := a.ensureSchemaTable(); err != nil {
		return err
	}
	applied, err := a.appliedMigrationsDesc()
	if err != nil {
		return err
	}
	if len(applied) == 0 {
		fmt.Fprintln(os.Stdout, "No applied migrations")
		return nil
	}
	if steps > len(applied) {
		steps = len(applied)
	}
	for i := 0; i < steps; i++ {
		record := applied[i]
		mig, ok := a.migrationByName(record.Name)
		if !ok {
			return fmt.Errorf("migration runtime: migration %s not registered", record.Name)
		}
		if mig.Down == nil {
			return fmt.Errorf("migration runtime: migration %s has no Down function", record.Name)
		}
		if err := a.db.Transaction(func(tx *gorm.DB) error {
			if err := mig.Down(tx); err != nil {
				return err
			}
			return a.removeApplied(record.Name)
		}); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Rolled back %s\n", record.Name)
	}
	return nil
}

// Status prints applied/pending migrations.
func (a *DBAdapter) Status(_ StatusOptions) error {
	if err := a.ensureSchemaTable(); err != nil {
		return err
	}
	applied, err := a.appliedMigrationsAsc()
	if err != nil {
		return err
	}
	appliedSet := make(map[string]time.Time, len(applied))
	for _, record := range applied {
		appliedSet[record.Name] = record.AppliedAt
	}
	regs := a.registeredMigrations()
	fmt.Fprintln(os.Stdout, "NAME\tSTATUS\tAPPLIED AT")
	for _, mig := range regs {
		if ts, ok := appliedSet[mig.Name]; ok {
			fmt.Fprintf(os.Stdout, "%s\tapplied\t%s\n", mig.Name, ts.UTC().Format(time.RFC3339))
		} else {
			fmt.Fprintf(os.Stdout, "%s\tpending\t-\n", mig.Name)
		}
	}
	fmt.Fprintf(os.Stdout, "Total: %d | Applied: %d | Pending: %d\n", len(regs), len(applied), len(regs)-len(applied))
	return nil
}

// Diff prints pending migrations (alias for Status pending section).
func (a *DBAdapter) Diff(_ DiffOptions) error {
	pending, err := a.pendingMigrations()
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		fmt.Fprintln(os.Stdout, "Models match the database schema")
		return nil
	}
	fmt.Fprintln(os.Stdout, "Pending migrations detected:")
	for _, mig := range pending {
		fmt.Fprintf(os.Stdout, "- %s\n", mig.Name)
	}
	return nil
}

// GenerateModel writes placeholder structs per table.
func (a *DBAdapter) GenerateModel(opts GenerateModelOptions) error {
	tables, err := a.db.Migrator().GetTables()
	if err != nil {
		return err
	}
	tables = filterTables(tables, opts.Tables)
	if len(tables) == 0 {
		fmt.Fprintln(os.Stdout, "No tables found")
		return nil
	}
	snippet := ""
	ns := schema.NamingStrategy{}
	pkg := "models"
	for _, table := range tables {
		structName := ns.SchemaName(table)
		path := filepath.Join(a.modelsDir(), fmt.Sprintf("%s.go", table))
		content := renderModelFile(pkg, table, structName, snippet)
		if opts.DryRun {
			fmt.Fprintf(os.Stdout, "--- model preview (%s) ---%c%s\n--- end ---\n", path, '\n', content)
			continue
		}
		ok, err := a.confirmWrite(path, opts.AutoApprove)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintf(os.Stdout, "Skipped %s\n", path)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Model written: %s\n", path)
	}
	return nil
}

// GenerateMigration scaffolds a timestamped migration file.
func (a *DBAdapter) GenerateMigration(opts GenerateMigrationOptions) error {
	if opts.Name == "" {
		return errors.New("migration name is required")
	}
	ts := time.Now().UTC().Format("20060102150405")
	slug := slugify(opts.Name)
	filename := fmt.Sprintf("%s_%s.go", ts, slug)
	path := filepath.Join(a.migrationsDir(), filename)
	content := renderMigrationFile(strings.TrimSuffix(filename, ".go"))
	if opts.DryRun {
		fmt.Fprintf(os.Stdout, "--- migration preview (%s) ---%c%s\n--- end ---\n", path, '\n', content)
		return nil
	}
	ok, err := a.confirmWrite(path, opts.AutoApprove)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintf(os.Stdout, "Skipped %s\n", path)
		return nil
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

func (a *DBAdapter) confirmWrite(path string, auto bool) (bool, error) {
	info, err := os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if errors.Is(err, os.ErrNotExist) && auto {
		return true, nil
	}
	action := "create"
	if err == nil && info.Mode().IsRegular() {
		action = "overwrite"
	}
	if auto {
		return true, nil
	}
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprintf(os.Stdout, "%s %s? [y/N]: ", strings.Title(action), path)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	response := strings.TrimSpace(strings.ToLower(line))
	return response == "y" || response == "yes", nil
}

func (a *DBAdapter) registeredMigrations() []Migration {
	if len(a.migrations) == 0 {
		return nil
	}
	out := make([]Migration, 0, len(a.migrations))
	for _, m := range a.migrations {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func (a *DBAdapter) migrationByName(name string) (Migration, bool) {
	m, ok := a.migrations[name]
	return m, ok
}

func (a *DBAdapter) modelsDir() string {
	if filepath.IsAbs(a.cfg.ModelsDir) {
		return a.cfg.ModelsDir
	}
	return filepath.Clean(a.cfg.ModelsDir)
}

func (a *DBAdapter) migrationsDir() string {
	if filepath.IsAbs(a.cfg.MigrationsDir) {
		return a.cfg.MigrationsDir
	}
	return filepath.Clean(a.cfg.MigrationsDir)
}

func filterTables(all, subset []string) []string {
	if len(subset) == 0 {
		sort.Strings(all)
		return all
	}
	filter := make(map[string]struct{}, len(subset))
	for _, name := range subset {
		filter[name] = struct{}{}
	}
	out := make([]string, 0, len(subset))
	for _, table := range all {
		if _, ok := filter[table]; ok {
			out = append(out, table)
		}
	}
	sort.Strings(out)
	return out
}
