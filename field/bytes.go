// Package field provides type-safe field operations for GORM query builder.
package field

import (
	"gorm.io/gorm/clause"
)

// Bytes represents a byte slice field that provides type-safe operations for building SQL queries.
type Bytes struct {
	column clause.Column
}

// Column returns the underlying column for this field
func (b Bytes) Column() clause.Column { return b.column }

// WithColumn creates a new Bytes field with the specified column name.
// This method allows you to change the column name while keeping other properties.
//
// Example:
//
//	avatar := field.Bytes{column: clause.Column{Name: "user_avatar"}}
//	thumbnail := avatar.WithColumn("thumbnail")
func (b Bytes) WithColumn(name string) Bytes {
	column := b.column
	column.Name = name
	return Bytes{column: column}
}

// WithTable creates a new Bytes field with the specified table name.
// This method is useful when working with joins and you need to qualify the column with a table name.
//
// Example:
//
//	avatar := field.Bytes{column: clause.Column{Name: "avatar"}}
//	userAvatar := avatar.WithTable("users")
func (b Bytes) WithTable(name string) Bytes {
	column := b.column
	column.Table = name
	return Bytes{column: column}
}

// Query functions

// Eq creates an equality comparison expression (field = value).
func (b Bytes) Eq(value []byte) clause.Expression {
	return clause.Eq{Column: b.column, Value: value}
}

// EqExpr creates an equality comparison expression (field = expression).
func (b Bytes) EqExpr(expr clause.Expression) clause.Expression {
	return clause.Eq{Column: b.column, Value: expr}
}

// Neq creates a not equal comparison expression (field != value).
func (b Bytes) Neq(value []byte) clause.Expression {
	return clause.Neq{Column: b.column, Value: value}
}

// NeqExpr creates a not equal comparison expression (field != expression).
func (b Bytes) NeqExpr(expr clause.Expression) clause.Expression {
	return clause.Neq{Column: b.column, Value: expr}
}

// In creates an IN comparison expression (field IN (values...)).
func (b Bytes) In(values ...[]byte) clause.Expression {
	interfaceValues := make([]any, len(values))
	for i, v := range values {
		interfaceValues[i] = v
	}
	return clause.IN{Column: b.column, Values: interfaceValues}
}

// NotIn creates a NOT IN comparison expression (field NOT IN (values...)).
func (b Bytes) NotIn(values ...[]byte) clause.Expression {
	interfaceValues := make([]any, len(values))
	for i, v := range values {
		interfaceValues[i] = v
	}
	return clause.Not(clause.IN{Column: b.column, Values: interfaceValues})
}

// IsNull creates a NULL check expression (field IS NULL).
func (b Bytes) IsNull() clause.Expression {
	return clause.Expr{SQL: "? IS NULL", Vars: []any{b.column}}
}

// IsNotNull creates a NOT NULL check expression (field IS NOT NULL).
func (b Bytes) IsNotNull() clause.Expression {
	return clause.Expr{SQL: "? IS NOT NULL", Vars: []any{b.column}}
}

// Set functions for UPDATE operations

// Set creates an assignment expression for UPDATE operations (field = value).
func (b Bytes) Set(val []byte) clause.Assignment {
	return clause.Assignment{Column: b.column, Value: val}
}

// SetExpr creates an assignment expression for UPDATE operations (field = expression).
func (b Bytes) SetExpr(expr clause.Expression) clause.Assignment {
	return clause.Assignment{Column: b.column, Value: expr}
}

// Binary functions

// Length creates a byte length expression (LENGTH(field)).
func (b Bytes) Length() clause.Expression {
	return clause.Expr{SQL: "LENGTH(?)", Vars: []any{b.column}}
}

// Concat creates a binary concatenation expression (CONCAT(field, value)).
func (b Bytes) Concat(value []byte) AssignerExpression {
	return colOpExpr{col: b.column, sql: "CONCAT(?, ?)", vars: []any{b.column, value}}
}

// Expr creates a custom SQL expression with parameters.
func (b Bytes) Expr(expr string, values ...any) clause.Expression {
	return clause.Expr{SQL: expr, Vars: values}
}

// Order expressions for sorting operations

// Asc creates an ascending order expression for ORDER BY clauses.
func (b Bytes) Asc() clause.OrderByColumn {
	return clause.OrderByColumn{Column: b.column, Desc: false}
}

// Desc creates a descending order expression for ORDER BY clauses.
func (b Bytes) Desc() clause.OrderByColumn {
	return clause.OrderByColumn{Column: b.column, Desc: true}
}

// OrderExpr creates a custom ORDER BY expression with parameters.
func (b Bytes) OrderExpr(expr string, values ...any) clause.Expression {
	return clause.Expr{SQL: expr, Vars: values}
}

// buildSelectArg allows Bytes to be passed to Select(...)
func (b Bytes) buildSelectArg() any { return b.column }

// As creates an alias for this column usable in Select(...)
func (b Bytes) As(alias string) Selectable {
	return selectExpr{clause.Expr{SQL: "? AS ?", Vars: []any{b.column, clause.Column{Name: alias}}}}
}

// SelectExpr wraps a custom expression built from this field for Select(...)
func (b Bytes) SelectExpr(sql string, values ...any) Selectable {
	return selectExpr{clause.Expr{SQL: sql, Vars: values}}
}
