package diff

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gorm.io/cli/gorm/internal/migration/schema"
)

// Formatter defines the interface for formatting diff results.
type Formatter interface {
	Format(w io.Writer, result Result) error
}

// TextFormatter outputs diff results as plain text.
type TextFormatter struct {
	ShowDetails bool
}

// Format writes the diff result as plain text.
func (f *TextFormatter) Format(w io.Writer, result Result) error {
	if result.Empty() {
		fmt.Fprintln(w, "Models match the database schema")
		return nil
	}

	fmt.Fprintln(w, "Model ↔ DB diff:")

	if len(result.CreatedTables) > 0 {
		fmt.Fprintln(w, "\t+ Tables to create:")
		for _, table := range result.CreatedTables {
			fmt.Fprintf(w, "\t\t- %s", table.Name)
			if table.ModelRef != "" {
				fmt.Fprintf(w, " (%s)", table.ModelRef)
			}
			fmt.Fprintln(w)
		}
	}

	if len(result.DroppedTables) > 0 {
		fmt.Fprintln(w, "\t- Tables to drop:")
		for _, table := range result.DroppedTables {
			fmt.Fprintf(w, "\t\t- %s\n", table.Name)
		}
	}

	for _, table := range result.ModifiedTables {
		fmt.Fprintf(w, "\t~ %s", table.TableName)
		if table.SourceModelRef != "" {
			fmt.Fprintf(w, " (%s)", table.SourceModelRef)
		}
		fmt.Fprintln(w)

		if len(table.AddedColumns) > 0 {
			fmt.Fprintln(w, "\t\t+ Columns to add:")
			for _, col := range table.AddedColumns {
				fmt.Fprintf(w, "\t\t\t- %s (%s)\n", col.DBName, describeFieldDetails(col))
			}
		}

		if len(table.DroppedColumns) > 0 {
			fmt.Fprintln(w, "\t\t- Columns to drop:")
			for _, col := range table.DroppedColumns {
				fmt.Fprintf(w, "\t\t\t- %s (%s)\n", col.DBName, describeFieldDetails(col))
			}
		}

		if len(table.ModifiedColumns) > 0 {
			fmt.Fprintln(w, "\t\t~ Columns to modify:")
			for _, mod := range table.ModifiedColumns {
				fmt.Fprintf(w, "\t\t\t- %s:\n", mod.New.DBName)
				f.writeColumnChanges(w, mod.Old, mod.New)
			}
		}
	}

	return nil
}

func (f *TextFormatter) writeColumnChanges(w io.Writer, old, new *schema.Field) {
	if old == nil || new == nil {
		return
	}

	if old.DataType != new.DataType {
		fmt.Fprintf(w, "\t\t\t\t- Type: %s → %s\n", old.DataType, new.DataType)
	}
	if old.Size != new.Size {
		fmt.Fprintf(w, "\t\t\t\t- Size: %d → %d\n", old.Size, new.Size)
	}
	if old.Precision != new.Precision {
		fmt.Fprintf(w, "\t\t\t\t- Precision: %d → %d\n", old.Precision, new.Precision)
	}
	if old.Scale != new.Scale {
		fmt.Fprintf(w, "\t\t\t\t- Scale: %d → %d\n", old.Scale, new.Scale)
	}
	if old.IsPrimaryKey != new.IsPrimaryKey {
		fmt.Fprintf(w, "\t\t\t\t- Primary Key: %t → %t\n", old.IsPrimaryKey, new.IsPrimaryKey)
	}
	if old.AutoIncrement != new.AutoIncrement {
		fmt.Fprintf(w, "\t\t\t\t- Auto Increment: %t → %t\n", old.AutoIncrement, new.AutoIncrement)
	}
	if old.IsNullable != new.IsNullable {
		fmt.Fprintf(w, "\t\t\t\t- Nullable: %t → %t\n", old.IsNullable, new.IsNullable)
	}
	if old.IsUnique != new.IsUnique {
		fmt.Fprintf(w, "\t\t\t\t- Unique: %t → %t\n", old.IsUnique, new.IsUnique)
	}

	oldDefault, newDefault := "", ""
	if old.DefaultValue != nil {
		oldDefault = *old.DefaultValue
	}
	if new.DefaultValue != nil {
		newDefault = *new.DefaultValue
	}
	if oldDefault != newDefault {
		fmt.Fprintf(w, "\t\t\t\t- Default: %q → %q\n", oldDefault, newDefault)
	}
}

// JSONFormatter outputs diff results as JSON.
type JSONFormatter struct {
	Pretty bool
}

// JSONDiffResult is the JSON representation of a diff result.
type JSONDiffResult struct {
	HasDiff        bool                `json:"has_diff"`
	CreatedTables  []JSONTable         `json:"created_tables,omitempty"`
	DroppedTables  []JSONTable         `json:"dropped_tables,omitempty"`
	ModifiedTables []JSONModifiedTable `json:"modified_tables,omitempty"`
}

// JSONTable represents a table in JSON format.
type JSONTable struct {
	Name     string      `json:"name"`
	ModelRef string      `json:"model_ref,omitempty"`
	Fields   []JSONField `json:"fields,omitempty"`
}

// JSONField represents a field in JSON format.
type JSONField struct {
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Size          int     `json:"size,omitempty"`
	Precision     int     `json:"precision,omitempty"`
	Scale         int     `json:"scale,omitempty"`
	IsPrimaryKey  bool    `json:"is_primary_key,omitempty"`
	IsNullable    bool    `json:"is_nullable,omitempty"`
	IsUnique      bool    `json:"is_unique,omitempty"`
	AutoIncrement bool    `json:"auto_increment,omitempty"`
	DefaultValue  *string `json:"default_value,omitempty"`
}

// JSONModifiedTable represents a modified table in JSON format.
type JSONModifiedTable struct {
	Name            string               `json:"name"`
	ModelRef        string               `json:"model_ref,omitempty"`
	AddedColumns    []JSONField          `json:"added_columns,omitempty"`
	DroppedColumns  []JSONField          `json:"dropped_columns,omitempty"`
	ModifiedColumns []JSONModifiedColumn `json:"modified_columns,omitempty"`
}

// JSONModifiedColumn represents a modified column in JSON format.
type JSONModifiedColumn struct {
	Name string    `json:"name"`
	Old  JSONField `json:"old"`
	New  JSONField `json:"new"`
}

// Format writes the diff result as JSON.
func (f *JSONFormatter) Format(w io.Writer, result Result) error {
	jsonResult := JSONDiffResult{
		HasDiff: !result.Empty(),
	}

	for _, table := range result.CreatedTables {
		jsonResult.CreatedTables = append(jsonResult.CreatedTables, tableToJSON(table))
	}

	for _, table := range result.DroppedTables {
		jsonResult.DroppedTables = append(jsonResult.DroppedTables, tableToJSON(table))
	}

	for _, mod := range result.ModifiedTables {
		jsonMod := JSONModifiedTable{
			Name:     mod.TableName,
			ModelRef: mod.SourceModelRef,
		}
		for _, col := range mod.AddedColumns {
			jsonMod.AddedColumns = append(jsonMod.AddedColumns, fieldToJSON(col))
		}
		for _, col := range mod.DroppedColumns {
			jsonMod.DroppedColumns = append(jsonMod.DroppedColumns, fieldToJSON(col))
		}
		for _, col := range mod.ModifiedColumns {
			jsonMod.ModifiedColumns = append(jsonMod.ModifiedColumns, JSONModifiedColumn{
				Name: col.New.DBName,
				Old:  fieldToJSON(col.Old),
				New:  fieldToJSON(col.New),
			})
		}
		jsonResult.ModifiedTables = append(jsonResult.ModifiedTables, jsonMod)
	}

	var enc *json.Encoder
	if f.Pretty {
		enc = json.NewEncoder(w)
		enc.SetIndent("", "  ")
	} else {
		enc = json.NewEncoder(w)
	}

	return enc.Encode(jsonResult)
}

func tableToJSON(table *schema.Table) JSONTable {
	jt := JSONTable{
		Name:     table.Name,
		ModelRef: table.ModelRef,
	}
	for _, field := range table.Fields {
		jt.Fields = append(jt.Fields, fieldToJSON(field))
	}
	return jt
}

func fieldToJSON(field *schema.Field) JSONField {
	return JSONField{
		Name:          field.DBName,
		Type:          field.DataType,
		Size:          field.Size,
		Precision:     field.Precision,
		Scale:         field.Scale,
		IsPrimaryKey:  field.IsPrimaryKey,
		IsNullable:    field.IsNullable,
		IsUnique:      field.IsUnique,
		AutoIncrement: field.AutoIncrement,
		DefaultValue:  field.DefaultValue,
	}
}

// SummaryFormatter outputs a brief summary of differences.
type SummaryFormatter struct{}

// Format writes a brief summary of the diff.
func (f *SummaryFormatter) Format(w io.Writer, result Result) error {
	if result.Empty() {
		fmt.Fprintln(w, "✓ No differences found")
		return nil
	}

	var parts []string

	if n := len(result.CreatedTables); n > 0 {
		parts = append(parts, fmt.Sprintf("+%d tables", n))
	}
	if n := len(result.DroppedTables); n > 0 {
		parts = append(parts, fmt.Sprintf("-%d tables", n))
	}

	addedCols, droppedCols, modifiedCols := 0, 0, 0
	for _, mod := range result.ModifiedTables {
		addedCols += len(mod.AddedColumns)
		droppedCols += len(mod.DroppedColumns)
		modifiedCols += len(mod.ModifiedColumns)
	}

	if addedCols > 0 {
		parts = append(parts, fmt.Sprintf("+%d columns", addedCols))
	}
	if droppedCols > 0 {
		parts = append(parts, fmt.Sprintf("-%d columns", droppedCols))
	}
	if modifiedCols > 0 {
		parts = append(parts, fmt.Sprintf("~%d columns", modifiedCols))
	}

	fmt.Fprintf(w, "⚠ Differences: %s\n", strings.Join(parts, ", "))
	return nil
}

// GetFormatter returns a formatter by name.
func GetFormatter(name string) Formatter {
	switch strings.ToLower(name) {
	case "json":
		return &JSONFormatter{Pretty: true}
	case "json-compact":
		return &JSONFormatter{Pretty: false}
	case "summary":
		return &SummaryFormatter{}
	default:
		return &TextFormatter{ShowDetails: true}
	}
}
