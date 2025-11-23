package adapter

import (
	"fmt"
	"io"
	"maps"
	"strings"

	"gorm.io/gorm/schema"
)

func (a *DBAdapter) loadSchemaDiff() (SchemaDiffResult, map[string]*TableSchema, map[string]*TableSchema, error) {
	models, err := a.collectModelSchemas()
	if err != nil {
		return SchemaDiffResult{}, nil, nil, err
	}
	dbSchemas, err := a.snapshotDatabase()
	if err != nil {
		return SchemaDiffResult{}, nil, nil, err
	}
	return diffSchemas(models, dbSchemas), models, dbSchemas, nil
}

type ModifiedColumn struct {
	Old *schema.Field
	New *schema.Field
}

type ModifiedTable struct {
	TableName          string
	Model              *ModelRef
	AddedColumns       []*schema.Field
	DroppedColumns     []*schema.Field
	ModifiedColumns    []*ModifiedColumn
	AddedIndexes       []*schema.Index
	DroppedIndexes     []*schema.Index
	AddedConstraints   []*schema.Constraint
	DroppedConstraints []*schema.Constraint
}

type SchemaDiffResult struct {
	CreatedTables  []*TableSchema
	DroppedTables  []*TableSchema
	ModifiedTables []*ModifiedTable
}

func (r SchemaDiffResult) Empty() bool {
	return len(r.CreatedTables) == 0 && len(r.DroppedTables) == 0 && len(r.ModifiedTables) == 0
}

func diffSchemas(models, db map[string]*TableSchema) SchemaDiffResult {
	var result SchemaDiffResult
	seen := make(map[string]struct{})
	for tableName, modelTable := range models {
		seen[tableName] = struct{}{}
		dbTable := db[tableName]
		if dbTable == nil {
			result.CreatedTables = append(result.CreatedTables, modelTable)
			continue
		}
		if modified := diffTable(modelTable, dbTable); modified != nil {
			result.ModifiedTables = append(result.ModifiedTables, modified)
		}
	}
	for tableName, dbTable := range db {
		if _, ok := seen[tableName]; ok {
			continue
		}
		result.DroppedTables = append(result.DroppedTables, dbTable)
	}
	return result
}

func diffTable(model, db *TableSchema) *ModifiedTable {
	addedCols, droppedCols, modifiedCols := diffColumns(model.Schema, db.Schema)
	addedIdx, droppedIdx := diffIndexes(model.Indexes, db.Indexes)
	if len(addedCols) == 0 && len(droppedCols) == 0 && len(modifiedCols) == 0 && len(addedIdx) == 0 && len(droppedIdx) == 0 {
		return nil
	}
	return &ModifiedTable{
		TableName:       model.Schema.Table,
		Model:           model.Model,
		AddedColumns:    addedCols,
		DroppedColumns:  droppedCols,
		ModifiedColumns: modifiedCols,
		AddedIndexes:    addedIdx,
		DroppedIndexes:  droppedIdx,
	}
}

func diffColumns(model, db *schema.Schema) (added, dropped []*schema.Field, modified []*ModifiedColumn) {
	modelFields := maps.Clone(model.FieldsByDBName)
	dbFields := maps.Clone(db.FieldsByDBName)

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
	return strings.EqualFold(a.DBName, b.DBName) &&
		a.PrimaryKey == b.PrimaryKey &&
		a.AutoIncrement == b.AutoIncrement &&
		a.DataType == b.DataType &&
		a.GORMDataType == b.GORMDataType &&
		a.Size == b.Size &&
		a.Precision == b.Precision &&
		a.Scale == b.Scale &&
		a.NotNull == b.NotNull &&
		a.Unique == b.Unique &&
		a.HasDefaultValue == b.HasDefaultValue &&
		strings.TrimSpace(strings.ToLower(a.DefaultValue)) == strings.TrimSpace(strings.ToLower(b.DefaultValue))
}

func diffIndexes(model, db []*schema.Index) (added, dropped []*schema.Index) {
	modelMap := make(map[string]*schema.Index, len(model))
	for _, idx := range model {
		modelMap[strings.ToLower(idx.Name)] = idx
	}

	dbMap := make(map[string]*schema.Index, len(db))
	for _, idx := range db {
		dbMap[strings.ToLower(idx.Name)] = idx
	}

	for name, idx := range modelMap {
		if _, ok := dbMap[name]; ok {
			delete(dbMap, name)
			continue
		}
		added = append(added, idx)
	}

	for _, idx := range dbMap {
		dropped = append(dropped, idx)
	}
	return
}

func writeSchemaDiff(w io.Writer, diff SchemaDiffResult) {
	if diff.Empty() {
		fmt.Fprintln(w, "Models match the database schema")
		return
	}
	fmt.Fprintln(w, "Model ↔ DB diff:")
	if len(diff.CreatedTables) > 0 {
		fmt.Fprintln(w, "  + Created Tables:")
		for _, table := range diff.CreatedTables {
			fmt.Fprintf(w, "    - %s\n", formatTableLabel(table))
		}
	}
	if len(diff.DroppedTables) > 0 {
		fmt.Fprintln(w, "  - Dropped Tables:")
		for _, table := range diff.DroppedTables {
			fmt.Fprintf(w, "    - %s\n", formatTableLabel(table))
		}
	}
	for _, table := range diff.ModifiedTables {
		fmt.Fprintf(w, "  ~ %s\n", formatModifiedTableHeader(table))
		writeTableChanges(w, table)
	}
}

func formatTableLabel(table *TableSchema) string {
	if table.Model != nil && table.Model.PackagePath != "" {
		return fmt.Sprintf("%s (%s.%s)", table.Schema.Table, table.Model.PackagePath, table.Model.TypeName)
	}
	return table.Schema.Table
}

func formatModifiedTableHeader(mt *ModifiedTable) string {
	if mt.Model != nil && mt.Model.PackagePath != "" {
		return fmt.Sprintf("%s (%s.%s)", mt.TableName, mt.Model.PackagePath, mt.Model.TypeName)
	}
	return mt.TableName
}

func writeTableChanges(w io.Writer, mt *ModifiedTable) {
	if len(mt.AddedColumns) > 0 {
		fmt.Fprintf(w, "    + Columns: %s\n", joinFieldNames(mt.AddedColumns))
	}
	if len(mt.DroppedColumns) > 0 {
		fmt.Fprintf(w, "    - Columns: %s\n", joinFieldNames(mt.DroppedColumns))
	}
	if len(mt.ModifiedColumns) > 0 {
		changes := make([]string, 0, len(mt.ModifiedColumns))
		for _, col := range mt.ModifiedColumns {
			changes = append(changes, col.New.DBName)
		}
		if len(changes) > 0 {
			fmt.Fprintf(w, "    ~ Columns: %s\n", strings.Join(changes, ", "))
		}
	}
	if len(mt.AddedIndexes) > 0 {
		fmt.Fprintf(w, "    + Indexes: %s\n", joinIndexNames(mt.AddedIndexes))
	}
	if len(mt.DroppedIndexes) > 0 {
		fmt.Fprintf(w, "    - Indexes: %s\n", joinIndexNames(mt.DroppedIndexes))
	}
}

func joinFieldNames(fields []*schema.Field) string {
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		names = append(names, field.DBName)
	}
	return strings.Join(names, ", ")
}

func joinIndexNames(indexes []*schema.Index) string {
	names := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		names = append(names, idx.Name)
	}
	return strings.Join(names, ", ")
}
