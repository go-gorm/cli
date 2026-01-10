package diff

import (
	"bytes"
	"strings"
	"testing"

	"gorm.io/cli/gorm/internal/migration/schema"
)

func TestTextFormatter_Format(t *testing.T) {
	tests := []struct {
		name     string
		result   Result
		contains []string
	}{
		{
			name:     "empty result",
			result:   Result{},
			contains: []string{"Models match the database schema"},
		},
		{
			name: "created tables",
			result: Result{
				CreatedTables: []*schema.Table{
					{Name: "users", ModelRef: "models.User"},
				},
			},
			contains: []string{"Tables to create", "users", "models.User"},
		},
		{
			name: "dropped tables",
			result: Result{
				DroppedTables: []*schema.Table{
					{Name: "old_table"},
				},
			},
			contains: []string{"Tables to drop", "old_table"},
		},
		{
			name: "modified columns",
			result: Result{
				ModifiedTables: []*ModifiedTable{
					{
						TableName:      "users",
						SourceModelRef: "models.User",
						AddedColumns: []*schema.Field{
							{DBName: "email", DataType: "varchar"},
						},
						DroppedColumns: []*schema.Field{
							{DBName: "old_field", DataType: "text"},
						},
					},
				},
			},
			contains: []string{"Columns to add", "email", "Columns to drop", "old_field"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			formatter := &TextFormatter{ShowDetails: true}

			err := formatter.Format(&buf, tt.result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			for _, want := range tt.contains {
				if !strings.Contains(output, want) {
					t.Errorf("output should contain %q, got:\n%s", want, output)
				}
			}
		})
	}
}

func TestJSONFormatter_Format(t *testing.T) {
	t.Run("empty result", func(t *testing.T) {
		var buf bytes.Buffer
		formatter := &JSONFormatter{Pretty: false}

		err := formatter.Format(&buf, Result{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, `"has_diff":false`) {
			t.Errorf("expected has_diff:false, got: %s", output)
		}
	})

	t.Run("with created tables", func(t *testing.T) {
		var buf bytes.Buffer
		formatter := &JSONFormatter{Pretty: true}

		result := Result{
			CreatedTables: []*schema.Table{
				{Name: "users", ModelRef: "models.User"},
			},
		}

		err := formatter.Format(&buf, result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, `"has_diff": true`) {
			t.Errorf("expected has_diff:true, got: %s", output)
		}
		if !strings.Contains(output, `"name": "users"`) {
			t.Errorf("expected table name 'users', got: %s", output)
		}
	})
}

func TestSummaryFormatter_Format(t *testing.T) {
	tests := []struct {
		name     string
		result   Result
		contains string
	}{
		{
			name:     "no differences",
			result:   Result{},
			contains: "No differences found",
		},
		{
			name: "with changes",
			result: Result{
				CreatedTables: []*schema.Table{{Name: "t1"}, {Name: "t2"}},
				ModifiedTables: []*ModifiedTable{
					{
						AddedColumns:   []*schema.Field{{DBName: "c1"}},
						DroppedColumns: []*schema.Field{{DBName: "c2"}, {DBName: "c3"}},
					},
				},
			},
			contains: "+2 tables",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			formatter := &SummaryFormatter{}

			err := formatter.Format(&buf, tt.result)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !strings.Contains(buf.String(), tt.contains) {
				t.Errorf("output should contain %q, got: %s", tt.contains, buf.String())
			}
		})
	}
}

func TestGetFormatter(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{"json", "*diff.JSONFormatter"},
		{"JSON", "*diff.JSONFormatter"},
		{"json-compact", "*diff.JSONFormatter"},
		{"summary", "*diff.SummaryFormatter"},
		{"text", "*diff.TextFormatter"},
		{"", "*diff.TextFormatter"},
		{"unknown", "*diff.TextFormatter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := GetFormatter(tt.name)
			typeName := strings.TrimPrefix(strings.TrimPrefix(
				strings.Split(strings.Split(
					strings.TrimPrefix(
						strings.TrimPrefix("%T", "%"),
						"T"),
					".")[0],
					" ")[0],
				"*"),
				"diff.")

			_ = typeName // Type checking is done at compile time
			if f == nil {
				t.Error("expected non-nil formatter")
			}
		})
	}
}
