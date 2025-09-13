// Package field provides type-safe field operations for GORM query builder.
package field

import (
	"golang.org/x/exp/constraints"
	"gorm.io/gorm/clause"
)

// Number represents a numeric field that supports both integer and float types.
// It provides type-safe operations for building SQL queries.
type Number[T constraints.Integer | constraints.Float] struct {
	column clause.Column
}

// Column returns the underlying column for this field
func (n Number[T]) Column() clause.Column { return n.column }

// WithColumn creates a new Number field with the specified column name.
// This method allows you to change the column name while keeping other properties.
//
// Example:
//
//	age := NewNumber[int]("user_age")
//	userAge := age.WithColumn("age")
func (n Number[T]) WithColumn(name string) Number[T] {
	column := n.column
	column.Name = name
	return Number[T]{column: column}
}

// WithTable creates a new Number field with the specified table name.
// This method is useful when working with joins and you need to qualify the column with a table name.
//
// Example:
//
//	age := field.Number[int]{column: clause.Column{Name: "age"}}
//	userAge := age.WithTable("users")
func (n Number[T]) WithTable(name string) Number[T] {
	column := n.column
	column.Table = name
	return Number[T]{column: column}
}

// Query functions

// Eq creates an equality comparison expression (field = value).
func (n Number[T]) Eq(value T) clause.Expression {
	return clause.Eq{Column: n.column, Value: value}
}

// EqExpr creates an equality comparison expression (field = expression).
func (n Number[T]) EqExpr(expr clause.Expression) clause.Expression {
	return clause.Eq{Column: n.column, Value: expr}
}

// Neq creates a not equal comparison expression (field != value).
func (n Number[T]) Neq(value T) clause.Expression {
	return clause.Neq{Column: n.column, Value: value}
}

// NeqExpr creates a not equal comparison expression (field != expression).
func (n Number[T]) NeqExpr(expr clause.Expression) clause.Expression {
	return clause.Neq{Column: n.column, Value: expr}
}

// Gt creates a greater than comparison expression (field > value).
func (n Number[T]) Gt(value T) clause.Expression {
	return clause.Gt{Column: n.column, Value: value}
}

// GtExpr creates a greater than comparison expression (field > expression).
func (n Number[T]) GtExpr(expr clause.Expression) clause.Expression {
	return clause.Gt{Column: n.column, Value: expr}
}

// Gte creates a greater than or equal comparison expression (field >= value).
func (n Number[T]) Gte(value T) clause.Expression {
	return clause.Gte{Column: n.column, Value: value}
}

// GteExpr creates a greater than or equal comparison expression (field >= expression).
func (n Number[T]) GteExpr(expr clause.Expression) clause.Expression {
	return clause.Gte{Column: n.column, Value: expr}
}

// Lt creates a less than comparison expression (field < value).
func (n Number[T]) Lt(value T) clause.Expression {
	return clause.Lt{Column: n.column, Value: value}
}

// LtExpr creates a less than comparison expression (field < expression).
func (n Number[T]) LtExpr(expr clause.Expression) clause.Expression {
	return clause.Lt{Column: n.column, Value: expr}
}

// Lte creates a less than or equal comparison expression (field <= value).
func (n Number[T]) Lte(value T) clause.Expression {
	return clause.Lte{Column: n.column, Value: value}
}

// LteExpr creates a less than or equal comparison expression (field <= expression).
func (n Number[T]) LteExpr(expr clause.Expression) clause.Expression {
	return clause.Lte{Column: n.column, Value: expr}
}

// Between creates a range comparison expression (field BETWEEN v1 AND v2).
func (n Number[T]) Between(v1, v2 T) clause.Expression {
	return clause.And(
		clause.Gte{Column: n.column, Value: v1},
		clause.Lte{Column: n.column, Value: v2},
	)
}

// In creates an IN comparison expression (field IN (values...)).
func (n Number[T]) In(values ...T) clause.Expression {
	interfaceValues := make([]any, len(values))
	for i, v := range values {
		interfaceValues[i] = v
	}
	return clause.IN{Column: n.column, Values: interfaceValues}
}

// NotIn creates a NOT IN comparison expression (field NOT IN (values...)).
func (n Number[T]) NotIn(values ...T) clause.Expression {
	interfaceValues := make([]any, len(values))
	for i, v := range values {
		interfaceValues[i] = v
	}
	return clause.Not(clause.IN{Column: n.column, Values: interfaceValues})
}

// IsNull creates a NULL check expression (field IS NULL).
func (n Number[T]) IsNull() clause.Expression {
	return clause.Expr{SQL: "? IS NULL", Vars: []any{n.column}}
}

// IsNotNull creates a NOT NULL check expression (field IS NOT NULL).
func (n Number[T]) IsNotNull() clause.Expression {
	return clause.Expr{SQL: "? IS NOT NULL", Vars: []any{n.column}}
}

// Set functions for UPDATE operations

// Set creates an assignment expression for UPDATE operations (field = value).
func (n Number[T]) Set(val T) clause.Assignment {
	return clause.Assignment{Column: n.column, Value: val}
}

// SetExpr creates an assignment expression for UPDATE operations (field = expression).
func (n Number[T]) SetExpr(expr clause.Expression) clause.Assignment {
	return clause.Assignment{Column: n.column, Value: expr}
}

// Basic SQL expression functions for arithmetic operations

// Incr creates an increment expression (field + value).
func (n Number[T]) Incr(value T) AssignerExpression {
	return colOpExpr{col: n.column, sql: "? + ?", vars: []any{n.column, value}}
}

// Decr creates a decrement expression (field - value).
func (n Number[T]) Decr(value T) AssignerExpression {
	return colOpExpr{col: n.column, sql: "? - ?", vars: []any{n.column, value}}
}

// Mul creates a multiplication expression (field * value).
func (n Number[T]) Mul(value T) AssignerExpression {
	return colOpExpr{col: n.column, sql: "? * ?", vars: []any{n.column, value}}
}

// Div creates a division expression (field / value).
func (n Number[T]) Div(value T) AssignerExpression {
	return colOpExpr{col: n.column, sql: "? / ?", vars: []any{n.column, value}}
}

// Expr creates a custom SQL expression with parameters.
func (n Number[T]) Expr(expr string, values ...any) clause.Expression {
	return clause.Expr{SQL: expr, Vars: values}
}

// numOpExpr is an internal expression that both renders an arithmetic op and carries
// its column so it can be adapted to an assignment by helper functions.
// removed numOpExpr in favor of generic colOpExpr

// Order expressions for sorting operations

// Asc creates an ascending order expression for ORDER BY clauses.
func (n Number[T]) Asc() clause.OrderByColumn {
	return clause.OrderByColumn{Column: n.column, Desc: false}
}

// Desc creates a descending order expression for ORDER BY clauses.
func (n Number[T]) Desc() clause.OrderByColumn {
	return clause.OrderByColumn{Column: n.column, Desc: true}
}

// OrderExpr creates a custom ORDER BY expression with parameters.
func (n Number[T]) OrderExpr(expr string, values ...any) clause.Expression {
	return clause.Expr{SQL: expr, Vars: values}
}

// buildSelectArg allows Number to be passed to Select(...)
func (n Number[T]) buildSelectArg() any { return n.column }

// As creates an alias for this column usable in Select(...)
func (n Number[T]) As(alias string) Selectable {
	return selectExpr{clause.Expr{SQL: "? AS ?", Vars: []any{n.column, clause.Column{Name: alias}}}}
}

// SelectExpr wraps a custom expression built from this field for Select(...)
func (n Number[T]) SelectExpr(sql string, values ...any) Selectable {
	return selectExpr{clause.Expr{SQL: sql, Vars: values}}
}
