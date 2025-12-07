package source

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"gorm.io/cli/gorm/internal/migration/schema"
	"gorm.io/gorm"
	gormSchema "gorm.io/gorm/schema"
)

// Parser encapsulates all operations for parsing model definitions from Go source code.
type Parser struct{}

// New creates a new source parser.
func New() *Parser {
	return &Parser{}
}

// GetSchemasFromModels receives model instances defined in main.go,
// uses GORM's reflection capabilities to parse their schemas, and converts them to the universal structure.
func (p *Parser) GetSchemasFromModels(db *gorm.DB, models []any) ([]*schema.Table, error) {
	if len(models) == 0 {
		return nil, nil
	}

	// 1. Capture snapshots from model instances, using the existing logic.
	snaps, err := captureModelSnapshots(db, models)
	if err != nil {
		return nil, fmt.Errorf("capture model snapshots: %w", err)
	}

	// 2. Convert snapshots to the universal schema.Table format.
	return toUniversalTables(snaps), nil
}

// toUniversalTables converts the internal snapshot representation to the universal schema.Table format.
func toUniversalTables(snaps []ModelSnapshot) []*schema.Table {
	tables := make([]*schema.Table, 0, len(snaps))
	for _, snap := range snaps {
		table := &schema.Table{
			Name:     snap.Table.Name,
			ModelRef: snap.PackagePath + "." + snap.TypeName,
			Fields:   make([]*schema.Field, len(snap.Table.Fields)),
			Indexes:  make([]*schema.Index, len(snap.Table.Indexes)),
		}

		for i, fSnap := range snap.Table.Fields {
			var defValPtr *string
			if fSnap.HasDefaultValue {
				// Make a copy to avoid all pointers pointing to the same loop variable.
				val := fSnap.DefaultValue
				defValPtr = &val
			}

			table.Fields[i] = &schema.Field{
				DBName:        fSnap.DBName,
				DataType:      fSnap.GORMDataType,
				IsPrimaryKey:  fSnap.PrimaryKey,
				IsNullable:    !fSnap.NotNull,
				IsUnique:      fSnap.Unique,
				Size:          fSnap.Size,
				Precision:     fSnap.Precision,
				Scale:         fSnap.Scale,
				DefaultValue:  defValPtr,
				AutoIncrement: fSnap.AutoIncrement,
				Comment:       fSnap.Comment,
			}
		}

		for i, iSnap := range snap.Table.Indexes {
			columns := make([]string, 0, len(iSnap.Fields))
			for _, f := range iSnap.Fields {
				columns = append(columns, f.Column)
			}
			table.Indexes[i] = &schema.Index{
				Name:     iSnap.Name,
				Columns:  columns,
				IsUnique: strings.EqualFold(iSnap.Class, "UNIQUE"),
				Option:   iSnap.Option,
			}
		}
		tables = append(tables, table)
	}
	return tables
}

// --- Logic migrated from adapter/db_adapter.go ---

func captureModelSnapshots(db *gorm.DB, models []any) ([]ModelSnapshot, error) {
	snaps := make([]ModelSnapshot, 0, len(models))
	seen := make(map[string]struct{})
	for _, mdl := range models {
		snap, err := buildSnapshot(db, mdl)
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

func encodeSchemaTable(sch *gormSchema.Schema) TableSnapshot {
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

func encodeField(field *gormSchema.Field) FieldSnapshot {
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

func encodeIndex(idx *gormSchema.Index) IndexSnapshot {
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

func encodeConstraints(sch *gormSchema.Schema) []ConstraintSnapshot {
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
