package field

import (
	"gorm.io/gorm/clause"
)

type (
	// QueryInterface defines the interface for building conditions
	QueryInterface = clause.Expression

	// AssociationInterface defines association
	AssociationInterface interface {
		Name() string
	}

	// ColumnInterface defines the interface for column operations
	ColumnInterface interface {
		Column() clause.Column
	}

	// Selectable defines the interface for Select operations
	Selectable interface {
		buildSelectArg() any
	}

	// AssignerExpression combines a clause.Expression with an Assignments provider.
	AssignerExpression interface {
		clause.Expression
		clause.Assigner
	}

	// OrderableInterface defines the interface for orderable expressions
	OrderableInterface interface {
		Build(clause.Builder)
	}

	// DistinctInterface defines the interface for distinct operations
	DistinctInterface interface {
		buildSelectArg() any
	}
)

func BuildSelectExpr(ss ...Selectable) clause.Expression {
	if len(ss) == 0 {
		return nil
	}
	exprs := make([]clause.Expression, 0, len(ss))
	for _, s := range ss {
		arg := s.buildSelectArg()
		switch v := arg.(type) {
		case clause.Expression:
			exprs = append(exprs, v)
		case clause.Column:
			// clause.Column does not implement clause.Expression in gorm v1.31.0,
			// wrap it with an Expr so it can be used as an expression.
			exprs = append(exprs, clause.Expr{SQL: "?", Vars: []any{v}})
		case string:
			// Wrap column name as an Expr with a Column so it builds quoted properly.
			exprs = append(exprs, clause.Expr{SQL: "?", Vars: []any{clause.Column{Name: v}}})
		default:
			exprs = append(exprs, clause.Expr{SQL: "?", Vars: []any{arg}})
		}
	}
	if len(exprs) == 1 {
		return exprs[0]
	}
	return clause.CommaExpression{Exprs: exprs}
}
