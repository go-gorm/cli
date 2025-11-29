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
	models = a.applyDiffTableRules(models)
	dbSchemas, err := a.snapshotDatabase()
	if err != nil {
		return SchemaDiffResult{}, nil, nil, err
	}
	dbSchemas = a.applyDiffTableRules(dbSchemas)
	return diffSchemas(models, dbSchemas), models, dbSchemas, nil
}

func (a *DBAdapter) applyDiffTableRules(schemas map[string]*TableSchema) map[string]*TableSchema {
	if len(a.cfg.TableRules) == 0 || len(schemas) == 0 {
		return schemas
	}
	filtered := make(map[string]*TableSchema, len(schemas))
	for _, table := range schemas {
		if table == nil || table.Schema == nil || table.Schema.Table == "" {
			continue
		}
		cfg, include := buildConfigForTable(table.Schema.Table, a.cfg.TableRules)
		if !include {
			continue
		}
		filteredSchema := filterTableSchemaForDiff(table, cfg.FieldRules)
		if filteredSchema == nil {
			continue
		}
		filtered[strings.ToLower(filteredSchema.Schema.Table)] = filteredSchema
	}
	return filtered
}

func filterTableSchemaForDiff(table *TableSchema, rules []FieldRule) *TableSchema {
	if table == nil || table.Schema == nil {
		return nil
	}
	excluded := identifyExcludedColumns(table.Schema.Table, table.Schema.Fields, rules)
	if len(excluded) == 0 {
		return table
	}
	newSchema := *table.Schema
	newSchema.Fields = make([]*schema.Field, 0, len(table.Schema.Fields)-len(excluded))
	newSchema.FieldsByName = make(map[string]*schema.Field, len(table.Schema.Fields)-len(excluded))
	newSchema.FieldsByDBName = make(map[string]*schema.Field, len(table.Schema.Fields)-len(excluded))
	newSchema.DBNames = make([]string, 0, len(table.Schema.DBNames))
	newSchema.PrimaryFields = nil
	newSchema.PrimaryFieldDBNames = nil
	newSchema.PrioritizedPrimaryField = nil
	for _, field := range table.Schema.Fields {
		if _, skip := excluded[field.DBName]; skip {
			continue
		}
		newSchema.Fields = append(newSchema.Fields, field)
		newSchema.FieldsByName[field.Name] = field
		newSchema.FieldsByDBName[field.DBName] = field
		newSchema.DBNames = append(newSchema.DBNames, field.DBName)
		if field.PrimaryKey {
			newSchema.PrimaryFields = append(newSchema.PrimaryFields, field)
			newSchema.PrimaryFieldDBNames = append(newSchema.PrimaryFieldDBNames, field.DBName)
			if newSchema.PrioritizedPrimaryField == nil {
				newSchema.PrioritizedPrimaryField = field
			}
		}
	}
	if len(newSchema.Fields) == 0 {
		return nil
	}
	filtered := &TableSchema{
		Schema:      &newSchema,
		Model:       table.Model,
		Indexes:     filterIndexesForDiff(table.Indexes, excluded),
		Constraints: filterConstraintsForDiff(table.Constraints, excluded),
	}
	return filtered
}

func identifyExcludedColumns(table string, fields []*schema.Field, rules []FieldRule) map[string]struct{} {
	if len(rules) == 0 {
		return nil
	}
	excluded := make(map[string]struct{})
	for _, field := range fields {
		if field == nil {
			continue
		}
		if rule, ok := matchFieldRule(rules, table, field.DBName); ok && rule.Exclude {
			excluded[field.DBName] = struct{}{}
		}
	}
	return excluded
}

func filterIndexesForDiff(indexes []*schema.Index, ignored map[string]struct{}) []*schema.Index {
	if len(indexes) == 0 || len(ignored) == 0 {
		return indexes
	}
	filtered := make([]*schema.Index, 0, len(indexes))
	for _, idx := range indexes {
		if idx == nil {
			continue
		}
		newIdx := *idx
		newIdx.Fields = nil
		for _, option := range idx.Fields {
			if option.Field != nil {
				if _, skip := ignored[option.Field.DBName]; skip {
					continue
				}
			}
			newIdx.Fields = append(newIdx.Fields, option)
		}
		if len(idx.Fields) > 0 && len(newIdx.Fields) == 0 {
			continue
		}
		filtered = append(filtered, &newIdx)
	}
	return filtered
}

func filterConstraintsForDiff(constraints []*schema.Constraint, ignored map[string]struct{}) []*schema.Constraint {
	if len(constraints) == 0 || len(ignored) == 0 {
		return constraints
	}
	filtered := make([]*schema.Constraint, 0, len(constraints))
	for _, cons := range constraints {
		if cons == nil {
			continue
		}
		if constraintReferencesExcludedField(cons, ignored) {
			continue
		}
		filtered = append(filtered, cons)
	}
	return filtered
}

func constraintReferencesExcludedField(cons *schema.Constraint, ignored map[string]struct{}) bool {
	for _, field := range cons.ForeignKeys {
		if field != nil {
			if _, skip := ignored[field.DBName]; skip {
				return true
			}
		}
	}
	for _, field := range cons.References {
		if field != nil {
			if _, skip := ignored[field.DBName]; skip {
				return true
			}
		}
	}
	return false
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
		fmt.Fprintln(w, "    + Columns:")
		for _, col := range mt.AddedColumns {
			fmt.Fprintf(w, "      - %s%s\n", col.DBName, formatFieldSummary(col))
		}
	}
	if len(mt.DroppedColumns) > 0 {
		fmt.Fprintln(w, "    - Columns:")
		for _, col := range mt.DroppedColumns {
			fmt.Fprintf(w, "      - %s%s\n", col.DBName, formatFieldSummary(col))
		}
	}
	if len(mt.ModifiedColumns) > 0 {
		fmt.Fprintln(w, "    ~ Columns:")
		for _, col := range mt.ModifiedColumns {
			details := describeFieldChanges(col.Old, col.New)
			fmt.Fprintf(w, "      - %s\n", col.New.DBName)
			for _, detail := range details {
				fmt.Fprintf(w, "        %s\n", detail)
			}
		}
	}
	if len(mt.AddedIndexes) > 0 {
		fmt.Fprintf(w, "    + Indexes: %s\n", joinIndexNames(mt.AddedIndexes))
	}
	if len(mt.DroppedIndexes) > 0 {
		fmt.Fprintf(w, "    - Indexes: %s\n", joinIndexNames(mt.DroppedIndexes))
	}
}

func joinIndexNames(indexes []*schema.Index) string {
	names := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		names = append(names, idx.Name)
	}
	return strings.Join(names, ", ")
}

func formatFieldSummary(field *schema.Field) string {
	if field == nil {
		return ""
	}
	parts := []string{}
	if t := describeFieldType(field); t != "" {
		parts = append(parts, t)
	}
	if field.PrimaryKey {
		parts = append(parts, "PRIMARY KEY")
	}
	if field.AutoIncrement {
		parts = append(parts, "AUTO INCREMENT")
	}
	if field.NotNull {
		parts = append(parts, "NOT NULL")
	}
	if field.Unique {
		parts = append(parts, "UNIQUE")
	}
	if field.HasDefaultValue {
		parts = append(parts, fmt.Sprintf("DEFAULT %s", describeDefaultValue(field)))
	}
	if len(parts) == 0 {
		return ""
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

func describeFieldChanges(oldField, newField *schema.Field) []string {
	if oldField == nil || newField == nil {
		return nil
	}
	var details []string
	oldType := describeFieldType(oldField)
	newType := describeFieldType(newField)
	if oldType != newType {
		details = append(details, fmt.Sprintf("type: %s -> %s", oldType, newType))
	}
	if oldField.PrimaryKey != newField.PrimaryKey {
		details = append(details, boolChange("primary key", oldField.PrimaryKey, newField.PrimaryKey))
	}
	if oldField.AutoIncrement != newField.AutoIncrement {
		details = append(details, boolChange("auto increment", oldField.AutoIncrement, newField.AutoIncrement))
	}
	if oldField.NotNull != newField.NotNull {
		details = append(details, boolChange("not null", oldField.NotNull, newField.NotNull))
	}
	if oldField.Unique != newField.Unique {
		details = append(details, boolChange("unique", oldField.Unique, newField.Unique))
	}
	oldDefault := describeDefaultValue(oldField)
	newDefault := describeDefaultValue(newField)
	if oldField.HasDefaultValue != newField.HasDefaultValue || oldDefault != newDefault {
		details = append(details, fmt.Sprintf("default: %s -> %s", oldDefault, newDefault))
	}
	return details
}

func boolChange(label string, oldVal, newVal bool) string {
	return fmt.Sprintf("%s: %t -> %t", label, oldVal, newVal)
}

func describeFieldType(field *schema.Field) string {
	if field == nil {
		return ""
	}
	base := string(field.DataType)
	switch {
	case field.Precision > 0 && field.Scale > 0:
		base = fmt.Sprintf("%s(%d,%d)", base, field.Precision, field.Scale)
	case field.Size > 0:
		base = fmt.Sprintf("%s(%d)", base, field.Size)
	}
	return strings.TrimSpace(base)
}

func describeDefaultValue(field *schema.Field) string {
	if field == nil || !field.HasDefaultValue {
		return "<none>"
	}
	if val := strings.TrimSpace(field.DefaultValue); val != "" {
		return val
	}
	return "<empty>"
}
