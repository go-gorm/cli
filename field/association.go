package field

import (
	"gorm.io/gorm/clause"
)

// Struct represents a struct field for single association operations
type Struct[T any] struct {
	associationWithConditions[T]
}

// Slice represents a slice field for multiple association operations
type Slice[T any] struct {
	associationWithConditions[T]
}

// associationWithConditions represents a field with conditions that can be applied to both Struct and Slice
type associationWithConditions[T any] struct {
	name       string
	conditions []clause.Expression
}

// WithName creates a new Struct with the specified field name
func (s Struct[T]) WithName(name string) Struct[T] {
	return Struct[T]{associationWithConditions[T]{name: name}}
}

// Name returns the association name (field name on the parent model)
func (s Struct[T]) Name() string { return s.name }

// WithName creates a new Slice with the specified field name
func (s Slice[T]) WithName(name string) Slice[T] {
	return Slice[T]{associationWithConditions[T]{name: name}}
}

// Name returns the association name (field name on the parent model)
func (s Slice[T]) Name() string { return s.name }

// Where adds conditions to a Struct field
func (s Struct[T]) Where(conditions ...clause.Expression) associationWithConditions[T] {
	return associationWithConditions[T]{
		name:       s.name,
		conditions: conditions,
	}
}

// Where adds conditions to a Slice field
func (s Slice[T]) Where(conditions ...clause.Expression) associationWithConditions[T] {
	return associationWithConditions[T]{
		name:       s.name,
		conditions: conditions,
	}
}

// Create prepares an association create operation for a single (has one/belongs to) association.
// Use with Set(...).Update(ctx) to create and associate a record per matched parent.
func (s Struct[T]) Create(assignments ...clause.Assignment) clause.Association {
	return clause.Association{
		Association: s.name,
		Type:        clause.OpCreate,
		Set:         assignments,
	}
}

// Update updates records in the associated table
// Update prepares an association update operation with optional conditions.
// Use with Set(...).Update(ctx) to update matched associated records for matched parents.
func (w associationWithConditions[T]) Update(assignments ...clause.Assignment) clause.Association {
	return clause.Association{
		Association: w.name,
		Type:        clause.OpUpdate,
		Conditions:  w.conditions,
		Set:         assignments,
	}
}

// Delete removes records from the associated table
// Delete prepares an association delete operation.
// Use with Set(...).Update(ctx) to delete matched associated records for matched parents.
func (w associationWithConditions[T]) Delete() clause.Association {
	return clause.Association{
		Association: w.name,
		Type:        clause.OpDelete,
		Conditions:  w.conditions,
	}
}

// Unlink removes the association without deleting associated records.
// Unlink semantics:
// - belongs to: sets the parent's foreign key to NULL
// - has one / has many: sets the child's foreign key to NULL
// - many2many: removes join table rows only
// Use with Set(...).Update(ctx).
func (w associationWithConditions[T]) Unlink() clause.Association {
	return clause.Association{
		Association: w.name,
		Type:        clause.OpUnlink,
		Conditions:  w.conditions,
	}
}

// Create prepares an association create operation for a slice (has many/many2many) association.
// Creates one associated record per matched parent using provided assignments.
func (s Slice[T]) Create(assignments ...clause.Assignment) clause.Association {
	return clause.Association{
		Association: s.name,
		Type:        clause.OpCreate,
		Set:         assignments,
	}
}

// CreateInBatch prepares an association batch create for a slice association.
// Creates all provided records for each matched parent.
func (s Slice[T]) CreateInBatch(records []T) clause.Association {
	vals := make([]any, len(records))
	for i := range records {
		vals[i] = &records[i]
	}
	return clause.Association{
		Association: s.name,
		Type:        clause.OpCreate,
		Values:      vals,
	}
}
