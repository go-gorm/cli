package adapter

import (
	"fmt"
	"io"
	"maps"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var regFullDataType = regexp.MustCompile(`\D*(\d+)\D?`)

const ignoreIndexDiffs = true

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
	return diffSchemas(models, dbSchemas, a.columnComparator()), models, dbSchemas, nil
}

func (a *DBAdapter) applyDiffTableRules(schemas map[string]*TableSchema) map[string]*TableSchema {
	if len(a.cfg.TableRules) == 0 || len(schemas) == 0 {
		return schemas
	}
	resolver := newTableRuleResolver(a.cfg.TableRules)
	filtered := make(map[string]*TableSchema, len(schemas))
	for _, table := range schemas {
		if table == nil || table.Schema == nil || table.Schema.Table == "" {
			continue
		}
		cfg, include := resolver.ConfigForTable(table.Schema.Table)
		if !include {
			continue
		}
		filteredSchema := applyFieldRulesToSchema(table, newFieldRuleMatcher(cfg.FieldRules))
		if filteredSchema == nil {
			continue
		}
		filtered[strings.ToLower(filteredSchema.Schema.Table)] = filteredSchema
	}
	return filtered
}

type columnComparator func(modelField, dbField *schema.Field, columnType gorm.ColumnType) bool

func (a *DBAdapter) columnComparator() columnComparator {
	if a == nil || a.db == nil {
		return nil
	}
	return func(modelField, _ *schema.Field, columnType gorm.ColumnType) bool {
		if modelField == nil {
			return columnType == nil
		}
		if columnType == nil {
			return false
		}
		return !columnNeedsMigration(a.db, modelField, columnType)
	}
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

func diffSchemas(models, db map[string]*TableSchema, cmp columnComparator) SchemaDiffResult {
	var result SchemaDiffResult
	seen := make(map[string]struct{})
	for tableName, modelTable := range models {
		seen[tableName] = struct{}{}
		dbTable := db[tableName]
		if dbTable == nil {
			result.CreatedTables = append(result.CreatedTables, modelTable)
			continue
		}
		if modified := diffTable(modelTable, dbTable, cmp); modified != nil {
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

func diffTable(model, db *TableSchema, cmp columnComparator) *ModifiedTable {
	addedCols, droppedCols, modifiedCols := diffColumns(model, db, cmp)
	var addedIdx, droppedIdx []*schema.Index
	if !ignoreIndexDiffs {
		modelIndexes := model.Indexes
		dbIndexes := db.Indexes
		if fkCols := foreignKeyColumnSet(model); len(fkCols) > 0 {
			modelIndexes = excludeForeignKeyIndexes(modelIndexes, fkCols)
			dbIndexes = excludeForeignKeyIndexes(dbIndexes, fkCols)
		}
		addedIdx, droppedIdx = diffIndexes(modelIndexes, dbIndexes)
	}
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

func diffColumns(model, db *TableSchema, cmp columnComparator) (added, dropped []*schema.Field, modified []*ModifiedColumn) {
	modelFields := maps.Clone(model.Schema.FieldsByDBName)
	dbFields := maps.Clone(db.Schema.FieldsByDBName)

	for key, field := range modelFields {
		dbField, ok := dbFields[key]
		if !ok {
			added = append(added, field)
			continue
		}
		columnType := db.columnTypeForField(dbField)
		fieldForCompare := prepareFieldForComparison(field, columnType)
		if cmp != nil {
			if !cmp(fieldForCompare, dbField, columnType) {
				modified = append(modified, &ModifiedColumn{Old: dbField, New: field})
			}
		} else if !fieldsEqual(field, dbField) {
			modified = append(modified, &ModifiedColumn{Old: dbField, New: field})
		}
		delete(dbFields, key)
	}
	for _, field := range dbFields {
		dropped = append(dropped, field)
	}
	return
}

func prepareFieldForComparison(field *schema.Field, columnType gorm.ColumnType) *schema.Field {
	if field == nil {
		return nil
	}
	clone := *field
	if field.TagSettings != nil {
		clone.TagSettings = make(map[string]string, len(field.TagSettings))
		for k, v := range field.TagSettings {
			clone.TagSettings[k] = v
		}
	} else {
		clone.TagSettings = make(map[string]string)
	}
	if columnType != nil {
		applyColumnTypeDefaults(&clone, columnType)
	}
	return &clone
}

func applyColumnTypeDefaults(field *schema.Field, columnType gorm.ColumnType) {
	if field == nil || columnType == nil {
		return
	}
	typeExpr := columnTypeExpression(columnType)
	if typeExpr != "" {
		field.TagSettings["TYPE"] = typeExpr
		base := typeExpr
		if idx := strings.Index(base, "("); idx >= 0 {
			base = base[:idx]
		}
		base = strings.TrimSpace(base)
		if base != "" {
			field.DataType = schema.DataType(base)
			field.GORMDataType = field.DataType
		}
	}
	if precision, scale, ok := columnType.DecimalSize(); ok {
		if precision > 0 {
			field.Precision = int(precision)
		}
		if scale > 0 {
			field.Scale = int(scale)
		}
	} else if length, ok := columnType.Length(); ok && length > 0 {
		field.Size = int(length)
	}
	if nullable, ok := columnType.Nullable(); ok {
		if field.FieldType != nil && !fieldTypeAllowsNull(field.FieldType) {
			field.NotNull = !nullable
		}
	}
}

func columnTypeExpression(columnType gorm.ColumnType) string {
	if columnType == nil {
		return ""
	}
	if ct, ok := columnType.ColumnType(); ok {
		if expr := strings.ToLower(strings.TrimSpace(ct)); expr != "" {
			return expr
		}
	}
	dbType := strings.ToLower(strings.TrimSpace(columnType.DatabaseTypeName()))
	if dbType == "" {
		return ""
	}
	if precision, scale, ok := columnType.DecimalSize(); ok && precision > 0 {
		if scale > 0 {
			return fmt.Sprintf("%s(%d,%d)", dbType, precision, scale)
		}
		return fmt.Sprintf("%s(%d)", dbType, precision)
	}
	if length, ok := columnType.Length(); ok && length > 0 {
		return fmt.Sprintf("%s(%d)", dbType, length)
	}
	return dbType
}

func fieldTypeAllowsNull(rt reflect.Type) bool {
	if rt == nil {
		return true
	}
	switch rt.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map:
		return true
	}
	if implementsScannerOrValuer(rt) {
		return true
	}
	return false
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

func foreignKeyColumnSet(table *TableSchema) map[string]struct{} {
	if table == nil || len(table.Constraints) == 0 {
		return nil
	}
	fkCols := make(map[string]struct{})
	for _, cons := range table.Constraints {
		if cons == nil {
			continue
		}
		for _, field := range cons.ForeignKeys {
			if field == nil {
				continue
			}
			fkCols[strings.ToLower(field.DBName)] = struct{}{}
		}
	}
	if len(fkCols) == 0 {
		return nil
	}
	return fkCols
}

func excludeForeignKeyIndexes(indexes []*schema.Index, fkCols map[string]struct{}) []*schema.Index {
	if len(indexes) == 0 || len(fkCols) == 0 {
		return indexes
	}
	filtered := make([]*schema.Index, 0, len(indexes))
	for _, idx := range indexes {
		if idx == nil {
			continue
		}
		if indexOnlyUsesColumns(idx, fkCols) {
			continue
		}
		filtered = append(filtered, idx)
	}
	return filtered
}

func indexOnlyUsesColumns(idx *schema.Index, cols map[string]struct{}) bool {
	if idx == nil || len(idx.Fields) == 0 {
		return false
	}
	for _, opt := range idx.Fields {
		if opt.Field == nil {
			return false
		}
		if _, ok := cols[strings.ToLower(opt.Field.DBName)]; !ok {
			return false
		}
	}
	return true
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

func columnNeedsMigration(db *gorm.DB, field *schema.Field, columnType gorm.ColumnType) bool {
	if field == nil || columnType == nil {
		return true
	}
	if field.IgnoreMigration {
		return false
	}
	if string(field.DataType) == "" {
		return true
	}

	migrator := db.Migrator()
	// Check if field.FieldType is nil, which would cause reflect.New(nil) panic
	if field.FieldType == nil {
		return true
	}

	fullDataType := strings.TrimSpace(strings.ToLower(migrator.FullDataTypeOf(field).SQL))
	realDataType := strings.ToLower(columnType.DatabaseTypeName())
	if realDataType == "" {
		return true
	}
	var (
		alterColumn bool
		isSameType  = fullDataType == realDataType
	)

	if !field.PrimaryKey {
		if !strings.HasPrefix(fullDataType, realDataType) {
			aliases := migrator.GetTypeAliases(realDataType)
			for _, alias := range aliases {
				if strings.HasPrefix(fullDataType, alias) {
					isSameType = true
					break
				}
			}
			if !isSameType {
				alterColumn = true
			}
		}
	}

	if !isSameType {
		if length, ok := columnType.Length(); length != int64(field.Size) {
			if length > 0 && field.Size > 0 {
				alterColumn = true
			} else {
				matches2 := regFullDataType.FindAllStringSubmatch(fullDataType, -1)
				if !field.PrimaryKey &&
					(len(matches2) == 1 && matches2[0][1] != fmt.Sprint(length) && ok) {
					alterColumn = true
				}
			}
		}
	}

	if realDataType == "decimal" || (realDataType == "numeric" &&
		regexp.MustCompile(realDataType+`\(.*\)`).FindString(fullDataType) != "") {
		precision, scale, ok := columnType.DecimalSize()
		if ok {
			if !strings.HasPrefix(fullDataType, fmt.Sprintf("%s(%d,%d)", realDataType, precision, scale)) &&
				!strings.HasPrefix(fullDataType, fmt.Sprintf("%s(%d)", realDataType, precision)) {
				alterColumn = true
			}
		}
	} else {
		type dataTyper interface {
			DataTypeOf(*schema.Field) string
		}
		var dataTypeOf string
		if dt, ok := migrator.(dataTyper); ok {
			dataTypeOf = dt.DataTypeOf(field)
		} else {
			dataTypeOf = string(field.DataType)
		}
		if precision, _, ok := columnType.DecimalSize(); ok && int64(field.Precision) != precision {
			if regexp.MustCompile(fmt.Sprintf("[^0-9]%d[^0-9]", field.Precision)).MatchString(dataTypeOf) {
				alterColumn = true
			}
		}
	}

	if nullable, ok := columnType.Nullable(); ok && nullable == field.NotNull {
		if !field.PrimaryKey && !nullable {
			alterColumn = true
		}
	}

	if !field.PrimaryKey {
		currentDefaultNotNull := field.HasDefaultValue && (field.DefaultValueInterface != nil || !strings.EqualFold(field.DefaultValue, "NULL"))
		dv, dvNotNull := columnType.DefaultValue()
		if dvNotNull && !currentDefaultNotNull {
			alterColumn = true
		} else if !dvNotNull && currentDefaultNotNull {
			alterColumn = true
		} else if currentDefaultNotNull || dvNotNull {
			switch field.GORMDataType {
			case schema.Time:
				if !strings.EqualFold(strings.TrimSuffix(dv, "()"), strings.TrimSuffix(field.DefaultValue, "()")) {
					alterColumn = true
				}
			case schema.Bool:
				v1, _ := strconv.ParseBool(dv)
				v2, _ := strconv.ParseBool(field.DefaultValue)
				alterColumn = v1 != v2
			case schema.String:
				if dv != field.DefaultValue && dv != strings.Trim(field.DefaultValue, "'\"") {
					alterColumn = true
				}
			default:
				alterColumn = dv != field.DefaultValue
			}
		}
	}

	if comment, ok := columnType.Comment(); ok && comment != field.Comment {
		if !field.PrimaryKey {
			alterColumn = true
		}
	}

	return alterColumn
}
