package main

import (
	"gorm.io/cli/gorm/migration"
	"gorm.io/gorm"
)

var (
	DB         *gorm.DB
	migrations []migration.Migration
	// tablesRules defines the configuration for model reflection.
	// Each rule matches tables using shell-style patterns.
	tablesRules = []migration.TableRule{
		// {
		// 	 Pattern: "users",
		// 	 Config: migration.TableConfig{
		// 		OutputPath: "internal/models/user.go",
		// 		FieldRules: []migration.FieldRule{
		// 			{Pattern: "name", FieldName: "FullName", Tags: map[string]string{"json": "{{.DBName}}"}},
		// 		},
		// 	 },
		// },
		// {
		// 	Pattern: "audit_*",
		// 	Exclude: true,
		// },
	}
)

func register(m migration.Migration) {
	migrations = append(migrations, m)
}

func main() {
	migration.New(migration.Config{
		ModelsDir:     "models",
		MigrationsDir: "migrations",
		TableRules:    tablesRules,
	}, migration.WithDBAdapter(DB)).Run(migrations)
}
