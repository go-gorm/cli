package adapter

import (
	"fmt"
	"reflect"
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
	ColumnTypes map[string]gorm.ColumnType
}

func (ts *TableSchema) columnTypeForField(field *schema.Field) gorm.ColumnType {
	if ts == nil || len(ts.ColumnTypes) == 0 || field == nil {
		return nil
	}
	return ts.columnTypeByName(field.DBName)
}

func (ts *TableSchema) columnTypeByName(name string) gorm.ColumnType {
	if name == "" || len(ts.ColumnTypes) == 0 {
		return nil
	}
	if ct, ok := ts.ColumnTypes[strings.ToLower(name)]; ok {
		return ct
	}
	return nil
}

func (a *DBAdapter) collectModelSchemas() (map[string]*TableSchema, error) {
	if len(a.cfg.DiffModels) == 0 {
		return map[string]*TableSchema{}, nil
	}
	result := make(map[string]*TableSchema, len(a.cfg.DiffModels))
	seen := make(map[string]struct{})
	for _, mdl := range a.cfg.DiffModels {
		schema, err := buildModelSchema(a.db, mdl)
		if err != nil {
			return nil, err
		}
		key := schemaKey(schema.Schema.Table, schema.Model)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result[strings.ToLower(schema.Schema.Table)] = schema
	}
	return result, nil
}

func buildModelSchema(db *gorm.DB, model interface{}) (*TableSchema, error) {
	if model == nil {
		return nil, fmt.Errorf("migration runtime: nil diff model")
	}
	typ := reflect.TypeOf(model)
	if typ.Kind() != reflect.Ptr || typ.Elem().Kind() != reflect.Struct {
		return nil, fmt.Errorf("migration runtime: diff model must be pointer to struct (got %T)", model)
	}
	value := reflect.New(typ.Elem()).Interface()
	stmt := &gorm.Statement{DB: db.Session(&gorm.Session{})}
	if err := stmt.Parse(value); err != nil {
		return nil, err
	}
	if stmt.Schema == nil {
		return nil, fmt.Errorf("migration runtime: parsed schema is nil for %T", model)
	}
	modelRef := &ModelRef{PackagePath: typ.Elem().PkgPath(), TypeName: typ.Elem().Name()}
	return buildTableSchemaFromSchema(stmt.Schema, modelRef), nil
}

func schemaKey(table string, model *ModelRef) string {
	if model == nil {
		return strings.ToLower(table)
	}
	return strings.ToLower(table + ":" + model.PackagePath + "." + model.TypeName)
}

func buildTableSchemaFromSchema(sch *schema.Schema, model *ModelRef) *TableSchema {
	if sch == nil {
		return nil
	}
	indexes := sch.ParseIndexes()
	indexCopies := make([]*schema.Index, len(indexes))
	copy(indexCopies, indexes)
	return &TableSchema{
		Schema:      sch,
		Indexes:     indexCopies,
		Constraints: collectSchemaConstraints(sch),
		Model:       model,
	}
}

func collectSchemaConstraints(sch *schema.Schema) []*schema.Constraint {
	if sch == nil {
		return nil
	}
	seen := make(map[string]struct{})
	constraints := make([]*schema.Constraint, 0)
	for _, rel := range sch.Relationships.Relations {
		if rel == nil {
			continue
		}
		c := rel.ParseConstraint()
		if c == nil || c.Name == "" || c.Schema != sch {
			continue
		}
		if _, ok := seen[c.Name]; ok {
			continue
		}
		seen[c.Name] = struct{}{}
		constraints = append(constraints, c)
	}
	return constraints
}
