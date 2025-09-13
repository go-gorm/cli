// Package field provides type-safe field operations for GORM query builder.
package field

import (
	"gorm.io/gorm/clause"
)

// Bool represents a boolean field that provides type-safe operations for building SQL queries.
type Bool struct {
	column clause.Column
}

// Column returns the underlying column for this field
func (b Bool) Column() clause.Column { return b.column }

// WithColumn creates a new Bool field with the specified column name.
// This method allows you to change the column name while keeping other properties.
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	isEnabled := isActive.WithColumn("is_enabled")
func (b Bool) WithColumn(name string) Bool {
	column := b.column
	column.Name = name
	return Bool{column: column}
}

// WithTable creates a new Bool field with the specified table name.
// This method is useful when working with joins and you need to qualify the column with a table name.
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	userIsActive := isActive.WithTable("users")
func (b Bool) WithTable(name string) Bool {
	column := b.column
	column.Table = name
	return Bool{column: column}
}

// Query functions

// Eq creates an equality comparison expression (field = value).
// Use this to compare the boolean field with a specific boolean value.
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	// Generate: WHERE is_active = true
//	condition := isActive.Eq(true)
func (b Bool) Eq(value bool) clause.Expression {
	return clause.Eq{Column: b.column, Value: value}
}

// EqExpr creates an equality comparison expression (field = expression).
// Use this to compare the boolean field with another expression or subquery.
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	isEnabled := field.Bool{column: clause.Column{Name: "is_enabled"}}
//	// Generate: WHERE is_active = is_enabled
//	condition := isActive.EqExpr(isEnabled)
func (b Bool) EqExpr(expr clause.Expression) clause.Expression {
	return clause.Eq{Column: b.column, Value: expr}
}

// NeqExpr creates a not equal comparison expression (field != expression).
// Use this to check if the boolean field is different from another expression.
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	isEnabled := field.Bool{column: clause.Column{Name: "is_enabled"}}
//	// Generate: WHERE is_active != is_enabled
//	condition := isActive.NeqExpr(isEnabled)
func (b Bool) NeqExpr(expr clause.Expression) clause.Expression {
	return clause.Neq{Column: b.column, Value: expr}
}

// IsNull creates a NULL check expression (field IS NULL).
// Use this to check if the boolean field value is NULL.
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	// Generate: WHERE is_active IS NULL
//	condition := isActive.IsNull()
func (b Bool) IsNull() clause.Expression {
	return clause.Expr{SQL: "? IS NULL", Vars: []any{b.column}}
}

// IsNotNull creates a NOT NULL check expression (field IS NOT NULL).
// Use this to check if the boolean field value is not NULL.
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	// Generate: WHERE is_active IS NOT NULL
//	condition := isActive.IsNotNull()
func (b Bool) IsNotNull() clause.Expression {
	return clause.Expr{SQL: "? IS NOT NULL", Vars: []any{b.column}}
}

// Set functions for UPDATE operations

// Set creates an assignment expression for UPDATE operations (field = value).
// Use this to set the boolean field to a specific boolean value.
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	// Generate: SET is_active = true
//	assignment := isActive.Set(true)
func (b Bool) Set(val bool) clause.Assignment {
	return clause.Assignment{Column: b.column, Value: val}
}

// SetExpr creates an assignment expression for UPDATE operations (field = expression).
// Use this to set the boolean field to the result of another expression or field.
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	isEnabled := field.Bool{column: clause.Column{Name: "is_enabled"}}
//	// Generate: SET is_active = is_enabled
//	assignment := isActive.SetExpr(isEnabled)
func (b Bool) SetExpr(expr clause.Expression) clause.Assignment {
	return clause.Assignment{Column: b.column, Value: expr}
}

// Boolean logic functions

// AndExpr creates a logical AND expression (field AND expression).
// Use this to combine the boolean field with another expression using logical AND.
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	isEnabled := field.Bool{column: clause.Column{Name: "is_enabled"}}
//	// Generate: WHERE is_active AND is_enabled
//	condition := isActive.AndExpr(isEnabled)
func (b Bool) AndExpr(expr clause.Expression) AssignerExpression {
	return colOpExpr{col: b.column, sql: "? AND ?", vars: []any{b.column, expr}}
}

// OrExpr creates a logical OR expression (field OR expression).
// Use this to combine the boolean field with another expression using logical OR.
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	isEnabled := field.Bool{column: clause.Column{Name: "is_enabled"}}
//	// Generate: WHERE is_active OR is_enabled
//	condition := isActive.OrExpr(isEnabled)
func (b Bool) OrExpr(expr clause.Expression) AssignerExpression {
	return colOpExpr{col: b.column, sql: "? OR ?", vars: []any{b.column, expr}}
}

// Not creates a logical NOT expression (NOT field).
// Use this to negate the boolean field value.
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	// Generate: WHERE NOT is_active
//	condition := isActive.Not()
func (b Bool) Not() AssignerExpression {
	return colOpExpr{col: b.column, sql: "NOT ?", vars: []any{b.column}}
}

// Xor creates a logical XOR expression (field XOR value).
// Use this to create an exclusive OR condition between the field and a boolean value.
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	// Generate: WHERE is_active XOR true
//	condition := isActive.Xor(true)
func (b Bool) Xor(value bool) AssignerExpression {
	return colOpExpr{col: b.column, sql: "? XOR ?", vars: []any{b.column, value}}
}

// XorExpr creates a logical XOR expression (field XOR expression).
// Use this to create an exclusive OR condition between the field and another expression.
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	isEnabled := field.Bool{column: clause.Column{Name: "is_enabled"}}
//	// Generate: WHERE is_active XOR is_enabled
//	condition := isActive.XorExpr(isEnabled)
func (b Bool) XorExpr(expr clause.Expression) AssignerExpression {
	return colOpExpr{col: b.column, sql: "? XOR ?", vars: []any{b.column, expr}}
}

// Expr creates a custom SQL expression with parameters.
// Use this to create complex SQL expressions with placeholders and values.
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	// Generate: WHERE is_active AND status IN ('active', 'pending')
//	condition := isActive.Expr("? AND status IN (?, ?)", isActive, "active", "pending")
func (b Bool) Expr(expr string, values ...any) clause.Expression {
	return clause.Expr{SQL: expr, Vars: values}
}

// Order expressions for sorting operations

// Asc creates an ascending order expression for ORDER BY clauses.
// Use this to sort the boolean field in ascending order (false before true).
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	// Generate: ORDER BY is_active ASC
//	order := isActive.Asc()
func (b Bool) Asc() clause.OrderByColumn {
	return clause.OrderByColumn{Column: b.column, Desc: false}
}

// Desc creates a descending order expression for ORDER BY clauses.
// Use this to sort the boolean field in descending order (true before false).
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	// Generate: ORDER BY is_active DESC
//	order := isActive.Desc()
func (b Bool) Desc() clause.OrderByColumn {
	return clause.OrderByColumn{Column: b.column, Desc: true}
}

// OrderExpr creates a custom ORDER BY expression with parameters.
// Use this to create complex ordering expressions for boolean fields.
//
// Example:
//
//	isActive := field.Bool{column: clause.Column{Name: "is_active"}}
//	// Generate: ORDER BY CASE WHEN is_active THEN 0 ELSE 1 END
//	order := isActive.OrderExpr("CASE WHEN ? THEN 0 ELSE 1 END", isActive)
func (b Bool) OrderExpr(expr string, values ...any) clause.Expression {
	return clause.Expr{SQL: expr, Vars: values}
}

// buildSelectArg allows Bool to be passed to Select(...)
func (b Bool) buildSelectArg() any { return b.column }

// As creates an alias for this column usable in Select(...)
func (b Bool) As(alias string) Selectable {
	return selectExpr{clause.Expr{SQL: "? AS ?", Vars: []any{b.column, clause.Column{Name: alias}}}}
}

// SelectExpr wraps a custom expression built from this field for Select(...)
func (b Bool) SelectExpr(sql string, values ...any) Selectable {
	return selectExpr{clause.Expr{SQL: sql, Vars: values}}
}
