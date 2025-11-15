package runtime

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

// Adapter describes the contract used by migrations/main.go.
type Adapter interface {
	Up(limit int) error
	Down(steps int) error
	Status() error
	Diff() error
	GenerateModel(GenerateModelOptions) error
	GenerateMigration(GenerateMigrationOptions) error
}

// GenerateModelOptions drives DBAdapter.GenerateModel.
type GenerateModelOptions struct {
	PackageName string
	SchemaPath  string
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
	RootDir       string
	ModelsDir     string
	MigrationsDir string
	Stdout        io.Writer
	Stderr        io.Writer
	Stdin         io.Reader
}

// DBAdapter implements Adapter using a gorm.DB connection.
type DBAdapter struct {
	db  *gorm.DB
	cfg Config
}

// NewDBAdapter wires a DBAdapter for the provided DB connection.
func NewDBAdapter(db *gorm.DB, cfg Config) (*DBAdapter, error) {
	if db == nil {
		return nil, errors.New("migration runtime: db is required")
	}
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	if cfg.Stderr == nil {
		cfg.Stderr = os.Stderr
	}
	if cfg.Stdin == nil {
		cfg.Stdin = os.Stdin
	}
	if cfg.RootDir == "" {
		cfg.RootDir = "."
	}
	if cfg.ModelsDir == "" {
		cfg.ModelsDir = "."
	}
	if cfg.MigrationsDir == "" {
		cfg.MigrationsDir = "."
	}
	return &DBAdapter{db: db, cfg: cfg}, nil
}

func (a *DBAdapter) ensureSchemaTable() error {
	return a.db.AutoMigrate(&schemaMigration{})
}

// Up applies pending migrations, tracking state in schema_migrations.
func (a *DBAdapter) Up(limit int) error {
	if err := a.ensureSchemaTable(); err != nil {
		return err
	}
	pending, err := a.pendingMigrations()
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		fmt.Fprintln(a.cfg.Stdout, "No pending migrations")
		return nil
	}
	if limit > 0 && limit < len(pending) {
		pending = pending[:limit]
	}
	for _, m := range pending {
		if err := a.db.Transaction(m.Up); err != nil {
			return err
		}
		if err := a.recordApplied(m.Name); err != nil {
			return err
		}
		fmt.Fprintf(a.cfg.Stdout, "Applied %s\n", m.Name)
	}
	return nil
}

// Down rolls back the latest applied migrations.
func (a *DBAdapter) Down(steps int) error {
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
		fmt.Fprintln(a.cfg.Stdout, "No applied migrations")
		return nil
	}
	if steps > len(applied) {
		steps = len(applied)
	}
	for i := 0; i < steps; i++ {
		record := applied[i]
		mig, ok := migrationByName(record.Name)
		if !ok {
			return fmt.Errorf("migration runtime: migration %s not registered", record.Name)
		}
		if mig.Down == nil {
			return fmt.Errorf("migration runtime: migration %s has no Down function", record.Name)
		}
		if err := a.db.Transaction(mig.Down); err != nil {
			return err
		}
		if err := a.removeApplied(record.Name); err != nil {
			return err
		}
		fmt.Fprintf(a.cfg.Stdout, "Rolled back %s\n", record.Name)
	}
	return nil
}

// Status prints applied/pending migrations.
func (a *DBAdapter) Status() error {
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
	regs := registeredMigrations()
	fmt.Fprintln(a.cfg.Stdout, "NAME\tSTATUS\tAPPLIED AT")
	for _, mig := range regs {
		if ts, ok := appliedSet[mig.Name]; ok {
			fmt.Fprintf(a.cfg.Stdout, "%s\tapplied\t%s\n", mig.Name, ts.UTC().Format(time.RFC3339))
		} else {
			fmt.Fprintf(a.cfg.Stdout, "%s\tpending\t-\n", mig.Name)
		}
	}
	fmt.Fprintf(a.cfg.Stdout, "Total: %d | Applied: %d | Pending: %d\n", len(regs), len(applied), len(regs)-len(applied))
	return nil
}

// Diff prints pending migrations (alias for Status pending section).
func (a *DBAdapter) Diff() error {
	pending, err := a.pendingMigrations()
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		fmt.Fprintln(a.cfg.Stdout, "Models match the database schema")
		return nil
	}
	fmt.Fprintln(a.cfg.Stdout, "Pending migrations detected:")
	for _, mig := range pending {
		fmt.Fprintf(a.cfg.Stdout, "- %s\n", mig.Name)
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
		fmt.Fprintln(a.cfg.Stdout, "No tables found")
		return nil
	}
	snippet := ""
	if opts.SchemaPath != "" {
		schemaPath := opts.SchemaPath
		if !filepath.IsAbs(schemaPath) {
			schemaPath = filepath.Join(a.rootDir(), schemaPath)
		}
		data, err := os.ReadFile(schemaPath)
		if err != nil {
			return fmt.Errorf("read schema: %w", err)
		}
		snippet = strings.TrimSpace(string(data))
	}
	ns := schema.NamingStrategy{}
	pkg := opts.PackageName
	if pkg == "" {
		pkg = "models"
	}
	for _, table := range tables {
		structName := ns.SchemaName(table)
		path := filepath.Join(a.modelsDir(), fmt.Sprintf("%s.go", table))
		content := renderModelFile(pkg, table, structName, snippet)
		if opts.DryRun {
			fmt.Fprintf(a.cfg.Stdout, "--- model preview (%s) ---%c%s\n--- end ---\n", path, '\n', content)
			continue
		}
		ok, err := a.confirmWrite(path, opts.AutoApprove)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintf(a.cfg.Stdout, "Skipped %s\n", path)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(a.cfg.Stdout, "Model written: %s\n", path)
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
		fmt.Fprintf(a.cfg.Stdout, "--- migration preview (%s) ---%c%s\n--- end ---\n", path, '\n', content)
		return nil
	}
	ok, err := a.confirmWrite(path, opts.AutoApprove)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintf(a.cfg.Stdout, "Skipped %s\n", path)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(a.cfg.Stdout, "Migration created: %s\n", path)
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
	reader := bufio.NewReader(a.cfg.Stdin)
	fmt.Fprintf(a.cfg.Stdout, "%s %s? [y/N]: ", strings.Title(action), path)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	response := strings.TrimSpace(strings.ToLower(line))
	return response == "y" || response == "yes", nil
}

func (a *DBAdapter) rootDir() string {
	return a.cfg.RootDir
}

func (a *DBAdapter) modelsDir() string {
	if filepath.IsAbs(a.cfg.ModelsDir) {
		return a.cfg.ModelsDir
	}
	return filepath.Join(a.rootDir(), a.cfg.ModelsDir)
}

func (a *DBAdapter) migrationsDir() string {
	if filepath.IsAbs(a.cfg.MigrationsDir) {
		return a.cfg.MigrationsDir
	}
	return filepath.Join(a.rootDir(), a.cfg.MigrationsDir)
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
