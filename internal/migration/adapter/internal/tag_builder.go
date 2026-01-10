package internal

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
	gormSchema "gorm.io/gorm/schema"
)

// TagBuilder builds GORM struct tags from column information.
type TagBuilder struct{}

// NewTagBuilder creates a new tag builder.
func NewTagBuilder() *TagBuilder {
	return &TagBuilder{}
}

// BuildGormTag builds a GORM tag string from column information.
func (tb *TagBuilder) BuildGormTag(ns gormSchema.Namer, col gorm.ColumnType, fieldName string) string {
	tags := []string{}

	if pk, _ := col.PrimaryKey(); pk {
		tags = append(tags, "primaryKey")
	}

	if col.Name() != ns.TableName(fieldName) {
		tags = append(tags, "column:"+col.Name())
	}

	dbType := strings.ToUpper(col.DatabaseTypeName())
	switch dbType {
	case "JSON":
		tags = append(tags, "type:json")
	case "JSONB":
		tags = append(tags, "type:jsonb")
	case "BLOB":
		tags = append(tags, "type:blob")
	case "TEXT":
		tags = append(tags, "type:text")
	case "DECIMAL":
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

// BuildStructTag builds a complete struct tag from a map of tag values.
func (tb *TagBuilder) BuildStructTag(tags map[string]string) string {
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

// GetDBNameFromField extracts the database column name from an AST field.
func (tb *TagBuilder) GetDBNameFromField(field interface {
	Tag() *struct{ Value string }
	Names() []interface{ Name() string }
}, ns gormSchema.Namer) string {
	tag := field.Tag()
	if tag == nil {
		return ns.TableName(field.Names()[0].Name())
	}

	tagValue := strings.Trim(tag.Value, "`")
	reflectTag := reflect.StructTag(tagValue)
	gormTag := reflectTag.Get("gorm")
	settings := gormSchema.ParseTagSetting(gormTag, ";")

	if _, disabled := settings["-"]; disabled {
		return "-"
	}

	if column, ok := settings["COLUMN"]; ok && column != "" {
		return column
	}

	return ns.TableName(field.Names()[0].Name())
}
