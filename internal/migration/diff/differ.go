package diff

import (
	"fmt"
	"io"
	"strings"

	"gorm.io/cli/gorm/internal/migration/schema"
)

// Result encapsulates the differences between two schema sets.
type Result struct {
	CreatedTables  []*schema.Table
	DroppedTables  []*schema.Table
	ModifiedTables []*ModifiedTable
}

// Empty returns true if no differences were found.
func (r Result) Empty() bool {
	return len(r.CreatedTables) == 0 && len(r.DroppedTables) == 0 && len(r.ModifiedTables) == 0
}

// ModifiedTable describes the specific changes for a modified table.
type ModifiedTable struct {
	TableName       string
	SourceModelRef  string
	AddedColumns    []*schema.Field
	DroppedColumns  []*schema.Field
	ModifiedColumns []*ModifiedColumn
}

// ModifiedColumn describes a column that has changed.
type ModifiedColumn struct {
	Old *schema.Field
	New *schema.Field
}

// Differ is a stateless comparison engine.
type Differ struct{}

// New creates a new Differ.
func New() *Differ { return &Differ{} }

// Compare receives two sets of tables and returns their differences.
func (d *Differ) Compare(sourceState, dbState []*schema.Table) Result {
	var result Result
	sourceTables := make(map[string]*schema.Table, len(sourceState))
	for _, t := range sourceState {
		sourceTables[strings.ToLower(t.Name)] = t
	}

	dbTables := make(map[string]*schema.Table, len(dbState))
	for _, t := range dbState {
		dbTables[strings.ToLower(t.Name)] = t
	}

	seen := make(map[string]struct{})
	for tableName, modelTable := range sourceTables {
		seen[tableName] = struct{}{}
		dbTable, ok := dbTables[tableName]
		if !ok {
			result.CreatedTables = append(result.CreatedTables, modelTable)
			continue
		}
		if modified := diffTable(modelTable, dbTable); modified != nil {
			result.ModifiedTables = append(result.ModifiedTables, modified)
		}
	}
	for tableName, dbTable := range dbTables {
		if _, ok := seen[tableName]; ok {
			continue
		}
		result.DroppedTables = append(result.DroppedTables, dbTable)
	}
	// TODO: Sort results for deterministic output
	return result
}

func diffTable(model, db *schema.Table) *ModifiedTable {
	addedCols, droppedCols, modifiedCols := diffColumns(model.Fields, db.Fields)

	if len(addedCols) == 0 && len(droppedCols) == 0 && len(modifiedCols) == 0 {
		return nil
	}
	return &ModifiedTable{
		TableName:       model.Name,
		SourceModelRef:  model.ModelRef,
		AddedColumns:    addedCols,
		DroppedColumns:  droppedCols,
		ModifiedColumns: modifiedCols,
	}
}

func diffColumns(model, db []*schema.Field) (added, dropped []*schema.Field, modified []*ModifiedColumn) {
	modelFields := make(map[string]*schema.Field, len(model))
	for _, f := range model {
		modelFields[strings.ToLower(f.DBName)] = f
	}
	dbFields := make(map[string]*schema.Field, len(db))
	for _, f := range db {
		dbFields[strings.ToLower(f.DBName)] = f
	}

	for key, field := range modelFields {
		dbField, ok := dbFields[key]
		if !ok {
			added = append(added, field)
			continue
		}
		if !fieldsEqual(field, dbField) {
			modified = append(modified, &ModifiedColumn{Old: dbField, New: field})
		}
		delete(dbFields, key)
	}
	for _, field := range dbFields {
		dropped = append(dropped, field)
	}
	return
}

func fieldsEqual(a, b *schema.Field) bool {
	// Abridged comparison for brevity. A real implementation would be more thorough.
	if a == nil || b == nil {
		return a == b
	}
	// Compare DefaultValue, checking for nil pointers
	aDefault, bDefault := "", ""
	if a.DefaultValue != nil {
		aDefault = *a.DefaultValue
	}
	if b.DefaultValue != nil {
		bDefault = *b.DefaultValue
	}

	return strings.EqualFold(a.DBName, b.DBName) &&
		a.IsPrimaryKey == b.IsPrimaryKey &&
		a.AutoIncrement == b.AutoIncrement &&
		strings.EqualFold(a.DataType, b.DataType) &&
		a.Size == b.Size &&
		a.Precision == b.Precision &&
		a.Scale == b.Scale &&
		a.IsNullable == b.IsNullable &&
		a.IsUnique == b.IsUnique &&
		(a.DefaultValue != nil) == (b.DefaultValue != nil) &&
		strings.TrimSpace(strings.ToLower(aDefault)) == strings.TrimSpace(strings.ToLower(bDefault))
}

// WriteSchemaDiff writes the schema differences in a human-readable format.
func WriteSchemaDiff(w io.Writer, diff Result) {
	if diff.Empty() {
		fmt.Fprintln(w, "Models match the database schema")
		return
	}
	fmt.Fprintln(w, "Model ↔ DB diff:")
	if len(diff.CreatedTables) > 0 {
		fmt.Fprintln(w, "\t+ Created Tables:")
		for _, table := range diff.CreatedTables {
			fmt.Fprintf(w, "\t\t- %s (%s)\n", table.Name, table.ModelRef)
		}
	}
	if len(diff.DroppedTables) > 0 {
		fmt.Fprintln(w, "\t- Dropped Tables:")
		for _, table := range diff.DroppedTables {
			fmt.Fprintf(w, "\t\t- %s\n", table.Name)
		}
	}
	for _, table := range diff.ModifiedTables {
		fmt.Fprintf(w, "\t~ %s (%s)\n", table.TableName, table.SourceModelRef)

		// 显示新增列
		if len(table.AddedColumns) > 0 {
			fmt.Fprintln(w, "\t\t+ Added Columns:")
			for _, col := range table.AddedColumns {
				fmt.Fprintf(w, "\t\t\t- %s (%s)\n", col.DBName, describeFieldDetails(col))
			}
		}

		// 显示删除列
		if len(table.DroppedColumns) > 0 {
			fmt.Fprintln(w, "\t\t- Dropped Columns:")
			for _, col := range table.DroppedColumns {
				fmt.Fprintf(w, "\t\t\t- %s (%s)\n", col.DBName, describeFieldDetails(col))
			}
		}

		// 显示修改列
		if len(table.ModifiedColumns) > 0 {
			fmt.Fprintln(w, "\t\t~ Modified Columns:")
			for _, modCol := range table.ModifiedColumns {
				fmt.Fprintf(w, "\t\t\t- %s:\n", modCol.New.DBName)
				describeFieldChangesWithIndent(w, modCol.Old, modCol.New)
			}
		}
	}
}

// describeFieldDetails provides a human-readable summary of field properties
func describeFieldDetails(field *schema.Field) string {
	if field == nil {
		return "<nil>"
	}

	parts := []string{field.DataType}

	if field.Size > 0 {
		parts = append(parts, fmt.Sprintf("size:%d", field.Size))
	}
	if field.Precision > 0 || field.Scale > 0 {
		parts = append(parts, fmt.Sprintf("precision:%d,scale:%d", field.Precision, field.Scale))
	}
	if field.IsPrimaryKey {
		parts = append(parts, "PK")
	}
	if field.AutoIncrement {
		parts = append(parts, "autoincr")
	}
	if !field.IsNullable {
		parts = append(parts, "NOT NULL")
	}
	if field.IsUnique {
		parts = append(parts, "UNIQUE")
	}
	if field.DefaultValue != nil && *field.DefaultValue != "" {
		parts = append(parts, fmt.Sprintf("default:%s", *field.DefaultValue))
	}
	if field.Comment != "" {
		parts = append(parts, fmt.Sprintf("comment:\"%s\"", field.Comment))
	}

	return strings.Join(parts, ", ")
}

// describeFieldChanges describes the specific changes between two field versions
func describeFieldChanges(w io.Writer, oldField, newField *schema.Field) {
	if oldField == nil || newField == nil {
		fmt.Fprintf(w, "\tError: field comparison invalid (one field is nil)\n")
		return
	}

	// Data type changed
	if oldField.DataType != newField.DataType {
		fmt.Fprintf(w, "\t\t- Type: %s → %s\n", oldField.DataType, newField.DataType)
	}

	// Size changed
	if oldField.Size != newField.Size {
		fmt.Fprintf(w, "\t\t- Size: %d → %d\n", oldField.Size, newField.Size)
	}

	// Precision changed
	if oldField.Precision != newField.Precision {
		fmt.Fprintf(w, "\t\t- Precision: %d → %d\n", oldField.Precision, newField.Precision)
	}

	// Scale changed
	if oldField.Scale != newField.Scale {
		fmt.Fprintf(w, "\t\t- Scale: %d → %d\n", oldField.Scale, newField.Scale)
	}

	// Primary key changed
	if oldField.IsPrimaryKey != newField.IsPrimaryKey {
		fmt.Fprintf(w, "\t\t- Primary Key: %t → %t\n", oldField.IsPrimaryKey, newField.IsPrimaryKey)
	}

	// Auto increment changed
	if oldField.AutoIncrement != newField.AutoIncrement {
		fmt.Fprintf(w, "\t\t- Auto Increment: %t → %t\n", oldField.AutoIncrement, newField.AutoIncrement)
	}

	// Nullable changed
	if oldField.IsNullable != newField.IsNullable {
		fmt.Fprintf(w, "\t\t- Nullable: %t → %t\n", oldField.IsNullable, newField.IsNullable)
	}

	// Unique changed
	if oldField.IsUnique != newField.IsUnique {
		fmt.Fprintf(w, "\t\t- Unique: %t → %t\n", oldField.IsUnique, newField.IsUnique)
	}

	// Default value changed
	oldDefault := ""
	if oldField.DefaultValue != nil {
		oldDefault = *oldField.DefaultValue
	}
	newDefault := ""
	if newField.DefaultValue != nil {
		newDefault = *newField.DefaultValue
	}
	if oldDefault != newDefault {
		fmt.Fprintf(w, "\t\t- Default: %s → %s\n", oldDefault, newDefault)
	}

	// Comment changed
	if oldField.Comment != newField.Comment {
		fmt.Fprintf(w, "\t\t- Comment: %s → %s\n", oldField.Comment, newField.Comment)
	}
}

// describeFieldChangesWithIndent describes the specific changes between two field versions with proper indent
func describeFieldChangesWithIndent(w io.Writer, oldField, newField *schema.Field) {
	if oldField == nil || newField == nil {
		fmt.Fprintf(w, "\t\t\tError: field comparison invalid (one field is nil)\n")
		return
	}

	// Data type changed
	if oldField.DataType != newField.DataType {
		fmt.Fprintf(w, "\t\t\t- Type: %s → %s\n", oldField.DataType, newField.DataType)
	}

	// Size changed
	if oldField.Size != newField.Size {
		fmt.Fprintf(w, "\t\t\t- Size: %d → %d\n", oldField.Size, newField.Size)
	}

	// Precision changed
	if oldField.Precision != newField.Precision {
		fmt.Fprintf(w, "\t\t\t- Precision: %d → %d\n", oldField.Precision, newField.Precision)
	}

	// Scale changed
	if oldField.Scale != newField.Scale {
		fmt.Fprintf(w, "\t\t\t- Scale: %d → %d\n", oldField.Scale, newField.Scale)
	}

	// Primary key changed
	if oldField.IsPrimaryKey != newField.IsPrimaryKey {
		fmt.Fprintf(w, "\t\t\t- Primary Key: %t → %t\n", oldField.IsPrimaryKey, newField.IsPrimaryKey)
	}

	// Auto increment changed
	if oldField.AutoIncrement != newField.AutoIncrement {
		fmt.Fprintf(w, "\t\t\t- Auto Increment: %t → %t\n", oldField.AutoIncrement, newField.AutoIncrement)
	}

	// Nullable changed
	if oldField.IsNullable != newField.IsNullable {
		fmt.Fprintf(w, "\t\t\t- Nullable: %t → %t\n", oldField.IsNullable, newField.IsNullable)
	}

	// Unique changed
	if oldField.IsUnique != newField.IsUnique {
		fmt.Fprintf(w, "\t\t\t- Unique: %t → %t\n", oldField.IsUnique, newField.IsUnique)
	}

	// Default value changed
	oldDefault := ""
	if oldField.DefaultValue != nil {
		oldDefault = *oldField.DefaultValue
	}
	newDefault := ""
	if newField.DefaultValue != nil {
		newDefault = *newField.DefaultValue
	}
	if oldDefault != newDefault {
		fmt.Fprintf(w, "\t\t\t- Default: %s → %s\n", oldDefault, newDefault)
	}

	// Comment changed
	if oldField.Comment != newField.Comment {
		fmt.Fprintf(w, "\t\t\t- Comment: %s → %s\n", oldField.Comment, newField.Comment)
	}
}
