package generated

import (
	"context"

	"gorm.io/cli/gorm/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Interface wraps gorm.Interface[T] and adds type-safe helpers for columns/associations.
type Interface[T any] struct{ gorm.ChainInterface[T] }

// G constructs a type-safe query builder for model T, mirroring gorm.G[T].
func G[T any](db *gorm.DB, opts ...clause.Expression) Interface[T] {
	return Interface[T]{gorm.G[T](db, opts...)}
}

// Select accepts generated field selectors, including columns and field-based expressions.
// Examples:
//
//	generated.G[User](db).Select(generated.User.Name).Find(ctx)
//	generated.G[User](db).Select(generated.User.Name.As("n")).Find(ctx)
//	generated.G[User](db).Select(generated.User.Age.SelectExpr("AVG(?) AS avg_age", generated.User.Age)).Find(ctx)
func (e Interface[T]) Select(items ...field.Selectable) Interface[T] {
	if len(items) == 0 {
		return e
	}
	cols, exprs := field.SelectArgs(items...)
	sel := clause.Select{Columns: cols}
	if len(exprs) > 0 {
		sel.Expressions = exprs
	}
	return Interface[T]{e.ChainInterface.Clauses(sel)}
}

// Omit accepts generated columns only. Use WithTable(...) on fields to qualify if needed.
func (e Interface[T]) Omit(cols ...interface{ Column() clause.Column }) Interface[T] {
	if len(cols) == 0 {
		return e
	}
	names := make([]string, 0, len(cols))
	for _, c := range cols {
		col := c.Column()
		if col.Table != "" {
			names = append(names, col.Table+"."+col.Name)
		} else {
			names = append(names, col.Name)
		}
	}
	return Interface[T]{e.ChainInterface.Omit(names...)}
}

// Preload accepts generated association fields (Struct/Slice) with optional conditions.
func (e Interface[T]) Preload(assoc interface{ Name() string }, conds ...clause.Expression) Interface[T] {
	if len(conds) == 0 {
		return Interface[T]{e.ChainInterface.Preload(assoc.Name())}
	}
	if len(conds) == 1 {
		return Interface[T]{e.ChainInterface.Preload(assoc.Name(), conds[0])}
	}
	return Interface[T]{e.ChainInterface.Preload(assoc.Name(), clause.And(conds...))}
}

// Group groups by generated columns.
func (e Interface[T]) Group(cols ...interface{ Column() clause.Column }) Interface[T] {
	if len(cols) == 0 {
		return e
	}
	gb := clause.GroupBy{}
	for _, c := range cols {
		gb.Columns = append(gb.Columns, c.Column())
	}
	return Interface[T]{e.ChainInterface.Clauses(gb)}
}

// Having is type-safe via clause.Expression built from field helpers.
func (e Interface[T]) Having(conds ...clause.Expression) Interface[T] {
	if len(conds) == 0 {
		return e
	}
	if len(conds) == 1 {
		return Interface[T]{e.ChainInterface.Having(conds[0])}
	}
	return Interface[T]{e.ChainInterface.Having(clause.And(conds...))}
}

// Where is already type-safe via field helpers (clause.Expression).
func (e Interface[T]) Where(conds ...clause.Expression) Interface[T] {
	if len(conds) == 0 {
		return e
	}
	if len(conds) == 1 {
		return Interface[T]{e.ChainInterface.Where(conds[0])}
	}
	return Interface[T]{e.ChainInterface.Where(clause.And(conds...))}
}

// Or combines conditions with OR.
func (e Interface[T]) Or(conds ...clause.Expression) Interface[T] {
	if len(conds) == 0 {
		return e
	}
	if len(conds) == 1 {
		return Interface[T]{e.ChainInterface.Or(conds[0])}
	}
	return Interface[T]{e.ChainInterface.Or(clause.And(conds...))}
}

// Not negates conditions.
func (e Interface[T]) Not(conds ...clause.Expression) Interface[T] {
	if len(conds) == 0 {
		return e
	}
	if len(conds) == 1 {
		return Interface[T]{e.ChainInterface.Not(conds[0])}
	}
	return Interface[T]{e.ChainInterface.Not(clause.And(conds...))}
}

// Order passes through order arguments but keeps the typed chain.
func (e Interface[T]) Order(values ...any) Interface[T] {
	return Interface[T]{e.ChainInterface.Order(values...)}
}

// Limit sets LIMIT.
func (e Interface[T]) Limit(n int) Interface[T] { return Interface[T]{e.ChainInterface.Limit(n)} }

// Offset sets OFFSET.
func (e Interface[T]) Offset(n int) Interface[T] { return Interface[T]{e.ChainInterface.Offset(n)} }

// Distinct marks SELECT DISTINCT.
func (e Interface[T]) Distinct(values ...any) Interface[T] {
	return Interface[T]{e.ChainInterface.Distinct(values...)}
}

// Clauses adds low-level clauses (use with care).
func (e Interface[T]) Clauses(exprs ...clause.Expression) Interface[T] {
	if len(exprs) == 0 {
		return e
	}
	return Interface[T]{e.ChainInterface.Clauses(exprs...)}
}

// Table sets the table or subquery.
func (e Interface[T]) Table(name string, args ...any) Interface[T] {
	return Interface[T]{e.ChainInterface.Table(name, args...)}
}

// Set collects assignments for Create/Update.
func (e Interface[T]) Set(assigns ...any) Interface[T] {
	if len(assigns) == 0 {
		return e
	}
	return Interface[T]{e.ChainInterface.Set(assigns...)}
}

// Raw sets a raw SQL for scanning.
func (e Interface[T]) Raw(sql string, vars ...any) Interface[T] {
	return Interface[T]{e.ChainInterface.Raw(sql, vars...)}
}

// --- Finishers (mirror gorm.Interface[T]) ---

func (e Interface[T]) Find(ctx context.Context) ([]T, error) { return e.ChainInterface.Find(ctx) }

func (e Interface[T]) First(ctx context.Context) (T, error) { return e.ChainInterface.First(ctx) }

func (e Interface[T]) Take(ctx context.Context) (T, error) { return e.ChainInterface.Take(ctx) }

func (e Interface[T]) Last(ctx context.Context) (T, error) { return e.ChainInterface.Last(ctx) }

func (e Interface[T]) Count(ctx context.Context, column any) (int64, error) {
	return e.ChainInterface.Count(ctx, column)
}

func (e Interface[T]) Scan(ctx context.Context, dest any) error {
	return e.ChainInterface.Scan(ctx, dest)
}

func (e Interface[T]) Exec(ctx context.Context, sql string, values ...any) error {
	return e.ChainInterface.Exec(ctx, sql, values...)
}

func (e Interface[T]) Create(ctx context.Context) error { return e.ChainInterface.Create(ctx) }

func (e Interface[T]) Update(ctx context.Context, args ...any) (int64, error) {
	return e.ChainInterface.Update(ctx, args...)
}

func (e Interface[T]) Delete(ctx context.Context) (int64, error) { return e.ChainInterface.Delete(ctx) }

// --- Additional builder mirrors ---

func (e Interface[T]) Debug() Interface[T] { return Interface[T]{e.ChainInterface.Debug()} }

func (e Interface[T]) Session(sess *gorm.Session) Interface[T] {
	return Interface[T]{e.ChainInterface.Session(sess)}
}

func (e Interface[T]) WithContext(ctx context.Context) Interface[T] {
	return Interface[T]{e.ChainInterface.WithContext(ctx)}
}

func (e Interface[T]) Joins(query string, args ...any) Interface[T] {
	return Interface[T]{e.ChainInterface.Joins(query, args...)}
}

func (e Interface[T]) Scopes(funcs ...func(gorm.Interface[T]) gorm.Interface[T]) Interface[T] {
	return Interface[T]{e.ChainInterface.Scopes(funcs...)}
}

func (e Interface[T]) Unscoped() Interface[T] { return Interface[T]{e.ChainInterface.Unscoped()} }
