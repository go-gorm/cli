// Package field provides type-safe field operations for GORM query builder.
package field

import (
	"gorm.io/gorm/clause"
)

// String represents a string field that provides type-safe operations for building SQL queries.
type String struct {
	column clause.Column
}

// Column returns the underlying column for this field
func (s String) Column() clause.Column { return s.column }

// WithColumn creates a new String field with the specified column name.
// This method allows you to change the column name while keeping other properties.
//
// Example:
//
//	name := field.String{column: clause.Column{Name: "user_name"}}
//	fullName := name.WithColumn("full_name")
func (s String) WithColumn(name string) String {
	column := s.column
	column.Name = name
	return String{column: column}
}

// WithTable creates a new String field with the specified table name.
// This method is useful when working with joins and you need to qualify the column with a table name.
//
// Example:
//
//	name := field.String{column: clause.Column{Name: "name"}}
//	userName := name.WithTable("users")
func (s String) WithTable(name string) String {
	column := s.column
	column.Table = name
	return String{column: column}
}

// Query functions

// Eq creates an equality comparison expression (field = value).
func (s String) Eq(value string) clause.Expression {
	return clause.Eq{Column: s.column, Value: value}
}

// EqExpr creates an equality comparison expression (field = expression).
func (s String) EqExpr(expr clause.Expression) clause.Expression {
	return clause.Eq{Column: s.column, Value: expr}
}

// Neq creates a not equal comparison expression (field != value).
func (s String) Neq(value string) clause.Expression {
	return clause.Neq{Column: s.column, Value: value}
}

// NeqExpr creates a not equal comparison expression (field != expression).
func (s String) NeqExpr(expr clause.Expression) clause.Expression {
	return clause.Neq{Column: s.column, Value: expr}
}

// Gt creates a greater than comparison expression (field > value).
func (s String) Gt(value string) clause.Expression {
	return clause.Gt{Column: s.column, Value: value}
}

// GtExpr creates a greater than comparison expression (field > expression).
func (s String) GtExpr(expr clause.Expression) clause.Expression {
	return clause.Gt{Column: s.column, Value: expr}
}

// Gte creates a greater than or equal comparison expression (field >= value).
func (s String) Gte(value string) clause.Expression {
	return clause.Gte{Column: s.column, Value: value}
}

// GteExpr creates a greater than or equal comparison expression (field >= expression).
func (s String) GteExpr(expr clause.Expression) clause.Expression {
	return clause.Gte{Column: s.column, Value: expr}
}

// Lt creates a less than comparison expression (field < value).
func (s String) Lt(value string) clause.Expression {
	return clause.Lt{Column: s.column, Value: value}
}

// LtExpr creates a less than comparison expression (field < expression).
func (s String) LtExpr(expr clause.Expression) clause.Expression {
	return clause.Lt{Column: s.column, Value: expr}
}

// Lte creates a less than or equal comparison expression (field <= value).
func (s String) Lte(value string) clause.Expression {
	return clause.Lte{Column: s.column, Value: value}
}

// LteExpr creates a less than or equal comparison expression (field <= expression).
func (s String) LteExpr(expr clause.Expression) clause.Expression {
	return clause.Lte{Column: s.column, Value: expr}
}

// Like creates a LIKE pattern matching expression (field LIKE pattern).
func (s String) Like(pattern string) clause.Expression {
	return clause.Like{Column: s.column, Value: pattern}
}

// NotLike creates a NOT LIKE pattern matching expression (field NOT LIKE pattern).
func (s String) NotLike(pattern string) clause.Expression {
	return clause.Expr{SQL: "? NOT LIKE ?", Vars: []any{s.column, pattern}}
}

// ILike creates a case-insensitive LIKE pattern matching expression (field ILIKE pattern).
func (s String) ILike(pattern string) clause.Expression {
	return clause.Expr{SQL: "? ILIKE ?", Vars: []any{s.column, pattern}}
}

// NotILike creates a case-insensitive NOT LIKE pattern matching expression (field NOT ILIKE pattern).
func (s String) NotILike(pattern string) clause.Expression {
	return clause.Expr{SQL: "? NOT ILIKE ?", Vars: []any{s.column, pattern}}
}

// Regexp creates a regular expression matching expression (field REGEXP pattern).
func (s String) Regexp(pattern string) clause.Expression {
	return clause.Expr{SQL: "? REGEXP ?", Vars: []any{s.column, pattern}}
}

// NotRegexp creates a regular expression not matching expression (field NOT REGEXP pattern).
func (s String) NotRegexp(pattern string) clause.Expression {
	return clause.Expr{SQL: "? NOT REGEXP ?", Vars: []any{s.column, pattern}}
}

// In creates an IN comparison expression (field IN (values...)).
func (s String) In(values ...string) clause.Expression {
	interfaceValues := make([]any, len(values))
	for i, v := range values {
		interfaceValues[i] = v
	}
	return clause.IN{Column: s.column, Values: interfaceValues}
}

// NotIn creates a NOT IN comparison expression (field NOT IN (values...)).
func (s String) NotIn(values ...string) clause.Expression {
	interfaceValues := make([]any, len(values))
	for i, v := range values {
		interfaceValues[i] = v
	}
	return clause.Not(clause.IN{Column: s.column, Values: interfaceValues})
}

// IsNull creates a NULL check expression (field IS NULL).
func (s String) IsNull() clause.Expression {
	return clause.Expr{SQL: "? IS NULL", Vars: []any{s.column}}
}

// IsNotNull creates a NOT NULL check expression (field IS NOT NULL).
func (s String) IsNotNull() clause.Expression {
	return clause.Expr{SQL: "? IS NOT NULL", Vars: []any{s.column}}
}

// Set functions for UPDATE operations

// Set creates an assignment expression for UPDATE operations (field = value).
func (s String) Set(val string) clause.Assignment {
	return clause.Assignment{Column: s.column, Value: val}
}

// SetExpr creates an assignment expression for UPDATE operations (field = expression).
func (s String) SetExpr(expr clause.Expression) clause.Assignment {
	return clause.Assignment{Column: s.column, Value: expr}
}

// String manipulation functions

// Concat creates a string concatenation expression.
func (s String) Concat(value string) AssignerExpression {
	return colOpExpr{col: s.column, sql: "CONCAT(?, ?)", vars: []any{s.column, value}}
}

// Length creates a string length expression.
func (s String) Length() clause.Expression {
	return clause.Expr{SQL: "LENGTH(?)", Vars: []any{s.column}}
}

// Upper creates an uppercase conversion expression.
func (s String) Upper() AssignerExpression {
	return colOpExpr{col: s.column, sql: "UPPER(?)", vars: []any{s.column}}
}

// Lower creates a lowercase conversion expression.
func (s String) Lower() AssignerExpression {
	return colOpExpr{col: s.column, sql: "LOWER(?)", vars: []any{s.column}}
}

// Trim creates a whitespace trimming expression.
func (s String) Trim() AssignerExpression {
	return colOpExpr{col: s.column, sql: "TRIM(?)", vars: []any{s.column}}
}

// Left creates a left substring expression.
func (s String) Left(length int) AssignerExpression {
	return colOpExpr{col: s.column, sql: "LEFT(?, ?)", vars: []any{s.column, length}}
}

// Right creates a right substring expression.
func (s String) Right(length int) AssignerExpression {
	return colOpExpr{col: s.column, sql: "RIGHT(?, ?)", vars: []any{s.column, length}}
}

// Substring creates a substring expression.
func (s String) Substring(start, length int) AssignerExpression {
	return colOpExpr{col: s.column, sql: "SUBSTRING(?, ?, ?)", vars: []any{s.column, start, length}}
}

// Expr creates a custom SQL expression with parameters.
func (s String) Expr(expr string, values ...any) clause.Expression {
	return clause.Expr{SQL: expr, Vars: values}
}

// Order expressions for sorting operations

// Asc creates an ascending order expression for ORDER BY clauses.
func (s String) Asc() clause.OrderByColumn {
	return clause.OrderByColumn{Column: s.column, Desc: false}
}

// Desc creates a descending order expression for ORDER BY clauses.
func (s String) Desc() clause.OrderByColumn {
	return clause.OrderByColumn{Column: s.column, Desc: true}
}

// OrderExpr creates a custom ORDER BY expression with parameters.
func (s String) OrderExpr(expr string, values ...any) clause.Expression {
	return clause.Expr{SQL: expr, Vars: values}
}

// buildSelectArg allows String to be passed to Select(...)
func (s String) buildSelectArg() any { return s.column }

// As creates an alias for this column usable in Select(...)
func (s String) As(alias string) Selectable {
	return selectExpr{clause.Expr{SQL: "? AS ?", Vars: []any{s.column, clause.Column{Name: alias}}}}
}

// SelectExpr wraps a custom expression built from this field for Select(...)
func (s String) SelectExpr(sql string, values ...any) Selectable {
	return selectExpr{clause.Expr{SQL: sql, Vars: values}}
}
