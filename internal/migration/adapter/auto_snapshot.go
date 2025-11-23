package adapter

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

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
