package field

import "gorm.io/gorm/clause"

// colOpExpr is a generic expression wrapper that also provides Assignments
// so it can be passed directly to Set(...).
type colOpExpr struct {
	col  clause.Column
	sql  string
	vars []any
}

func (e colOpExpr) Build(builder clause.Builder) {
	clause.Expr{SQL: e.sql, Vars: e.vars}.Build(builder)
}

func (e colOpExpr) Assignments() []clause.Assignment {
	return []clause.Assignment{{Column: e.col, Value: e}}
}
