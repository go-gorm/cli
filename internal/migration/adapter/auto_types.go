package adapter

import (
	"fmt"
	"sort"
	"strings"

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

// helperModelSnapshot mirrors the JSON emitted by the Go helper when collecting
// model metadata.
type helperModelSnapshot struct {
	PackagePath string              `json:"pkg"`
	TypeName    string              `json:"type"`
	Table       helperTableSnapshot `json:"table"`
}

type helperTableSnapshot struct {
	Name        string                     `json:"name"`
	Fields      []helperFieldSnapshot      `json:"fields"`
	Indexes     []helperIndexSnapshot      `json:"indexes"`
	Constraints []helperConstraintSnapshot `json:"constraints"`
}

type helperFieldSnapshot struct {
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

type helperIndexSnapshot struct {
	Name    string                     `json:"name"`
	Class   string                     `json:"class"`
	Type    string                     `json:"type"`
	Where   string                     `json:"where"`
	Comment string                     `json:"comment"`
	Option  string                     `json:"option"`
	Fields  []helperIndexFieldSnapshot `json:"fields"`
}

type helperIndexFieldSnapshot struct {
	Column     string `json:"column"`
	Expression string `json:"expression"`
	Sort       string `json:"sort"`
	Collate    string `json:"collate"`
	Length     int    `json:"length"`
	Priority   int    `json:"priority"`
}

type helperConstraintSnapshot struct {
	Name             string   `json:"name"`
	Type             string   `json:"type"`
	Columns          []string `json:"columns"`
	ReferenceTable   string   `json:"ref_table"`
	ReferenceColumns []string `json:"ref_columns"`
	OnUpdate         string   `json:"on_update"`
	OnDelete         string   `json:"on_delete"`
	Expression       string   `json:"expression"`
}

func (snap helperModelSnapshot) toTableSchema() (*TableSchema, error) {
	tblSchema := &schema.Schema{
		Name:           snap.TypeName,
		Table:          snap.Table.Name,
		Fields:         make([]*schema.Field, 0, len(snap.Table.Fields)),
		FieldsByName:   make(map[string]*schema.Field, len(snap.Table.Fields)),
		FieldsByDBName: make(map[string]*schema.Field, len(snap.Table.Fields)),
		DBNames:        make([]string, 0, len(snap.Table.Fields)),
	}

	for idx := range snap.Table.Fields {
		field := snap.Table.Fields[idx].toSchemaField(tblSchema)
		tblSchema.Fields = append(tblSchema.Fields, field)
		tblSchema.FieldsByName[field.Name] = field
		tblSchema.FieldsByDBName[field.DBName] = field
		tblSchema.DBNames = append(tblSchema.DBNames, field.DBName)
		if field.PrimaryKey {
			tblSchema.PrimaryFields = append(tblSchema.PrimaryFields, field)
			tblSchema.PrimaryFieldDBNames = append(tblSchema.PrimaryFieldDBNames, field.DBName)
			if tblSchema.PrioritizedPrimaryField == nil {
				tblSchema.PrioritizedPrimaryField = field
			}
		}
	}

	indexes := make([]*schema.Index, 0, len(snap.Table.Indexes))
	for _, idx := range snap.Table.Indexes {
		indexes = append(indexes, idx.toSchemaIndex(tblSchema))
	}
	constraints := make([]*schema.Constraint, 0, len(snap.Table.Constraints))
	for _, cons := range snap.Table.Constraints {
		constraints = append(constraints, cons.toSchemaConstraint(tblSchema))
	}

	model := &ModelRef{PackagePath: snap.PackagePath, TypeName: snap.TypeName}
	return &TableSchema{Schema: tblSchema, Indexes: indexes, Constraints: constraints, Model: model}, nil
}

func (snap helperFieldSnapshot) toSchemaField(parent *schema.Schema) *schema.Field {
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

func (snap helperIndexSnapshot) toSchemaIndex(parent *schema.Schema) *schema.Index {
	idx := &schema.Index{
		Name:    snap.Name,
		Class:   snap.Class,
		Type:    snap.Type,
		Where:   snap.Where,
		Comment: snap.Comment,
		Option:  snap.Option,
	}

	// Ensure deterministic ordering when comparing indexes.
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

func (snap helperConstraintSnapshot) toSchemaConstraint(parent *schema.Schema) *schema.Constraint {
	return &schema.Constraint{
		Name:   snap.Name,
		Schema: parent,
	}
}

// parseHelperSnapshots converts helper JSON output into TableSchemas keyed by table name.
func parseHelperSnapshots(snaps []helperModelSnapshot) (map[string]*TableSchema, error) {
	result := make(map[string]*TableSchema, len(snaps))
	for _, snap := range snaps {
		tableSchema, err := snap.toTableSchema()
		if err != nil {
			return nil, fmt.Errorf("convert table schema for %s.%s: %w", snap.PackagePath, snap.TypeName, err)
		}
		result[strings.ToLower(tableSchema.Schema.Table)] = tableSchema
	}
	return result, nil
}
