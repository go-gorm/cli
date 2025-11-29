package adapter

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	astutil "golang.org/x/tools/go/ast/astutil"
	importsutil "golang.org/x/tools/imports"
	"gorm.io/cli/gorm/internal/project"
	"gorm.io/cli/gorm/internal/utils"
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
	RegisterMigrations([]Migration)
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

// DiffOptions controls diff output behaviour.
type DiffOptions struct {
	GeneratedFile bool
	Writer        io.Writer
}

// GenerateModelOptions drives DBAdapter.GenerateModel.
type GenerateModelOptions struct {
	DryRun      bool
	AutoApprove bool
}

// GenerateMigrationOptions drives DBAdapter.GenerateMigration.
type GenerateMigrationOptions struct {
	Name        string
	DryRun      bool
	AutoApprove bool
}

type FieldRule struct {
	Pattern   string
	FieldName string
	FieldType string
	Tags      map[string]string
	Imports   []string
	Exclude   bool
}

type TableConfig struct {
	OutputPath string
	FieldRules []FieldRule
}

// TableRule describes a table-matching rule and associated configuration.
type TableRule struct {
	Pattern string
	Config  TableConfig
	Exclude bool
}

// Config configures the DBAdapter.
type Config struct {
	ModelsDir     string
	MigrationsDir string
	TableRules    []TableRule
	DiffModels    []interface{}
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

func (a *DBAdapter) RegisterMigrations(migs []Migration) {
	for _, m := range migs {
		if m.Name == "" {
			panic("migration runtime: migration must have a name")
		}
		a.migrations[m.Name] = m
	}
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
			return a.recordApplied(tx, m.Name)
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
			return a.removeApplied(tx, record.Name)
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
func (a *DBAdapter) Diff(opts DiffOptions) error {
	if opts.GeneratedFile {
		return a.renderDiffModels(opts.Writer)
	}
	diff, _, _, err := a.loadSchemaDiff()
	if err != nil {
		return err
	}
	if diff.Empty() {
		fmt.Fprintln(os.Stdout, "Models match the database schema")
		return nil
	}
	writeSchemaDiff(os.Stdout, diff)
	return nil
}

// GenerateModel writes placeholder structs per table.
func (a *DBAdapter) GenerateModel(opts GenerateModelOptions) error {
	ns := a.db.NamingStrategy
	modelDir := a.cfg.ModelsDir
	defaultPkg := project.DetectPackage(modelDir)
	if defaultPkg == "" {
		defaultPkg = "models"
	}
	configs, err := a.resolveTableConfigs()
	if err != nil {
		return err
	}
	if len(configs) == 0 {
		fmt.Fprintln(os.Stdout, "No tables found in database")
		return nil
	}
	writeFlags := utils.ConfirmFlag(0)
	if opts.AutoApprove {
		writeFlags = utils.ConfirmAuto
	}

	for tableName := range configs {
		finalConfig := configs[tableName]
		// Determine output path
		var path string
		if finalConfig.OutputPath != "" {
			if filepath.IsAbs(finalConfig.OutputPath) {
				path = finalConfig.OutputPath
			} else {
				path = filepath.Join(project.Root(), finalConfig.OutputPath)
			}
		} else {
			path = filepath.Join(modelDir, fmt.Sprintf("%s.go", project.Slugify(tableName)))
		}
		structName := ns.SchemaName(tableName)
		outputPkg := defaultPkg
		if pkg := project.DetectPackage(filepath.Dir(path)); pkg != "" {
			outputPkg = pkg
		}

		// If file exists, attempt to merge/append before overwriting.
		if _, err := os.Stat(path); err == nil {
			err := a.mergeModelChanges(path, tableName, structName, finalConfig, opts)
			if err == nil {
				continue // Successfully merged or up-to-date
			}
			if errors.Is(err, errStructNotFound) {
				appendErr := a.appendModelDefinition(ns, path, tableName, structName, finalConfig, opts)
				if appendErr == nil {
					continue
				}
				fmt.Fprintf(os.Stderr, "Could not append model %s to %s, falling back to overwrite: %v\n", structName, path, appendErr)
			} else {
				fmt.Fprintf(os.Stderr, "Could not merge changes for %s, falling back to overwrite: %v\n", path, err)
			}
			// Fallback to overwrite logic below
		}

		// Create new file or overwrite on merge failure
		cols, err := a.db.Migrator().ColumnTypes(tableName)
		if err != nil {
			return fmt.Errorf("get column types for %s: %w", tableName, err)
		}

		fields, imports, err := a.buildStructFields(ns, tableName, cols, finalConfig)
		if err != nil {
			return err
		}
		content := renderModelFile(outputPkg, tableName, structName, fields, imports)
		formatted, err := importsutil.Process(path, []byte(content), nil)
		if err != nil {
			return fmt.Errorf("format generated model %s: %w", path, err)
		}
		contentBytes := formatted

		if opts.DryRun {
			fmt.Fprintf(os.Stdout, "--- model preview (%s) ---\n%s\n--- end ---\n", path, string(contentBytes))
			continue
		}

		ok, err := utils.ConfirmWrite(path, writeFlags)
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
		if err := os.WriteFile(path, contentBytes, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Model written: %s\n", path)
	}
	return nil
}

func (a *DBAdapter) resolveTableConfigs() (map[string]TableConfig, error) {
	tables, err := a.db.Migrator().GetTables()
	if err != nil {
		return nil, err
	}
	configs := make(map[string]TableConfig)
	for _, table := range tables {
		cfg, include := buildConfigForTable(table, a.cfg.TableRules)
		if !include {
			continue
		}
		configs[table] = cfg
	}
	return configs, nil
}

var errStructNotFound = errors.New("struct not found")

func (a *DBAdapter) mergeModelChanges(path, table, structName string, cfg TableConfig, opts GenerateModelOptions) error {
	writeFlags := utils.ConfirmFlag(0)
	if opts.AutoApprove {
		writeFlags = utils.ConfirmAuto
	}
	fset := token.NewFileSet()
	original, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read existing model file %s: %w", path, err)
	}
	node, err := parser.ParseFile(fset, path, original, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse existing model file %s: %w", path, err)
	}

	// Find the struct declaration
	var structType *ast.StructType
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == structName {
			if st, ok := ts.Type.(*ast.StructType); ok {
				structType = st
				return false
			}
		}
		return true
	})

	if structType == nil {
		return fmt.Errorf("%w: %s", errStructNotFound, structName)
	}

	// Map existing fields by DB column name
	existingFields := make(map[string]bool)
	for _, field := range structType.Fields.List {
		if len(field.Names) > 0 {
			dbName := getDBName(field, a.db.NamingStrategy)
			if dbName != "-" {
				existingFields[dbName] = true
			}
		}
	}

	// Get current DB columns
	cols, err := a.db.Migrator().ColumnTypes(table)
	if err != nil {
		return fmt.Errorf("get column types for %s: %w", table, err)
	}

	// Find and generate new fields
	var newFields []*ast.Field
	newImports := make(map[string]struct{})
	for _, col := range cols {
		if _, ok := existingFields[col.Name()]; ok {
			continue
		}
		rule, hasRule := matchFieldRule(cfg.FieldRules, table, col.Name())
		if hasRule && rule.Exclude {
			continue
		}
		fieldName, goType, tagLiteral, imports, err := a.generateField(a.db.NamingStrategy, table, col, rule, hasRule)
		if err != nil {
			return err
		}
		for _, imp := range imports {
			newImports[imp] = struct{}{}
		}
		field := &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(fieldName)},
			Type:  ast.NewIdent(goType),
		}
		if tagLiteral != "" {
			field.Tag = &ast.BasicLit{Kind: token.STRING, Value: tagLiteral}
		}
		newFields = append(newFields, field)
	}

	if len(newFields) == 0 {
		fmt.Fprintf(os.Stdout, "Model %s is already up to date.\n", path)
		return nil
	}

	// Add new fields to the struct
	structType.Fields.List = append(structType.Fields.List, newFields...)

	// Add new imports
	for imp := range newImports {
		quoted := strconv.Quote(imp)
		var found bool
		for _, spec := range node.Imports {
			if spec.Path.Value == quoted {
				found = true
				break
			}
		}
		if !found {
			newImport := &ast.ImportSpec{Path: &ast.BasicLit{Kind: token.STRING, Value: quoted}}
			node.Imports = append(node.Imports, newImport)
		}
	}

	// Format the modified AST
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return fmt.Errorf("failed to format updated model code: %w", err)
	}

	utils.PrintDiff(path, original, buf.Bytes())
	if opts.DryRun {
		return nil
	}

	ok, err := utils.ConfirmWrite(path, writeFlags)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintf(os.Stdout, "Skipped %s\n", path)
		return nil
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Model updated: %s\n", path)
	return nil
}

func (a *DBAdapter) appendModelDefinition(ns schema.Namer, path, table, structName string, cfg TableConfig, opts GenerateModelOptions) error {
	writeFlags := utils.ConfirmFlag(0)
	if opts.AutoApprove {
		writeFlags = utils.ConfirmAuto
	}
	cols, err := a.db.Migrator().ColumnTypes(table)
	if err != nil {
		return fmt.Errorf("get column types for %s: %w", table, err)
	}

	fields, imports, err := a.buildStructFields(ns, table, cols, cfg)
	if err != nil {
		return err
	}

	fset := token.NewFileSet()
	original, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read existing model file %s: %w", path, err)
	}
	node, err := parser.ParseFile(fset, path, original, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse existing model file %s: %w", path, err)
	}
	for _, imp := range imports {
		if imp == "" {
			continue
		}
		astutil.AddImport(fset, node, imp)
	}
	var existing bytes.Buffer
	if err := format.Node(&existing, fset, node); err != nil {
		return fmt.Errorf("failed to format existing model code: %w", err)
	}

	snippet := renderModelStruct(structName, table, fields)
	trimmed := bytes.TrimRight(existing.Bytes(), "\n")
	combined := append(trimmed, []byte("\n\n"+snippet)...)
	formatted, err := importsutil.Process(path, combined, nil)
	if err != nil {
		return fmt.Errorf("format appended model %s: %w", path, err)
	}

	utils.PrintDiff(path, original, formatted)
	if opts.DryRun {
		return nil
	}
	ok, err := utils.ConfirmWrite(path, writeFlags)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintf(os.Stdout, "Skipped %s\n", path)
		return nil
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Model updated: %s\n", path)
	return nil
}

func getDBName(field *ast.Field, ns schema.Namer) string {
	if field.Tag == nil {
		return ns.TableName(field.Names[0].Name)
	}
	tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
	gormTag := tag.Get("gorm")
	settings := schema.ParseTagSetting(gormTag, ";")
	if _, disabled := settings["-"]; disabled {
		return "-"
	}
	if column, ok := settings["COLUMN"]; ok && column != "" {
		return column
	}
	return ns.TableName(field.Names[0].Name)
}

// GenerateMigration scaffolds a timestamped migration file.
func (a *DBAdapter) GenerateMigration(opts GenerateMigrationOptions) error {
	if opts.Name == "" {
		return errors.New("migration name is required")
	}
	writeFlags := utils.ConfirmFlag(0)
	if opts.AutoApprove {
		writeFlags = utils.ConfirmAuto
	}
	ts := time.Now().UTC().Format("20060102150405")
	slug := project.Slugify(opts.Name)
	filename := fmt.Sprintf("%s_%s.go", ts, slug)
	path := filepath.Join(a.cfg.MigrationsDir, filename)
	content := renderMigrationFile(strings.TrimSuffix(filename, ".go"))
	formatted, err := importsutil.Process(path, []byte(content), nil)
	if err != nil {
		return fmt.Errorf("format generated migration %s: %w", path, err)
	}
	contentBytes := formatted
	if opts.DryRun {
		fmt.Fprintf(os.Stdout, "--- model preview (%s) ---%c%s\n--- end ---\n", path, '\n', string(contentBytes))
		return nil
	}
	ok, err := utils.ConfirmWrite(path, writeFlags)
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
	if err := os.WriteFile(path, contentBytes, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "Migration created: %s\n", path)
	return nil
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

func (a *DBAdapter) buildStructFields(ns schema.Namer, table string, cols []gorm.ColumnType, cfg TableConfig) (string, []string, error) {
	imports := make(map[string]struct{})
	fieldStrings := make([]string, 0, len(cols))

	for _, col := range cols {
		colName := col.Name()
		rule, hasRule := matchFieldRule(cfg.FieldRules, table, colName)
		if hasRule && rule.Exclude {
			continue
		}
		fieldName, goType, tagLiteral, newImports, err := a.generateField(ns, table, col, rule, hasRule)
		if err != nil {
			return "", nil, err
		}
		for _, imp := range newImports {
			imports[imp] = struct{}{}
		}
		fieldDef := fmt.Sprintf("\t%s %s", fieldName, goType)
		if tagLiteral != "" {
			fieldDef += " " + tagLiteral
		}
		fieldStrings = append(fieldStrings, fieldDef)
	}

	importList := make([]string, 0, len(imports))
	for imp := range imports {
		importList = append(importList, imp)
	}

	return strings.Join(fieldStrings, "\n"), importList, nil
}

func (a *DBAdapter) generateField(ns schema.Namer, table string, col gorm.ColumnType, rule FieldRule, hasRule bool) (fieldName, goType, structTag string, imports []string, err error) {
	colName := col.Name()

	fieldName = ns.SchemaName(colName)
	if hasRule && rule.FieldName != "" {
		fieldName = rule.FieldName
	}

	if hasRule && rule.FieldType != "" {
		goType = rule.FieldType
		imports = rule.Imports
	} else {
		goType, imports = toGoType(col, a.db.Dialector.Name())
	}

	tags := make(map[string]string)
	if gormVal := buildGormTag(ns, col, fieldName); gormVal != "" {
		tags["gorm"] = gormVal
	}
	if hasRule && len(rule.Tags) > 0 {
		ctx := tagContext{Table: table, Name: fieldName, DBName: colName, DBType: col.DatabaseTypeName()}
		for key, tpl := range rule.Tags {
			val, tplErr := renderTagValue(tpl, ctx)
			if tplErr != nil {
				err = tplErr
				return
			}
			if strings.TrimSpace(val) == "" {
				delete(tags, key)
				continue
			}
			tags[key] = val
		}
	}
	structTag = buildStructTag(tags)
	return
}

func toGoType(col gorm.ColumnType, dialect string) (string, []string) {
	_ = dialect
	isNullable, _ := col.Nullable()
	if pk, _ := col.PrimaryKey(); pk {
		isNullable = false
	}
	scanType := col.ScanType()
	if scanType == nil {
		return "any", nil
	}
	if isSQLNullType(scanType) {
		if !isNullable {
			if base, imports, ok := sqlNullBaseType(scanType.Name()); ok {
				return base, imports
			}
		}
		goType, imports := reflectTypeString(scanType)
		return goType, imports
	}
	baseType := scanType
	if baseType.Kind() == reflect.Pointer {
		baseType = baseType.Elem()
	}
	goType, imports := reflectTypeString(baseType)
	if isNullable {
		if baseType.Kind() == reflect.Slice || implementsScannerOrValuer(baseType) {
			return goType, imports
		}
		return "*" + goType, imports
	}
	return goType, imports
}

func reflectTypeString(rt reflect.Type) (string, []string) {
	switch rt.Kind() {
	case reflect.Pointer:
		inner, imports := reflectTypeString(rt.Elem())
		return "*" + inner, imports
	case reflect.Slice:
		inner, imports := reflectTypeString(rt.Elem())
		if inner == "uint8" {
			return "[]byte", imports
		}
		return "[]" + inner, imports
	default:
		pkg := rt.PkgPath()
		name := rt.String()
		if pkg == "" {
			return name, nil
		}
		return name, []string{pkg}
	}
}

func isSQLNullType(rt reflect.Type) bool {
	return rt.PkgPath() == "database/sql" && strings.HasPrefix(rt.Name(), "Null")
}

func sqlNullBaseType(name string) (string, []string, bool) {
	switch name {
	case "NullString":
		return "string", nil, true
	case "NullBool":
		return "bool", nil, true
	case "NullInt16":
		return "int16", nil, true
	case "NullInt32":
		return "int32", nil, true
	case "NullInt64":
		return "int64", nil, true
	case "NullFloat64":
		return "float64", nil, true
	case "NullTime":
		return "time.Time", []string{"time"}, true
	default:
		return "", nil, false
	}
}

var (
	scannerIface = reflect.TypeOf((*sql.Scanner)(nil)).Elem()
	valuerIface  = reflect.TypeOf((*driver.Valuer)(nil)).Elem()
)

func implementsScannerOrValuer(rt reflect.Type) bool {
	if rt.Implements(scannerIface) || rt.Implements(valuerIface) {
		return true
	}
	if rt.Kind() != reflect.Ptr {
		ptr := reflect.PointerTo(rt)
		return ptr.Implements(scannerIface) || ptr.Implements(valuerIface)
	}
	return false
}

func buildGormTag(ns schema.Namer, col gorm.ColumnType, fieldName string) string {
	tags := []string{}
	if pk, _ := col.PrimaryKey(); pk {
		tags = append(tags, "primaryKey")
	}

	if col.Name() != ns.TableName(fieldName) {
		tags = append(tags, "column:"+col.Name())
	}

	if val, ok := col.DefaultValue(); ok {
		tags = append(tags, "default:"+val)
	}

	if len(tags) == 0 {
		return ""
	}
	return strings.Join(tags, ";")
}

func buildStructTag(tags map[string]string) string {
	if len(tags) == 0 {
		return ""
	}
	keys := make([]string, 0, len(tags))
	for k, v := range tags {
		if strings.TrimSpace(v) == "" {
			continue
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s:%s", k, strconv.Quote(tags[k])))
	}
	return "`" + strings.Join(parts, " ") + "`"
}
