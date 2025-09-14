package field

import "gorm.io/gorm/clause"

// Field<T> represents a generic field that provides type-safe operations for building SQL queries.
// This is the base type for all field operations in GORM query builder.
type Field[T any] struct {
	column clause.Column
}

// Column returns the underlying clause.Column for selection and grouping.
func (f Field[T]) Column() clause.Column { return f.column }

// WithColumn creates a new Field[T]<T> with the specified column name.
// This method allows you to change the column name while keeping other properties.
//
// Example:
//
//	field := field.Field[T]{column: clause.Column{Name: "original_name"}}
//	newField[T] := field.WithColumn("new_name")
func (f Field[T]) WithColumn(name string) Field[T] {
	column := f.column
	column.Name = name
	return Field[T]{column: column}
}

// WithTable creates a new Field[T]<T> with the specified table name.
// This method is useful when working with joins and you need to qualify the column with a table name.
//
// Example:
//
//	field := field.Field[T]{column: clause.Column{Name: "name"}}
//	userField[T] := field.WithTable("users")
func (f Field[T]) WithTable(name string) Field[T] {
	column := f.column
	column.Table = name
	return Field[T]{column: column}
}

// Eq creates an equality comparison expression (field = value).
// Use this to compare the field with a specific value.
//
// Example:
//
//	field := field.Field[T]{column: clause.Column{Name: "status"}}
//	// Generate: WHERE status = 'active'
//	condition := field.Eq("active")
func (f Field[T]) Eq(value T) clause.Expression {
	return clause.Eq{Column: f.column, Value: value}
}

// EqExpr creates an equality comparison expression (field = expression).
// Use this to compare the field with another expression or subquery.
//
// Example:
//
//	field1 := field.Field[T]{column: clause.Column{Name: "field1"}}
//	field2 := field.Field[T]{column: clause.Column{Name: "field2"}}
//	// Generate: WHERE field1 = field2
//	condition := field1.EqExpr(field2)
func (f Field[T]) EqExpr(expr clause.Expression) clause.Expression {
	return clause.Eq{Column: f.column, Value: expr}
}

// Neq creates a not equal comparison expression (field != value).
// Use this to check if the field is different from a specific value.
//
// Example:
//
//	field := field.Field[T]{column: clause.Column{Name: "status"}}
//	// Generate: WHERE status != 'inactive'
//	condition := field.Neq("inactive")
func (f Field[T]) Neq(value T) clause.Expression {
	return clause.Neq{Column: f.column, Value: value}
}

// NeqExpr creates a not equal comparison expression (field != expression).
// Use this to check if the field is different from another expression.
//
// Example:
//
//	field1 := field.Field[T]{column: clause.Column{Name: "field1"}}
//	field2 := field.Field[T]{column: clause.Column{Name: "field2"}}
//	// Generate: WHERE field1 != field2
//	condition := field1.NeqExpr(field2)
func (f Field[T]) NeqExpr(expr clause.Expression) clause.Expression {
	return clause.Neq{Column: f.column, Value: expr}
}

// IsNull creates a NULL check expression (field IS NULL).
// Use this to check if the field value is NULL.
//
// Example:
//
//	field := field.Field[T]{column: clause.Column{Name: "deleted_at"}}
//	// Generate: WHERE deleted_at IS NULL
//	condition := field.IsNull()
func (f Field[T]) IsNull() clause.Expression {
	return clause.Expr{SQL: "? IS NULL", Vars: []any{f.column}}
}

// IsNotNull creates a NOT NULL check expression (field IS NOT NULL).
// Use this to check if the field value is not NULL.
//
// Example:
//
//	field := field.Field[T]{column: clause.Column{Name: "deleted_at"}}
//	// Generate: WHERE deleted_at IS NOT NULL
//	condition := field.IsNotNull()
func (f Field[T]) IsNotNull() clause.Expression {
	return clause.Expr{SQL: "? IS NOT NULL", Vars: []any{f.column}}
}

// Set creates an assignment expression for UPDATE operations (field = value).
// Use this to set the field to a specific value.
//
// Example:
//
//	field := field.Field[T]{column: clause.Column{Name: "status"}}
//	// Generate: SET status = 'active'
//	assignment := field.Set("active")
func (f Field[T]) Set(value T) clause.Assignment {
	return clause.Assignment{Column: f.column, Value: value}
}

// SetExpr creates an assignment expression for UPDATE operations (field = expression).
// Use this to set the field to the result of another expression or field.
//
// Example:
//
//	field1 := field.Field[T]{column: clause.Column{Name: "field1"}}
//	field2 := field.Field[T]{column: clause.Column{Name: "field2"}}
//	// Generate: SET field1 = field2
//	assignment := field1.SetExpr(field2)
func (f Field[T]) SetExpr(expr clause.Expression) clause.Assignment {
	return clause.Assignment{Column: f.column, Value: expr}
}

// Expr creates a custom SQL expression with parameters.
// Use this to create complex SQL expressions with placeholders and values.
//
// Example:
//
//	field := field.Field[T]{column: clause.Column{Name: "status"}}
//	// Generate: WHERE status IN ('active', 'pending')
//	condition := field.Expr("? IN (?, ?)", field, "active", "pending")
func (f Field[T]) Expr(expr string, values ...any) clause.Expression {
	return clause.Expr{SQL: expr, Vars: values}
}

// Order expressions for sorting operations

// Asc creates an ascending order expression for ORDER BY clauses.
// Use this to sort the field in ascending order.
//
// Example:
//
//	field := field.Field[T]{column: clause.Column{Name: "created_at"}}
//	// Generate: ORDER BY created_at ASC
//	order := field.Asc()
func (f Field[T]) Asc() clause.OrderByColumn {
	return clause.OrderByColumn{Column: f.column, Desc: false}
}

// Desc creates a descending order expression for ORDER BY clauses.
// Use this to sort the field in descending order.
//
// Example:
//
//	field := field.Field[T]{column: clause.Column{Name: "created_at"}}
//	// Generate: ORDER BY created_at DESC
//	order := field.Desc()
func (f Field[T]) Desc() clause.OrderByColumn {
	return clause.OrderByColumn{Column: f.column, Desc: true}
}

// OrderExpr creates a custom ORDER BY expression with parameters.
// Use this to create complex ordering expressions.
//
// Example:
//
//	field := field.Field[T]{column: clause.Column{Name: "name"}}
//	// Generate: ORDER BY CASE WHEN name IS NULL THEN 1 ELSE 0 END
//	order := field.OrderExpr("CASE WHEN ? IS NULL THEN 1 ELSE 0 END", field)
func (f Field[T]) OrderExpr(expr string, values ...any) clause.Expression {
	return clause.Expr{SQL: expr, Vars: values}
}

// buildSelectArg allows Field[T] to be used directly in Select(...)
func (f Field[T]) buildSelectArg() any { return f.column }

// selectExpr wraps an expression to be used in Select(...)
type selectExpr struct{ clause.Expression }

func (e selectExpr) buildSelectArg() any { return e.Expression }

// As creates a column alias usable in Select(...), e.g. SELECT col AS alias
func (f Field[T]) As(alias string) Selectable {
	return selectExpr{clause.Expr{SQL: "? AS ?", Vars: []any{f.column, clause.Column{Name: alias}}}}
}

// SelectExpr wraps a custom expression built from this field for Select(...)
func (f Field[T]) SelectExpr(sql string, values ...any) Selectable {
	return selectExpr{clause.Expr{SQL: sql, Vars: values}}
}
