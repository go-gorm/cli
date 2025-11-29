package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"golang.org/x/tools/go/packages"
	"gorm.io/cli/gorm/internal/project"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// ModelRef identifies a Go struct that maps to a database table.
type ModelRef struct {
	PackagePath string
	TypeName    string
}

// TableSchema contains a parsed schema plus auxiliary metadata used for diffs.
type TableSchema struct {
	Schema      *schema.Schema
	Indexes     []*schema.Index
	Constraints []*schema.Constraint
	Model       *ModelRef
}

// ModelSnapshot mirrors the JSON emitted by the helper when collecting model metadata.
type ModelSnapshot struct {
	PackagePath string        `json:"pkg"`
	TypeName    string        `json:"type"`
	Table       TableSnapshot `json:"table"`
}

type TableSnapshot struct {
	Name        string               `json:"name"`
	Fields      []FieldSnapshot      `json:"fields"`
	Indexes     []IndexSnapshot      `json:"indexes"`
	Constraints []ConstraintSnapshot `json:"constraints"`
}

type FieldSnapshot struct {
	Name                   string            `json:"name"`
	DBName                 string            `json:"dbName"`
	DataType               string            `json:"dataType"`
	GORMDataType           string            `json:"gormDataType"`
	PrimaryKey             bool              `json:"primaryKey"`
	AutoIncrement          bool              `json:"autoIncrement"`
	AutoIncrementIncrement int64             `json:"autoIncrementIncrement"`
	NotNull                bool              `json:"notNull"`
	Unique                 bool              `json:"unique"`
	Size                   int               `json:"size"`
	Precision              int               `json:"precision"`
	Scale                  int               `json:"scale"`
	HasDefaultValue        bool              `json:"hasDefaultValue"`
	DefaultValue           string            `json:"defaultValue"`
	Comment                string            `json:"comment"`
	GormTag                string            `json:"gormTag"`
	TagSettings            map[string]string `json:"tagSettings"`
}

type IndexSnapshot struct {
	Name    string               `json:"name"`
	Class   string               `json:"class"`
	Type    string               `json:"type"`
	Where   string               `json:"where"`
	Comment string               `json:"comment"`
	Option  string               `json:"option"`
	Fields  []IndexFieldSnapshot `json:"fields"`
}

type IndexFieldSnapshot struct {
	Column     string `json:"column"`
	Expression string `json:"expression"`
	Sort       string `json:"sort"`
	Collate    string `json:"collate"`
	Length     int    `json:"length"`
	Priority   int    `json:"priority"`
}

type ConstraintSnapshot struct {
	Name             string   `json:"name"`
	Type             string   `json:"type"`
	Columns          []string `json:"columns"`
	ReferenceTable   string   `json:"ref_table"`
	ReferenceColumns []string `json:"ref_columns"`
	OnUpdate         string   `json:"on_update"`
	OnDelete         string   `json:"on_delete"`
	Expression       string   `json:"expression"`
}

const diffHelperFile = "gorm_generated_diff_models_helper.go"

func (a *DBAdapter) renderDiffModels(w io.Writer) error {
	snaps, err := a.captureModelSnapshots()
	if err != nil {
		return err
	}
	if w == nil {
		w = os.Stdout
	}
	if snaps == nil {
		snaps = []ModelSnapshot{}
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(snaps)
}

func (a *DBAdapter) collectModelSchemas() (map[string]*TableSchema, error) {
	snaps, err := a.captureModelSnapshots()
	if err != nil {
		return nil, err
	}
	if len(snaps) == 0 {
		return map[string]*TableSchema{}, nil
	}
	return parseHelperSnapshots(snaps)
}

func (a *DBAdapter) captureModelSnapshots() ([]ModelSnapshot, error) {
	if len(a.cfg.DiffModels) == 0 {
		return nil, nil
	}

	snaps := make([]ModelSnapshot, 0, len(a.cfg.DiffModels))
	seen := make(map[string]struct{})
	for _, mdl := range a.cfg.DiffModels {
		snap, err := buildSnapshot(a.db, mdl)
		if err != nil {
			return nil, err
		}
		key := snap.PackagePath + "." + snap.TypeName
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		snaps = append(snaps, snap)
	}

	sort.Slice(snaps, func(i, j int) bool {
		if snaps[i].Table.Name == snaps[j].Table.Name {
			return snaps[i].TypeName < snaps[j].TypeName
		}
		return snaps[i].Table.Name < snaps[j].Table.Name
	})

	return snaps, nil
}

func buildSnapshot(db *gorm.DB, model interface{}) (ModelSnapshot, error) {
	if model == nil {
		return ModelSnapshot{}, fmt.Errorf("migration runtime: nil diff model")
	}
	typ := reflect.TypeOf(model)
	if typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Struct {
		return ModelSnapshot{}, fmt.Errorf("migration runtime: diff model must be pointer to struct (got %T)", model)
	}
	value := reflect.New(typ.Elem()).Interface()
	stmt := &gorm.Statement{DB: db.Session(&gorm.Session{})}
	if err := stmt.Parse(value); err != nil {
		return ModelSnapshot{}, err
	}
	if stmt.Schema == nil {
		return ModelSnapshot{}, fmt.Errorf("migration runtime: parsed schema is nil for %T", model)
	}
	return ModelSnapshot{
		PackagePath: typ.Elem().PkgPath(),
		TypeName:    typ.Elem().Name(),
		Table:       encodeSchemaTable(stmt.Schema),
	}, nil
}

func parseHelperSnapshots(snaps []ModelSnapshot) (map[string]*TableSchema, error) {
	result := make(map[string]*TableSchema, len(snaps))
	for _, snap := range snaps {
		tableSchema, err := snap.toTableSchema()
		if err != nil {
			return nil, err
		}
		result[strings.ToLower(tableSchema.Schema.Table)] = tableSchema
	}
	return result, nil
}

func (snap ModelSnapshot) toTableSchema() (*TableSchema, error) {
	sch := &schema.Schema{
		Name:           snap.TypeName,
		Table:          snap.Table.Name,
		Fields:         make([]*schema.Field, 0, len(snap.Table.Fields)),
		FieldsByName:   make(map[string]*schema.Field, len(snap.Table.Fields)),
		FieldsByDBName: make(map[string]*schema.Field, len(snap.Table.Fields)),
		DBNames:        make([]string, 0, len(snap.Table.Fields)),
	}

	for i := range snap.Table.Fields {
		field := snap.Table.Fields[i].toSchemaField(sch)
		sch.Fields = append(sch.Fields, field)
		sch.FieldsByName[field.Name] = field
		sch.FieldsByDBName[field.DBName] = field
		sch.DBNames = append(sch.DBNames, field.DBName)
		if field.PrimaryKey {
			sch.PrimaryFields = append(sch.PrimaryFields, field)
			sch.PrimaryFieldDBNames = append(sch.PrimaryFieldDBNames, field.DBName)
			if sch.PrioritizedPrimaryField == nil {
				sch.PrioritizedPrimaryField = field
			}
		}
	}

	indexes := make([]*schema.Index, 0, len(snap.Table.Indexes))
	for _, idx := range snap.Table.Indexes {
		indexes = append(indexes, idx.toSchemaIndex(sch))
	}
	constraints := make([]*schema.Constraint, 0, len(snap.Table.Constraints))
	for _, cons := range snap.Table.Constraints {
		constraints = append(constraints, cons.toSchemaConstraint(sch))
	}

	model := &ModelRef{PackagePath: snap.PackagePath, TypeName: snap.TypeName}
	return &TableSchema{Schema: sch, Indexes: indexes, Constraints: constraints, Model: model}, nil
}

func (snap FieldSnapshot) toSchemaField(parent *schema.Schema) *schema.Field {
	field := &schema.Field{
		Name:                   snap.Name,
		DBName:                 snap.DBName,
		Schema:                 parent,
		DataType:               schema.DataType(strings.ToLower(snap.DataType)),
		GORMDataType:           schema.DataType(strings.ToLower(snap.GORMDataType)),
		PrimaryKey:             snap.PrimaryKey,
		AutoIncrement:          snap.AutoIncrement,
		AutoIncrementIncrement: snap.AutoIncrementIncrement,
		NotNull:                snap.NotNull,
		Unique:                 snap.Unique,
		Size:                   snap.Size,
		Precision:              snap.Precision,
		Scale:                  snap.Scale,
		HasDefaultValue:        snap.HasDefaultValue,
		DefaultValue:           snap.DefaultValue,
		Comment:                snap.Comment,
		TagSettings:            make(map[string]string, len(snap.TagSettings)),
	}
	for k, v := range snap.TagSettings {
		field.TagSettings[k] = v
	}
	return field
}

func (snap IndexSnapshot) toSchemaIndex(parent *schema.Schema) *schema.Index {
	idx := &schema.Index{
		Name:    snap.Name,
		Class:   snap.Class,
		Type:    snap.Type,
		Where:   snap.Where,
		Comment: snap.Comment,
		Option:  snap.Option,
	}
	sort.SliceStable(snap.Fields, func(i, j int) bool {
		if snap.Fields[i].Priority == snap.Fields[j].Priority {
			return snap.Fields[i].Column < snap.Fields[j].Column
		}
		return snap.Fields[i].Priority < snap.Fields[j].Priority
	})
	for _, fieldSnap := range snap.Fields {
		field := parent.FieldsByDBName[fieldSnap.Column]
		if field == nil {
			continue
		}
		idx.Fields = append(idx.Fields, schema.IndexOption{
			Field:      field,
			Expression: fieldSnap.Expression,
			Sort:       fieldSnap.Sort,
			Collate:    fieldSnap.Collate,
			Length:     fieldSnap.Length,
			Priority:   fieldSnap.Priority,
		})
	}
	return idx
}

func (snap ConstraintSnapshot) toSchemaConstraint(parent *schema.Schema) *schema.Constraint {
	return &schema.Constraint{Name: snap.Name, Schema: parent}
}

func encodeSchemaTable(sch *schema.Schema) TableSnapshot {
	fields := make([]FieldSnapshot, 0, len(sch.Fields))
	for _, field := range sch.Fields {
		if field.IgnoreMigration {
			continue
		}
		fields = append(fields, encodeField(field))
	}
	indexes := sch.ParseIndexes()
	indexSnaps := make([]IndexSnapshot, 0, len(indexes))
	for _, idx := range indexes {
		indexSnaps = append(indexSnaps, encodeIndex(idx))
	}
	constraints := encodeConstraints(sch)
	return TableSnapshot{
		Name:        sch.Table,
		Fields:      fields,
		Indexes:     indexSnaps,
		Constraints: constraints,
	}
}

func encodeField(field *schema.Field) FieldSnapshot {
	tagSettings := make(map[string]string, len(field.TagSettings))
	for k, v := range field.TagSettings {
		tagSettings[k] = v
	}
	return FieldSnapshot{
		Name:                   field.Name,
		DBName:                 field.DBName,
		DataType:               string(field.DataType),
		GORMDataType:           string(field.GORMDataType),
		PrimaryKey:             field.PrimaryKey,
		AutoIncrement:          field.AutoIncrement,
		AutoIncrementIncrement: field.AutoIncrementIncrement,
		NotNull:                field.NotNull,
		Unique:                 field.Unique,
		Size:                   field.Size,
		Precision:              field.Precision,
		Scale:                  field.Scale,
		HasDefaultValue:        field.HasDefaultValue,
		DefaultValue:           field.DefaultValue,
		Comment:                field.Comment,
		GormTag:                string(field.Tag),
		TagSettings:            tagSettings,
	}
}

func encodeIndex(idx *schema.Index) IndexSnapshot {
	fields := make([]IndexFieldSnapshot, 0, len(idx.Fields))
	for _, field := range idx.Fields {
		column := ""
		if field.Field != nil {
			column = field.Field.DBName
		}
		fields = append(fields, IndexFieldSnapshot{
			Column:     column,
			Expression: field.Expression,
			Sort:       field.Sort,
			Collate:    field.Collate,
			Length:     field.Length,
			Priority:   field.Priority,
		})
	}
	return IndexSnapshot{
		Name:    idx.Name,
		Class:   idx.Class,
		Type:    idx.Type,
		Where:   idx.Where,
		Comment: idx.Comment,
		Option:  idx.Option,
		Fields:  fields,
	}
}

func encodeConstraints(sch *schema.Schema) []ConstraintSnapshot {
	seen := make(map[string]struct{})
	constraints := make([]ConstraintSnapshot, 0)

	for name, chk := range sch.ParseCheckConstraints() {
		constraints = append(constraints, ConstraintSnapshot{
			Name:       name,
			Type:       "CHECK",
			Columns:    []string{chk.Field.DBName},
			Expression: chk.Constraint,
		})
		seen[name] = struct{}{}
	}

	for name, uni := range sch.ParseUniqueConstraints() {
		if _, ok := seen[name]; ok {
			continue
		}
		column := ""
		if uni.Field != nil {
			column = uni.Field.DBName
		}
		constraints = append(constraints, ConstraintSnapshot{
			Name:    name,
			Type:    "UNIQUE",
			Columns: []string{column},
		})
		seen[name] = struct{}{}
	}

	for _, rel := range sch.Relationships.Relations {
		if rel == nil || rel.Field == nil {
			continue
		}
		c := rel.ParseConstraint()
		if c == nil || c.Name == "" || c.Schema != sch {
			continue
		}
		if _, ok := seen[c.Name]; ok {
			continue
		}
		columns := make([]string, 0, len(c.ForeignKeys))
		for _, fk := range c.ForeignKeys {
			if fk != nil {
				columns = append(columns, fk.DBName)
			}
		}
		refCols := make([]string, 0, len(c.References))
		for _, ref := range c.References {
			if ref != nil {
				refCols = append(refCols, ref.DBName)
			}
		}
		refTable := ""
		if c.ReferenceSchema != nil {
			refTable = c.ReferenceSchema.Table
		}
		constraints = append(constraints, ConstraintSnapshot{
			Name:             c.Name,
			Type:             "FOREIGN KEY",
			Columns:          columns,
			ReferenceTable:   refTable,
			ReferenceColumns: refCols,
			OnUpdate:         c.OnUpdate,
			OnDelete:         c.OnDelete,
		})
		seen[c.Name] = struct{}{}
	}

	sort.Slice(constraints, func(i, j int) bool {
		if constraints[i].Name == constraints[j].Name {
			return constraints[i].Type < constraints[j].Type
		}
		return constraints[i].Name < constraints[j].Name
	})

	return constraints
}

func (a *DBAdapter) snapshotDatabase() (map[string]*TableSchema, error) {
	tables, err := a.db.Migrator().GetTables()
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}

	result := make(map[string]*TableSchema, len(tables))
	ns := a.db.NamingStrategy
	for _, table := range tables {
		if table == (schemaMigration{}).TableName() {
			continue
		}

		columns, err := a.db.Migrator().ColumnTypes(table)
		if err != nil {
			return nil, fmt.Errorf("describe table %s: %w", table, err)
		}

		sch := &schema.Schema{
			Name:           ns.SchemaName(table),
			Table:          table,
			Fields:         make([]*schema.Field, 0, len(columns)),
			FieldsByDBName: make(map[string]*schema.Field, len(columns)),
		}

		for _, column := range columns {
			field := columnTypeToField(column, sch, ns)
			sch.Fields = append(sch.Fields, field)
			sch.FieldsByDBName[field.DBName] = field
		}

		gormIndexes, idxErr := a.db.Migrator().GetIndexes(table)
		if idxErr != nil {
			return nil, fmt.Errorf("indexes for %s: %w", table, idxErr)
		}
		result[strings.ToLower(table)] = &TableSchema{
			Schema:  sch,
			Indexes: convertIndexes(gormIndexes, sch),
		}
	}
	return result, nil
}

func columnTypeToField(col gorm.ColumnType, parent *schema.Schema, ns schema.Namer) *schema.Field {
	field := &schema.Field{
		Schema:                 parent,
		DBName:                 col.Name(),
		Name:                   ns.SchemaName(col.Name()),
		TagSettings:            map[string]string{"COLUMN": col.Name()},
		AutoIncrementIncrement: schema.DefaultAutoIncrementIncrement,
	}
	field.DataType = schema.DataType(col.DatabaseTypeName())

	if pk, ok := col.PrimaryKey(); ok {
		field.PrimaryKey = pk
	}
	if ai, ok := col.AutoIncrement(); ok {
		field.AutoIncrement = ai
	}
	if nullable, ok := col.Nullable(); ok {
		field.NotNull = !nullable
	}
	if unique, ok := col.Unique(); ok {
		field.Unique = unique
	}
	if length, ok := col.Length(); ok {
		field.Size = int(length)
	}
	if precision, scale, ok := col.DecimalSize(); ok {
		field.Precision = int(precision)
		field.Scale = int(scale)
	}
	if def, ok := col.DefaultValue(); ok {
		field.DefaultValue = def
		field.HasDefaultValue = def != "" && !strings.EqualFold(def, "null")
	}
	return field
}

func convertIndexes(indexes []gorm.Index, sch *schema.Schema) []*schema.Index {
	result := make([]*schema.Index, 0, len(indexes))
	for _, idx := range indexes {
		name := idx.Name()
		if pk, ok := idx.PrimaryKey(); ok && pk {
			continue
		}

		converted := &schema.Index{Name: name}
		if unique, ok := idx.Unique(); ok && unique {
			converted.Class = "UNIQUE"
		}

		converted.Option = idx.Option()
		for _, col := range idx.Columns() {
			if field := sch.FieldsByDBName[col]; field != nil {
				converted.Fields = append(converted.Fields, schema.IndexOption{Field: field})
			}
		}

		result = append(result, converted)
	}
	return result
}

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
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		return "", fmt.Errorf("prepare migrations dir: %w", err)
	}
	path := filepath.Join(migrationsDir, diffHelperFile)
	return path, os.WriteFile(path, []byte(source), 0o644)
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
