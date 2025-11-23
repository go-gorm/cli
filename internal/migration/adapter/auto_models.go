package adapter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"

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
