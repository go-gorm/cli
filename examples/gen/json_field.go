package examples

import (
	"encoding/json"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// JSON is an example field wrapper for JSON columns.
//
// It demonstrates how to create a custom field type with database-specific
// SQL generation and the required WithColumn method so it can be
// used in config mappings and query building.
type JSON struct {
	column clause.Column
}

// WithColumn sets the column name for this JSON field.
func (j JSON) WithColumn(name string) JSON {
	c := j.column
	c.Name = name
	return JSON{column: c}
}

// Equal builds an expression using database-specific JSON functions to compare
// the JSON value at the given JSON path with the provided value.
// Example: json_extract(column, '$.vip') = 1
// Path must be a valid JSON path like "$.vip".
func (j JSON) Equal(path string, value any) clause.Expression {
	return jsonEqualExpr{col: j.column, path: path, val: value}
}

type jsonEqualExpr struct {
	col  clause.Column
	path string
	val  any
}

func (e jsonEqualExpr) Build(builder clause.Builder) {
	if stmt, ok := builder.(*gorm.Statement); ok {
		switch stmt.Dialector.Name() {
		case "mysql":
			v, _ := json.Marshal(e.val)
			clause.Expr{SQL: "JSON_EXTRACT(?, ?) = CAST(? AS JSON)", Vars: []any{e.col, e.path, string(v)}}.Build(builder)
		case "sqlite":
			clause.Expr{SQL: "json_valid(?) AND json_extract(?, ?) = ?", Vars: []any{e.col, e.col, e.path, e.val}}.Build(builder)
		default:
			clause.Expr{SQL: "jsonb_extract_path_text(?, ?) = ?", Vars: []any{e.col, e.path[2:], e.val}}.Build(builder)
		}
	}
}

// Contains creates a JSON containment predicate.
// Example (MySQL): JSON_CONTAINS(column, @value)
func (j JSON) Contains(value any) clause.Expression {
	return clause.Expr{SQL: "JSON_CONTAINS(?, ?)", Vars: []any{j.column, value}}
}
