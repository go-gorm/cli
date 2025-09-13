// Package field provides type-safe field operations for GORM query builder.
package field

import (
	"time"

	"gorm.io/gorm/clause"
)

// Time represents a time field that provides type-safe operations for building SQL queries.
type Time struct {
	column clause.Column
}

// Column returns the underlying column for this field
func (t Time) Column() clause.Column { return t.column }

// WithColumn creates a new Time field with the specified column name.
// This method allows you to change the column name while keeping other properties.
//
// Example:
//
//	createdAt := field.Time{column: clause.Column{Name: "created_at"}}
//	updatedAt := createdAt.WithColumn("updated_at")
func (t Time) WithColumn(name string) Time {
	column := t.column
	column.Name = name
	return Time{column: column}
}

// WithTable creates a new Time field with the specified table name.
// This method is useful when working with joins and you need to qualify the column with a table name.
//
// Example:
//
//	createdAt := field.Time{column: clause.Column{Name: "created_at"}}
//	userCreatedAt := createdAt.WithTable("users")
func (t Time) WithTable(name string) Time {
	column := t.column
	column.Table = name
	return Time{column: column}
}

// Query functions

// Eq creates an equality comparison expression (field = value).
func (t Time) Eq(value time.Time) clause.Expression {
	return clause.Eq{Column: t.column, Value: value}
}

// EqExpr creates an equality comparison expression (field = expression).
func (t Time) EqExpr(expr clause.Expression) clause.Expression {
	return clause.Eq{Column: t.column, Value: expr}
}

// Neq creates a not equal comparison expression (field != value).
func (t Time) Neq(value time.Time) clause.Expression {
	return clause.Neq{Column: t.column, Value: value}
}

// NeqExpr creates a not equal comparison expression (field != expression).
func (t Time) NeqExpr(expr clause.Expression) clause.Expression {
	return clause.Neq{Column: t.column, Value: expr}
}

// Gt creates a greater than comparison expression (field > value).
func (t Time) Gt(value time.Time) clause.Expression {
	return clause.Gt{Column: t.column, Value: value}
}

// GtExpr creates a greater than comparison expression (field > expression).
func (t Time) GtExpr(expr clause.Expression) clause.Expression {
	return clause.Gt{Column: t.column, Value: expr}
}

// Gte creates a greater than or equal comparison expression (field >= value).
func (t Time) Gte(value time.Time) clause.Expression {
	return clause.Gte{Column: t.column, Value: value}
}

// GteExpr creates a greater than or equal comparison expression (field >= expression).
func (t Time) GteExpr(expr clause.Expression) clause.Expression {
	return clause.Gte{Column: t.column, Value: expr}
}

// Lt creates a less than comparison expression (field < value).
func (t Time) Lt(value time.Time) clause.Expression {
	return clause.Lt{Column: t.column, Value: value}
}

// LtExpr creates a less than comparison expression (field < expression).
func (t Time) LtExpr(expr clause.Expression) clause.Expression {
	return clause.Lt{Column: t.column, Value: expr}
}

// Lte creates a less than or equal comparison expression (field <= value).
func (t Time) Lte(value time.Time) clause.Expression {
	return clause.Lte{Column: t.column, Value: value}
}

// LteExpr creates a less than or equal comparison expression (field <= expression).
func (t Time) LteExpr(expr clause.Expression) clause.Expression {
	return clause.Lte{Column: t.column, Value: expr}
}

// Between creates a range comparison expression (field BETWEEN v1 AND v2).
func (t Time) Between(v1, v2 time.Time) clause.Expression {
	return clause.And(
		clause.Gte{Column: t.column, Value: v1},
		clause.Lte{Column: t.column, Value: v2},
	)
}

// In creates an IN comparison expression (field IN (values...)).
func (t Time) In(values ...time.Time) clause.Expression {
	interfaceValues := make([]any, len(values))
	for i, v := range values {
		interfaceValues[i] = v
	}
	return clause.IN{Column: t.column, Values: interfaceValues}
}

// NotIn creates a NOT IN comparison expression (field NOT IN (values...)).
func (t Time) NotIn(values ...time.Time) clause.Expression {
	interfaceValues := make([]any, len(values))
	for i, v := range values {
		interfaceValues[i] = v
	}
	return clause.Not(clause.IN{Column: t.column, Values: interfaceValues})
}

// IsNull creates a NULL check expression (field IS NULL).
func (t Time) IsNull() clause.Expression {
	return clause.Expr{SQL: "? IS NULL", Vars: []any{t.column}}
}

// IsNotNull creates a NOT NULL check expression (field IS NOT NULL).
func (t Time) IsNotNull() clause.Expression {
	return clause.Expr{SQL: "? IS NOT NULL", Vars: []any{t.column}}
}

// Set functions for UPDATE operations

// Set creates an assignment expression for UPDATE operations (field = value).
func (t Time) Set(val time.Time) clause.Assignment {
	return clause.Assignment{Column: t.column, Value: val}
}

// SetExpr creates an assignment expression for UPDATE operations (field = expression).
func (t Time) SetExpr(expr clause.Expression) clause.Assignment {
	return clause.Assignment{Column: t.column, Value: expr}
}

// Time-specific functions

// Add creates a date addition expression (DATE_ADD(field, INTERVAL seconds SECOND)).
func (t Time) Add(duration time.Duration) AssignerExpression {
	seconds := int64(duration.Seconds())
	return colOpExpr{col: t.column, sql: "DATE_ADD(?, INTERVAL ? SECOND)", vars: []any{t.column, seconds}}
}

// Sub creates a date subtraction expression (DATE_SUB(field, INTERVAL seconds SECOND)).
func (t Time) Sub(duration time.Duration) AssignerExpression {
	seconds := int64(duration.Seconds())
	return colOpExpr{col: t.column, sql: "DATE_SUB(?, INTERVAL ? SECOND)", vars: []any{t.column, seconds}}
}

// DateDiff creates a date difference expression (DATEDIFF(field, date)).
func (t Time) DateDiff(date time.Time) clause.Expression {
	return clause.Expr{SQL: "DATEDIFF(?, ?)", Vars: []any{t.column, date}}
}

// DateFormat creates a date formatting expression (DATE_FORMAT(field, format)).
func (t Time) DateFormat(format string) clause.Expression {
	return clause.Expr{SQL: "DATE_FORMAT(?, ?)", Vars: []any{t.column, format}}
}

// Year extracts the year from the date field.
func (t Time) Year() clause.Expression {
	return clause.Expr{SQL: "YEAR(?)", Vars: []any{t.column}}
}

// Month extracts the month from the date field.
func (t Time) Month() clause.Expression {
	return clause.Expr{SQL: "MONTH(?)", Vars: []any{t.column}}
}

// Day extracts the day from the date field.
func (t Time) Day() clause.Expression {
	return clause.Expr{SQL: "DAY(?)", Vars: []any{t.column}}
}

// Hour extracts the hour from the datetime field.
func (t Time) Hour() clause.Expression {
	return clause.Expr{SQL: "HOUR(?)", Vars: []any{t.column}}
}

// Minute extracts the minute from the datetime field.
func (t Time) Minute() clause.Expression {
	return clause.Expr{SQL: "MINUTE(?)", Vars: []any{t.column}}
}

// Second extracts the second from the datetime field.
func (t Time) Second() clause.Expression {
	return clause.Expr{SQL: "SECOND(?)", Vars: []any{t.column}}
}

// Date extracts the date part from a datetime field.
func (t Time) Date() clause.Expression {
	return clause.Expr{SQL: "DATE(?)", Vars: []any{t.column}}
}

// Time extracts the time part from a datetime field.
func (t Time) Time() clause.Expression {
	return clause.Expr{SQL: "TIME(?)", Vars: []any{t.column}}
}

// Unix converts the datetime to Unix timestamp.
func (t Time) Unix() clause.Expression {
	return clause.Expr{SQL: "UNIX_TIMESTAMP(?)", Vars: []any{t.column}}
}

// Now creates a NOW() expression for current timestamp.
func (t Time) Now() AssignerExpression {
	return colOpExpr{col: t.column, sql: "NOW()", vars: nil}
}

// Expr creates a custom SQL expression with parameters.
func (t Time) Expr(expr string, values ...any) clause.Expression {
	return clause.Expr{SQL: expr, Vars: values}
}

// Order expressions for sorting operations

// Asc creates an ascending order expression for ORDER BY clauses.
func (t Time) Asc() clause.OrderByColumn {
	return clause.OrderByColumn{Column: t.column, Desc: false}
}

// Desc creates a descending order expression for ORDER BY clauses.
func (t Time) Desc() clause.OrderByColumn {
	return clause.OrderByColumn{Column: t.column, Desc: true}
}

// OrderExpr creates a custom ORDER BY expression with parameters.
func (t Time) OrderExpr(expr string, values ...any) clause.Expression {
	return clause.Expr{SQL: expr, Vars: values}
}

// buildSelectArg allows Time to be passed to Select(...)
func (t Time) buildSelectArg() any { return t.column }

// As creates an alias for this column usable in Select(...)
func (t Time) As(alias string) Selectable {
	return selectExpr{clause.Expr{SQL: "? AS ?", Vars: []any{t.column, clause.Column{Name: alias}}}}
}

// SelectExpr wraps a custom expression built from this field for Select(...)
func (t Time) SelectExpr(sql string, values ...any) Selectable {
	return selectExpr{clause.Expr{SQL: sql, Vars: values}}
}
