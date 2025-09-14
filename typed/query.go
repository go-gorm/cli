package typed

import (
	"context"

	"gorm.io/cli/gorm/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Interface[T any] interface {
	Raw(sql string, values ...any) gorm.ExecInterface[T]
	Exec(ctx context.Context, sql string, values ...interface{}) error
	CreateInterface[T]
}

type CreateInterface[T any] interface {
	gorm.ExecInterface[T]
	Scopes(scopes ...func(db *gorm.Statement)) ChainInterface[T]
	Where(...field.QueryInterface) ChainInterface[T]
	Not(...field.QueryInterface) ChainInterface[T]
	Or(...field.QueryInterface) ChainInterface[T]
	Limit(offset int) ChainInterface[T]
	Offset(offset int) ChainInterface[T]
	Joins(query clause.JoinTarget, on func(db JoinBuilder, joinTable clause.Table, curTable clause.Table) error) ChainInterface[T]
	Preload(assoc field.AssociationInterface, query func(db PreloadBuilder) error) ChainInterface[T]
	Select(...field.Selectable) ChainInterface[T]
	Omit(...field.ColumnInterface) ChainInterface[T]
	MapColumns(m map[string]string) ChainInterface[T]
	Distinct(...field.ColumnInterface) ChainInterface[T]
	Group(sel field.ColumnInterface) ChainInterface[T]
	Having(...field.QueryInterface) ChainInterface[T]
	Order(field.OrderableInterface) ChainInterface[T]

	Delete(ctx context.Context) (rowsAffected int, err error)
	Update(ctx context.Context, name string, value any) (rowsAffected int, err error)
	Updates(ctx context.Context, t T) (rowsAffected int, err error)
	Count(ctx context.Context, column string) (result int64, err error)

	Table(name string, args ...interface{}) CreateInterface[T]
	Create(ctx context.Context, r *T) error
	CreateInBatches(ctx context.Context, r *[]T, batchSize int) error

	Build(builder clause.Builder)
	Set(assignments ...clause.Assigner) gorm.SetCreateOrUpdateInterface[T]
}

type ChainInterface[T any] interface {
	ChainExecInterface[T]
	Scopes(scopes ...func(db *gorm.Statement)) ChainInterface[T]
	Where(...field.QueryInterface) ChainInterface[T]
	Not(...field.QueryInterface) ChainInterface[T]
	Or(...field.QueryInterface) ChainInterface[T]
	Limit(offset int) ChainInterface[T]
	Offset(offset int) ChainInterface[T]
	Joins(query clause.JoinTarget, on func(db JoinBuilder, joinTable clause.Table, curTable clause.Table) error) ChainInterface[T]
	Preload(assoc field.AssociationInterface, query func(db PreloadBuilder) error) ChainInterface[T]
	Select(...field.Selectable) ChainInterface[T]
	Omit(...field.ColumnInterface) ChainInterface[T]
	MapColumns(m map[string]string) ChainInterface[T]
	Distinct(...field.ColumnInterface) ChainInterface[T]
	Group(field.ColumnInterface) ChainInterface[T]
	Having(...field.QueryInterface) ChainInterface[T]
	Order(field.OrderableInterface) ChainInterface[T]

	Table(name string, args ...interface{}) ChainInterface[T]
	Build(builder clause.Builder)
}

type ChainExecInterface[T any] interface {
	gorm.ExecInterface[T]

	Delete(ctx context.Context) (rowsAffected int, err error)
	Update(ctx context.Context, name string, value any) (rowsAffected int, err error)
	Updates(ctx context.Context, t T) (rowsAffected int, err error)
	Count(ctx context.Context, column string) (result int64, err error)

	Set(assignments ...clause.Assigner) gorm.SetUpdateOnlyInterface[T]
}

// Builder adapters used in callbacks
type JoinBuilder interface {
	Select(...field.ColumnInterface) JoinBuilder
	Omit(...field.ColumnInterface) JoinBuilder
	Where(...field.QueryInterface) JoinBuilder
	Not(...field.QueryInterface) JoinBuilder
	Or(...field.QueryInterface) JoinBuilder
}

type PreloadBuilder interface {
	Select(...field.ColumnInterface) PreloadBuilder
	Omit(...field.ColumnInterface) PreloadBuilder
	Where(...field.QueryInterface) PreloadBuilder
	Not(...field.QueryInterface) PreloadBuilder
	Or(...field.QueryInterface) PreloadBuilder
	Limit(offset int) PreloadBuilder
	Offset(offset int) PreloadBuilder
	Order(field.OrderableInterface) PreloadBuilder
	LimitPerRecord(num int) PreloadBuilder
}

type g[T any] struct {
	g gorm.Interface[T]
	createG[T]
}

type createG[T any] struct {
	g gorm.CreateInterface[T]
	chainG[T]
}

type chainG[T any] struct {
	g gorm.ChainInterface[T]
	ChainExecInterface[T]
}

func G[T any](db *gorm.DB, opts ...clause.Expression) Interface[T] {
	v := gorm.G[T](db, opts...)
	return &g[T]{
		g: v,
		createG: createG[T]{
			g: v,
			chainG: chainG[T]{
				g:                  v.Scopes(),
				ChainExecInterface: v.Scopes(),
			},
		},
	}
}

func (v g[T]) Raw(sql string, values ...interface{}) gorm.ExecInterface[T] {
	return v.g.Raw(sql, values...)
}

func (v g[T]) Exec(ctx context.Context, sql string, values ...interface{}) error {
	return v.g.Exec(ctx, sql, values...)
}

func (c createG[T]) Table(name string, args ...interface{}) CreateInterface[T] {
	v := c.g.Table(name, args...)
	return createG[T]{
		g: v,
		chainG: chainG[T]{
			g:                  v.Scopes(),
			ChainExecInterface: v.Scopes(),
		},
	}
}

func (c createG[T]) Set(assignments ...clause.Assigner) gorm.SetCreateOrUpdateInterface[T] {
	return c.g.Set(assignments...)
}

func (c createG[T]) Create(ctx context.Context, r *T) error {
	return c.g.Create(ctx, r)
}

func (c createG[T]) CreateInBatches(ctx context.Context, r *[]T, batchSize int) error {
	return c.g.CreateInBatches(ctx, r, batchSize)
}

func (c chainG[T]) with(v gorm.ChainInterface[T]) chainG[T] {
	return chainG[T]{
		g:                  v,
		ChainExecInterface: v,
	}
}

func (c chainG[T]) Table(name string, args ...interface{}) ChainInterface[T] {
	return c.with(c.g.Table(name, args...))
}

func (c chainG[T]) Scopes(scopes ...func(db *gorm.Statement)) ChainInterface[T] {
	return c.with(c.g.Scopes(scopes...))
}

func (c chainG[T]) Where(exprs ...field.QueryInterface) ChainInterface[T] {
	return c.with(c.g.Where(exprs))
}

func (c chainG[T]) Not(exprs ...field.QueryInterface) ChainInterface[T] {
	return c.with(c.g.Not(exprs))
}

func (c chainG[T]) Or(exprs ...field.QueryInterface) ChainInterface[T] {
	return c.with(c.g.Or(exprs))
}

func (c chainG[T]) Limit(limit int) ChainInterface[T] {
	return c.with(c.g.Limit(limit))
}

func (c chainG[T]) Offset(offset int) ChainInterface[T] {
	return c.with(c.g.Offset(offset))
}

// JoinBuilder adapter that collects conditions/selects/omits on a *gorm.DB
type joinBuilder struct {
	db gorm.JoinBuilder
}

func (q *joinBuilder) Where(exprs ...field.QueryInterface) JoinBuilder {
	q.db = q.db.Where(exprs)
	return q
}

func (q *joinBuilder) Or(exprs ...field.QueryInterface) JoinBuilder {
	q.db = q.db.Or(exprs)
	return q
}

func (q *joinBuilder) Not(exprs ...field.QueryInterface) JoinBuilder {
	q.db = q.db.Not(exprs)
	return q
}

func (q *joinBuilder) Select(cols ...field.ColumnInterface) JoinBuilder {
	q.db = q.db.Select(columnsToNames(cols...)...)
	return q
}

func (q *joinBuilder) Omit(cols ...field.ColumnInterface) JoinBuilder {
	q.db = q.db.Omit(columnsToNames(cols...)...)
	return q
}

type preloadBuilder struct {
	db gorm.PreloadBuilder
}

func (q *preloadBuilder) Where(exprs ...field.QueryInterface) PreloadBuilder {
	q.db = q.db.Where(exprs)
	return q
}

func (q *preloadBuilder) Or(exprs ...field.QueryInterface) PreloadBuilder {
	q.db = q.db.Or(exprs)
	return q
}

func (q *preloadBuilder) Not(exprs ...field.QueryInterface) PreloadBuilder {
	q.db = q.db.Not(exprs)
	return q
}

func (q *preloadBuilder) Select(cols ...field.ColumnInterface) PreloadBuilder {
	q.db = q.db.Select(columnsToNames(cols...)...)
	return q
}

func (q *preloadBuilder) Omit(cols ...field.ColumnInterface) PreloadBuilder {
	q.db = q.db.Omit(columnsToNames(cols...)...)
	return q
}

func (q *preloadBuilder) Limit(limit int) PreloadBuilder {
	q.db = q.db.Limit(limit)
	return q
}

func (q *preloadBuilder) Offset(offset int) PreloadBuilder {
	q.db = q.db.Offset(offset)
	return q
}

func (q *preloadBuilder) Order(o field.OrderableInterface) PreloadBuilder {
	q.db = q.db.Order(o)
	return q
}

func (q *preloadBuilder) LimitPerRecord(num int) PreloadBuilder {
	q.db = q.db.LimitPerRecord(num)
	return q
}

func (c chainG[T]) Joins(jt clause.JoinTarget, on func(db JoinBuilder, joinTable clause.Table, curTable clause.Table) error) ChainInterface[T] {
	return c.with(c.g.Joins(jt, func(db gorm.JoinBuilder, joinTable clause.Table, curTable clause.Table) error {
		return on(&joinBuilder{db}, joinTable, curTable)
	}))
}

func (c chainG[T]) Select(ss ...field.Selectable) ChainInterface[T] {
	args := field.BuildSelectExpr(ss...)
	return c.with(c.g.Select("?", args))
}

func (c chainG[T]) Omit(cols ...field.ColumnInterface) ChainInterface[T] {
	names := columnsToNames(cols...)
	return c.with(c.g.Omit(names...))
}

func (c chainG[T]) MapColumns(m map[string]string) ChainInterface[T] {
	return c.with(c.g.MapColumns(m))
}

func (c chainG[T]) Set(assignments ...clause.Assigner) gorm.SetUpdateOnlyInterface[T] {
	return c.g.Set(assignments...)
}

func (c chainG[T]) Distinct(cols ...field.ColumnInterface) ChainInterface[T] {
	args := make([]interface{}, 0, len(cols))
	for _, col := range columnsToNames(cols...) {
		args = append(args, col)
	}
	return c.with(c.g.Distinct(args...))
}

func (c chainG[T]) Group(sel field.ColumnInterface) ChainInterface[T] {
	return c.with(c.g.Group(sel.Column().Name))
}

func (c chainG[T]) Having(exprs ...field.QueryInterface) ChainInterface[T] {
	return c.with(c.g.Having(exprs))
}

func (c chainG[T]) Order(o field.OrderableInterface) ChainInterface[T] {
	return c.with(c.g.Order(o))
}

func (c chainG[T]) Preload(assoc field.AssociationInterface, query func(db PreloadBuilder) error) ChainInterface[T] {
	return c.with(c.g.Preload(assoc.Name(), func(db gorm.PreloadBuilder) error {
		return query(&preloadBuilder{db: db})
	}))
}

func (c chainG[T]) Build(builder clause.Builder) {
	c.g.Build(builder)
}

func columnsToNames(cols ...field.ColumnInterface) []string {
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		col := c.Column()
		if col.Table != "" {
			out = append(out, col.Table+"."+col.Name)
		} else {
			out = append(out, col.Name)
		}
	}
	return out
}
