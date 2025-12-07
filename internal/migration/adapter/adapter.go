package adapter

import (
	"io"

	"gorm.io/gorm"
)

// Migration represents a named schema change.
type Migration struct {
	Name string
	Up   func(tx *gorm.DB) error
	Down func(tx *gorm.DB) error
}

// Adapter describes the contract used by migrations/main.go.
type Adapter interface {
	Up(UpOptions) error
	Down(DownOptions) error
	Status(StatusOptions) error
	Diff(DiffOptions) error
	RegisterMigrations([]Migration)
	GenerateModel(GenerateModelOptions) error
	GenerateMigration(GenerateMigrationOptions) error
}

// UpOptions controls how many migrations to apply.
type UpOptions struct {
	Limit int
}

// DownOptions controls how many migrations to rollback.
type DownOptions struct {
	Steps int
}

// StatusOptions currently holds no fields; defined for future extension.
type StatusOptions struct{}

// DiffOptions controls diff output behaviour.
type DiffOptions struct {
	GeneratedFile bool
	Writer        io.Writer
}

// GenerateModelOptions drives DBAdapter.GenerateModel.
type GenerateModelOptions struct {
	DryRun      bool
	AutoApprove bool
}

// GenerateMigrationOptions drives DBAdapter.GenerateMigration.
type GenerateMigrationOptions struct {
	Name        string
	DryRun      bool
	AutoApprove bool
}

type FieldRule struct {
	Pattern   string
	FieldName string
	FieldType string
	Tags      map[string]string
	Imports   []string
	Exclude   bool
}

type TableConfig struct {
	OutputPath string
	FieldRules []FieldRule
}

// TableRule describes a table-matching rule and associated configuration.
type TableRule struct {
	Pattern string
	Config  TableConfig
	Exclude bool
}

// Config configures the DBAdapter.
type Config struct {
	ModelsDir     string
	MigrationsDir string
	TableRules    []TableRule
	DiffModels    []any
}
