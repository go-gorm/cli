package field

import "gorm.io/gorm/clause"

type (
	// QueryInterface defines the interface for building conditions
	QueryInterface = clause.Expr

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
