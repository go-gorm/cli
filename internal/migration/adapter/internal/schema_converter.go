package internal

import (
	"strings"

	"gorm.io/cli/gorm/internal/migration/schema"
	"gorm.io/gorm"
	gormSchema "gorm.io/gorm/schema"
)

// SchemaConverter handles conversion between GORM and universal schema representations.
type SchemaConverter struct{}

// NewSchemaConverter creates a new schema converter.
func NewSchemaConverter() *SchemaConverter {
	return &SchemaConverter{}
}

// ColumnTypeToField converts a GORM column type to a universal field.
func (sc *SchemaConverter) ColumnTypeToField(col gorm.ColumnType, ns gormSchema.Namer) *schema.Field {
	dataType := col.DatabaseTypeName()
	size, _ := col.Length()
	precision, scale, _ := col.DecimalSize()

	// Reset size/precision for types that don't use them
	switch strings.ToUpper(dataType) {
	case "BIGINT", "INT", "INTEGER", "SMALLINT", "TINYINT", "MEDIUMINT", "FLOAT", "DOUBLE",
		"INT8", "INT4", "INT2", "SERIAL", "BIGSERIAL", "TIMESTAMP", "TIMESTAMPTZ", "TIME", "DATE",
		"REAL", "DOUBLE PRECISION", "FLOAT4", "FLOAT8":
		precision = 0
		scale = 0
		size = 0
	}

	// Normalize Postgres types
	dataType = sc.normalizeDataType(dataType)

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

// GormIndexesToUniversalIndexes converts GORM indexes to universal indexes.
func (sc *SchemaConverter) GormIndexesToUniversalIndexes(indexes []gorm.Index) []*schema.Index {
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

func (sc *SchemaConverter) normalizeDataType(dataType string) string {
	switch strings.ToUpper(dataType) {
	case "INT8":
		return "bigint"
	case "INT4":
		return "integer"
	case "INT2":
		return "smallint"
	case "BOOL":
		return "boolean"
	case "FLOAT4", "REAL":
		return "decimal"
	case "FLOAT8", "DOUBLE PRECISION":
		return "decimal"
	case "NUMERIC":
		return "decimal"
	case "JSONB":
		return "jsonb"
	default:
		return dataType
	}
}
