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
	"text/template"
	"time"

	"golang.org/x/exp/maps"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
	importsutil "golang.org/x/tools/imports"
	"gorm.io/cli/gorm/internal/migration/diff"
	"gorm.io/cli/gorm/internal/migration/schema"
	"gorm.io/cli/gorm/internal/migration/source"
	"gorm.io/cli/gorm/internal/project"
	"gorm.io/cli/gorm/internal/utils"
	"gorm.io/gorm"
	gormSchema "gorm.io/gorm/schema"
)

// DBAdapter implements Adapter using a gorm.DB connection.
type DBAdapter struct {
	db           *gorm.DB
	cfg          Config
	migrations   map[string]Migration
	sourceParser *source.Parser
	differ       *diff.Differ
}

// NewDBAdapter wires a DBAdapter for the provided DB connection.
func NewDBAdapter(db *gorm.DB, cfg Config) (*DBAdapter, error) {
	if db == nil {
		return nil, errors.New("migration runtime: db is required")
	}
	return &DBAdapter{
			db:           db,
			cfg:          cfg,
			migrations:   make(map[string]Migration),
			sourceParser: source.New(),
			differ:       diff.New(),
		},
		nil
}

// RegisterMigrations registers a list of migrations for execution.
func (a *DBAdapter) RegisterMigrations(migs []Migration) {
	for _, m := range migs {
		if m.Name == "" {
			panic("migration runtime: migration must have a name")
		}
		a.migrations[m.Name] = m
	}
}

// Up applies pending migrations up to the limit.
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

// Down rolls back applied migrations.
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

// Status prints the status of all migrations.
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

// Diff compares the database schema with the models and prints the differences.
func (a *DBAdapter) Diff(opts DiffOptions) error {
	dbState, err := a.GetDBSchemas()
	if err != nil {
		return fmt.Errorf("could not get database schema: %w", err)
	}
	sourceState, err := a.sourceParser.GetSchemasFromModels(a.db, a.cfg.DiffModels)
	if err != nil {
		return fmt.Errorf("could not parse models from source: %w", err)
	}
	diffResult := a.differ.Compare(sourceState, dbState)
	writer := opts.Writer
	if writer == nil {
		writer = os.Stdout
	}
	if diffResult.Empty() {
		fmt.Fprintln(writer, "Models match the database schema")
		return nil
	}
	writeSchemaDiff(writer, diffResult)
	return nil
}

// GetDBSchemas retrieves the schema definition from the database.
func (a *DBAdapter) GetDBSchemas() ([]*schema.Table, error) {
	tables, err := a.db.Migrator().GetTables()
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	result := make([]*schema.Table, 0, len(tables))
	ns := a.db.NamingStrategy
	for _, tableName := range tables {
		if tableName == (schemaMigration{}).TableName() {
			continue
		}
		if _, include := buildConfigForTable(tableName, a.cfg.TableRules); !include {
			continue
		}
		columns, err := a.db.Migrator().ColumnTypes(tableName)
		if err != nil {
			return nil, fmt.Errorf("describe table %s: %w", tableName, err)
		}
		table := &schema.Table{
			Name:   tableName,
			Fields: make([]*schema.Field, len(columns)),
		}
		for i, column := range columns {
			table.Fields[i] = columnTypeToField(column, ns)
		}
		indexes, err := a.db.Migrator().GetIndexes(tableName)
		if err != nil {
			return nil, fmt.Errorf("indexes for %s: %w", tableName, err)
		}
		table.Indexes = gormIndexesToUniversalIndexes(indexes)
		result = append(result, table)
	}
	return result, nil
}

// GenerateModel reverse-engineers the database schema to Go structs.
func (a *DBAdapter) GenerateModel(opts GenerateModelOptions) error {
	ns := a.db.NamingStrategy
	modelDir := a.cfg.ModelsDir
	configs, err := a.resolveTableConfigs()
	if err != nil {
		return err
	}
	if len(configs) == 0 {
		fmt.Fprintln(os.Stdout, "No tables found in database")
		return nil
	}
	for tableName := range configs {
		finalConfig := configs[tableName]
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
		outputPkg := project.DetectPackage(filepath.Dir(path))
		originalContent, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to read existing model file %s: %w", path, err)
		}
		if len(originalContent) > 0 {
			err := a.mergeModelChanges(path, tableName, structName, finalConfig, originalContent, opts)
			if err == nil {
				continue
			}
			if errors.Is(err, errStructNotFound) {
				appendErr := a.appendModelDefinition(ns, path, tableName, structName, finalConfig, originalContent, opts)
				if appendErr == nil {
					continue
				}
				fmt.Fprintf(os.Stderr, "Could not append model %s to %s, falling back to overwrite: %v\n", structName, path, appendErr)
			} else {
				fmt.Fprintf(os.Stderr, "Could not merge changes for %s, falling back to overwrite: %v\n", path, err)
			}
		}

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
		if err := utils.WriteFileWithConfirmation(path, originalContent, formatted, opts.DryRun, opts.AutoApprove, false); err != nil {
			return err
		}
	}
	return nil
}

// GenerateMigration scaffolds a new migration file.
func (a *DBAdapter) GenerateMigration(opts GenerateMigrationOptions) error {
	if opts.Name == "" {
		return errors.New("migration name is required")
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
	return utils.WriteFileWithConfirmation(path, nil, formatted, opts.DryRun, opts.AutoApprove, false)
}

// --- Private state management methods ---

type schemaMigration struct {
	Name      string    `gorm:"primaryKey;size:200"`
	AppliedAt time.Time `gorm:"autoUpdateTime:false"`
}

func (schemaMigration) TableName() string { return "schema_migrations" }

func (a *DBAdapter) ensureSchemaTable() error {
	return a.db.AutoMigrate(&schemaMigration{})
}

func (a *DBAdapter) recordApplied(tx *gorm.DB, name string) error {
	if len(name) > 150 {
		return fmt.Errorf("migration name exceeds 150 characters: %s", name)
	}
	if tx == nil {
		tx = a.db
	}
	return tx.Create(&schemaMigration{Name: name, AppliedAt: time.Now().UTC()}).Error
}

func (a *DBAdapter) removeApplied(tx *gorm.DB, name string) error {
	if tx == nil {
		tx = a.db
	}
	return tx.Delete(&schemaMigration{Name: name}).Error
}

func (a *DBAdapter) appliedMigrationsAsc() ([]schemaMigration, error) {
	var records []schemaMigration
	if err := a.db.Order("name asc").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (a *DBAdapter) appliedMigrationsDesc() ([]schemaMigration, error) {
	var records []schemaMigration
	if err := a.db.Order("applied_at desc").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (a *DBAdapter) pendingMigrations() ([]Migration, error) {
	applied, err := a.appliedMigrationsAsc()
	if err != nil {
		return nil, err
	}
	appliedSet := make(map[string]struct{}, len(applied))
	for _, record := range applied {
		appliedSet[record.Name] = struct{}{}
	}
	regs := a.registeredMigrations()
	pending := make([]Migration, 0)
	for _, mig := range regs {
		if _, ok := appliedSet[mig.Name]; !ok {
			pending = append(pending, mig)
		}
	}
	return pending, nil
}

func (a *DBAdapter) registeredMigrations() []Migration {
	if len(a.migrations) == 0 {
		return nil
	}
	out := make([]Migration, 0, len(a.migrations))
	for _, m := range a.migrations {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (a *DBAdapter) migrationByName(name string) (Migration, bool) {
	m, ok := a.migrations[name]
	return m, ok
}

// --- Private helper methods ---

var errStructNotFound = errors.New("struct not found")

func (a *DBAdapter) mergeModelChanges(path, table, structName string, cfg TableConfig, original []byte, opts GenerateModelOptions) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, original, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse existing model file %s: %w", path, err)
	}
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
	existingFields := make(map[string]bool)
	for _, field := range structType.Fields.List {
		if len(field.Names) > 0 {
			dbName := getDBName(field, a.db.NamingStrategy)
			if dbName != "-" {
				existingFields[dbName] = true
			}
		}
	}
	cols, err := a.db.Migrator().ColumnTypes(table)
	if err != nil {
		return fmt.Errorf("get column types for %s: %w", table, err)
	}
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
	structType.Fields.List = append(structType.Fields.List, newFields...)
	for imp := range newImports {
		astutil.AddImport(fset, node, imp)
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return fmt.Errorf("failed to format updated model code: %w", err)
	}
	final, err := importsutil.Process(path, buf.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("format merged model %s: %w", path, err)
	}
	return utils.WriteFileWithConfirmation(path, original, final, opts.DryRun, opts.AutoApprove, false)
}

func (a *DBAdapter) appendModelDefinition(ns gormSchema.Namer, path, table, structName string, cfg TableConfig, original []byte, opts GenerateModelOptions) error {
	cols, err := a.db.Migrator().ColumnTypes(table)
	if err != nil {
		return fmt.Errorf("get column types for %s: %w", table, err)
	}
	fields, imports, err := a.buildStructFields(ns, table, cols, cfg)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
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
	return utils.WriteFileWithConfirmation(path, original, formatted, opts.DryRun, opts.AutoApprove, false)
}

func (a *DBAdapter) buildStructFields(ns gormSchema.Namer, table string, cols []gorm.ColumnType, cfg TableConfig) (string, []string, error) {
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

func (a *DBAdapter) generateField(ns gormSchema.Namer, table string, col gorm.ColumnType, rule FieldRule, hasRule bool) (fieldName, goType, structTag string, imports []string, err error) {
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

func columnTypeToField(col gorm.ColumnType, ns gormSchema.Namer) *schema.Field {
	dataType := col.DatabaseTypeName()
	size, _ := col.Length()
	precision, scale, _ := col.DecimalSize()
	switch strings.ToUpper(dataType) {
	case "BIGINT", "INT", "INTEGER", "SMALLINT", "TINYINT", "MEDIUMINT", "FLOAT", "DOUBLE":
		precision = 0
		scale = 0
	}
	var defValPtr *string
	if def, ok := col.DefaultValue(); ok {
		defValPtr = &def
	}
	pk, _ := col.PrimaryKey()
	autoInc, _ := col.AutoIncrement()
	nullable, _ := col.Nullable()

	unique, _ := col.Unique()
	if pk {
		unique = false
	}
	return &schema.Field{
		DBName:        col.Name(),
		DataType:      dataType,
		IsPrimaryKey:  pk,
		IsNullable:    nullable,
		IsUnique:      unique,
		Size:          int(size),
		Precision:     int(precision),
		Scale:         int(scale),
		DefaultValue:  defValPtr,
		AutoIncrement: autoInc,
		Comment:       "",
	}
}

func gormIndexesToUniversalIndexes(indexes []gorm.Index) []*schema.Index {
	result := make([]*schema.Index, 0, len(indexes))
	for _, idx := range indexes {
		if pk, ok := idx.PrimaryKey(); ok && pk {
			continue
		}

		unique, _ := idx.Unique()
		result = append(result, &schema.Index{
			Name:     idx.Name(),
			Columns:  idx.Columns(),
			IsUnique: unique,
			Option:   idx.Option(),
		})
	}
	return result
}

func getDBName(field *ast.Field, ns gormSchema.Namer) string {
	if field.Tag == nil {
		return ns.TableName(field.Names[0].Name)
	}
	tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
	gormTag := tag.Get("gorm")
	settings := gormSchema.ParseTagSetting(gormTag, ";")
	if _, disabled := settings["-"]; disabled {
		return "-"
	}
	if column, ok := settings["COLUMN"]; ok && column != "" {
		return column
	}
	return ns.TableName(field.Names[0].Name)
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

func buildGormTag(ns gormSchema.Namer, col gorm.ColumnType, fieldName string) string {
	tags := []string{}
	if pk, _ := col.PrimaryKey(); pk {
		tags = append(tags, "primaryKey")
	}
	if col.Name() != ns.TableName(fieldName) {
		tags = append(tags, "column:"+col.Name())
	}

	dbType := strings.ToUpper(col.DatabaseTypeName())
	if dbType == "JSON" {
		tags = append(tags, "type:json")
	}
	if dbType == "BLOB" {
		tags = append(tags, "type:blob")
	}
	if dbType == "TEXT" {
		tags = append(tags, "type:text")
	}
	if dbType == "DECIMAL" {
		tags = append(tags, "type:decimal")
	}

	if size, ok := col.Length(); ok && size > 0 {
		tags = append(tags, fmt.Sprintf("size:%d", size))
	}

	if precision, scale, ok := col.DecimalSize(); ok && precision > 0 {
		switch dbType {
		case "DECIMAL", "NUMERIC":
			tags = append(tags, fmt.Sprintf("precision:%d", precision))
			if scale > 0 {
				tags = append(tags, fmt.Sprintf("scale:%d", scale))
			}
		}
	}

	if nullable, ok := col.Nullable(); ok && !nullable {
		tags = append(tags, "not null")
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

func writeSchemaDiff(w io.Writer, result diff.Result) {
	diff.WriteSchemaDiff(w, result)
}

func buildConfigForTable(table string, rules []TableRule) (TableConfig, bool) {
	var finalCfg TableConfig
	include := true
	foundRule := false
	for _, rule := range rules {
		if rule.Pattern == "" {
			continue
		}
		ok, err := filepath.Match(rule.Pattern, table)
		if err != nil || !ok {
			continue
		}
		foundRule = true
		if rule.Exclude {
			include = false
			break
		}
		if len(rule.Config.FieldRules) > 0 {
			finalCfg.FieldRules = append(finalCfg.FieldRules, cloneFieldRules(rule.Config.FieldRules)...)
		}
		if rule.Config.OutputPath != "" {
			finalCfg.OutputPath = rule.Config.OutputPath
		}
	}
	if !foundRule {
		return TableConfig{}, true
	}
	return finalCfg, include
}

func cloneFieldRules(src []FieldRule) []FieldRule {
	dup := make([]FieldRule, len(src))
	for i, v := range src {
		dup[i] = FieldRule{
			Pattern:   v.Pattern,
			FieldName: v.FieldName,
			FieldType: v.FieldType,
			Tags:      maps.Clone(v.Tags),
			Imports:   append([]string(nil), v.Imports...),
			Exclude:   v.Exclude,
		}
	}
	return dup
}

func matchFieldRule(rules []FieldRule, table, column string) (FieldRule, bool) {
	full := table + "." + column
	for _, rule := range rules {
		pattern := strings.TrimSpace(rule.Pattern)
		if pattern == "" {
			pattern = full
		}
		if pattern == full || pattern == column {
			return rule, true
		}
		if matched, _ := filepath.Match(pattern, full); matched {
			return rule, true
		}
		if matched, _ := filepath.Match(pattern, column); matched {
			return rule, true
		}
	}
	return FieldRule{}, false
}

func renderModelFile(pkg, table, structName, fields string, imports []string) string {
	var buf bytes.Buffer
	sort.Strings(imports)
	structSrc := renderModelStruct(structName, table, fields)
	_ = modelFileTemplate.Execute(&buf, struct {
		Pkg     string
		Struct  string
		Imports []string
	}{Pkg: pkg, Struct: structSrc, Imports: imports})
	formatted, err := format.Source(buf.Bytes())
	if err == nil {
		return string(formatted)
	}
	return buf.String()
}

func renderModelStruct(structName, table, fields string) string {
	var buf bytes.Buffer
	_ = modelStructTemplate.Execute(&buf, struct {
		StructName string
		Table      string
		Fields     string
	}{StructName: structName, Table: table, Fields: fields})
	return buf.String()
}

func renderMigrationFile(name string) string {
	var buf bytes.Buffer
	_ = migrationFileTemplate.Execute(&buf, struct{ Name string }{Name: name})
	return buf.String()
}

var migrationFileTemplate = template.Must(template.New("migration").Parse(`package main

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

var modelFileTemplate = template.Must(template.New("model").Parse(`// Code generated by gorm migrate reflect

package {{.Pkg}}
{{if .Imports}}
import (
{{- range .Imports }}
	"{{.}}"
{{- end }}
)
{{end}}

{{.Struct}}
`))

var modelStructTemplate = template.Must(template.New("modelStruct").Parse(`type {{.StructName}} struct {
{{.Fields}}
}

func ({{.StructName}}) TableName() string {
	return "{{.Table}}"
}
`))

type tagContext struct {
	Table  string
	Name   string
	DBName string
	DBType string
}

func renderTagValue(tpl string, ctx tagContext) (string, error) {
	if !strings.Contains(tpl, "{{") {
		return tpl, nil
	}
	t, err := template.New("tag").Parse(tpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

const diffHelperFile = "gorm_generated_diff_models_helper.go"

func GenerateDiffFile(modelsDir, migrationsDir string) (string, error) {
	structs, err := collectModelStructs(modelsDir)
	if err != nil {
		return "", err
	}
	data := buildDiffTemplateData(structs)
	source, err := renderDiffFile(data)
	if err != nil {
		return "", err
	}
	migrationsDir = project.ResolveRootPath(migrationsDir)
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return "", fmt.Errorf("prepare migrations dir: %w", err)
	}
	path := filepath.Join(migrationsDir, diffHelperFile)
	return path, os.WriteFile(path, []byte(source), 0644)
}

func collectModelStructs(modelsDir string) ([]modelStruct, error) {
	root := project.Root()
	if root == "" {
		return nil, fmt.Errorf("unable to determine project root")
	}
	dir := project.ResolveRootPath(modelsDir)
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat models dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("models directory must be a folder: %s", dir)
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return nil, fmt.Errorf("models directory must reside within the module root: %w", err)
	}
	rel = filepath.ToSlash(rel)
	pattern := "./" + rel
	if rel == "." {
		pattern = "./..."
	} else {
		pattern += "/..."
	}
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedSyntax | packages.NeedFiles, Dir: root}
	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		return nil, fmt.Errorf("load model packages: %w", err)
	}
	var structs []modelStruct
	seen := make(map[string]struct{})
	for _, pkg := range pkgs {
		if pkg == nil || len(pkg.Syntax) == 0 {
			continue
		}
		if strings.HasSuffix(pkg.PkgPath, ".test") {
			continue
		}
		if len(pkg.Errors) > 0 {
			return nil, fmt.Errorf("load package %s: %v", pkg.PkgPath, pkg.Errors[0])
		}
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.TYPE {
					continue
				}
				for _, spec := range gen.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok || !ts.Name.IsExported() {
						continue
					}
					if _, ok := ts.Type.(*ast.StructType); !ok {
						continue
					}
					name := ts.Name.Name
					key := pkg.PkgPath + "." + name
					if _, ok := seen[key]; ok {
						continue
					}
					seen[key] = struct{}{}
					structs = append(structs, modelStruct{PackagePath: pkg.PkgPath, TypeName: name})
				}
			}
		}
	}
	sort.Slice(structs, func(i, j int) bool {
		if structs[i].PackagePath == structs[j].PackagePath {
			return structs[i].TypeName < structs[j].TypeName
		}
		return structs[i].PackagePath < structs[j].PackagePath
	})
	return structs, nil
}

type modelStruct struct {
	PackagePath string
	TypeName    string
}

type diffImport struct {
	Alias string
	Path  string
}

type diffTarget struct {
	Alias    string
	TypeName string
}

type diffTemplateData struct {
	Imports []diffImport
	Targets []diffTarget
}

func buildDiffTemplateData(structs []modelStruct) diffTemplateData {
	imports := make([]diffImport, 0, len(structs))
	targets := make([]diffTarget, 0, len(structs))
	aliasMap := make(map[string]string)
	for _, st := range structs {
		alias, ok := aliasMap[st.PackagePath]
		if !ok {
			alias = fmt.Sprintf("pkg%d", len(aliasMap))
			aliasMap[st.PackagePath] = alias
			imports = append(imports, diffImport{Alias: alias, Path: st.PackagePath})
		}
		targets = append(targets, diffTarget{Alias: alias, TypeName: st.TypeName})
	}
	return diffTemplateData{Imports: imports, Targets: targets}
}

func renderDiffFile(data diffTemplateData) (string, error) {
	var buf bytes.Buffer
	if err := diffFileTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render diff helper: %w", err)
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return "", fmt.Errorf("format diff helper: %w", err)
	}
	return string(formatted), nil
}

var diffFileTemplate = template.Must(template.New("diff-helper").Parse(`// Code generated by gorm migrate diff; DO NOT EDIT.
package main

import (
	"gorm.io/cli/gorm/migration"
	{{- range .Imports }}
	{{ .Alias }} "{{ .Path }}"
	{{- end }}
)

func init() {
	{{- if .Targets }}
	migration.RegisterDiffModels(
		{{- range .Targets }}
		&{{ .Alias }}.{{ .TypeName }}{},
		{{- end }}
	)
	{{- else }}
	migration.RegisterDiffModels()
	{{- end }}
}
`))
