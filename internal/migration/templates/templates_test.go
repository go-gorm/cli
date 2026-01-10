package templates

import (
	"strings"
	"testing"
)

func TestRenderMigrationFile(t *testing.T) {
	content, err := RenderMigrationFile("20240101120000_create_users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []string{
		"package main",
		"gorm.io/cli/gorm/migration",
		"20240101120000_create_users",
		"Up: func(tx *gorm.DB)",
		"Down: func(tx *gorm.DB)",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("rendered content should contain %q", check)
		}
	}
}

func TestRenderModelStruct(t *testing.T) {
	content, err := RenderModelStruct("User", "users", "\tID uint64 `gorm:\"primaryKey\"`\n\tName string")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []string{
		"type User struct",
		"ID uint64",
		"Name string",
		"func (User) TableName()",
		`return "users"`,
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("rendered content should contain %q, got:\n%s", check, content)
		}
	}
}

func TestRenderModelFile(t *testing.T) {
	content, err := RenderModelFile("models", "users", "User", "\tID uint64", []string{"time"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []string{
		"package models",
		`"time"`,
		"type User struct",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("rendered content should contain %q, got:\n%s", check, content)
		}
	}
}

func TestRenderDiffFile(t *testing.T) {
	data := struct {
		Imports []struct {
			Alias string
			Path  string
		}
		Targets []struct {
			Alias    string
			TypeName string
		}
	}{
		Imports: []struct {
			Alias string
			Path  string
		}{
			{Alias: "pkg0", Path: "example.com/models"},
		},
		Targets: []struct {
			Alias    string
			TypeName string
		}{
			{Alias: "pkg0", TypeName: "User"},
		},
	}

	content, err := RenderDiffFile(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []string{
		"package main",
		"migration.RegisterDiffModels",
		"&pkg0.User{}",
	}

	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("rendered content should contain %q, got:\n%s", check, content)
		}
	}
}
