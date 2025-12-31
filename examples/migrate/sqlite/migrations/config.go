package main

import (
	"gorm.io/cli/gorm/migration"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func init() {
	var err error
	DB, err = gorm.Open(sqlite.Open("./sqlite.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	resetSchema(DB)

	tablesRules = []migration.TableRule{
		{Pattern: "mig_sqlite_types"},
		{
			Pattern: "org_companies",
			Config: migration.TableConfig{
				OutputPath: "models/org_models.go",
				FieldRules: []migration.FieldRule{
					{Pattern: "name", FieldName: "CompanyName", Tags: map[string]string{"json": "company_name"}},
					{Pattern: "metadata", FieldName: "MetadataBlob", FieldType: "[]byte"},
				},
			},
		},
		{
			Pattern: "org_employees",
			Config: migration.TableConfig{
				OutputPath: "models/org_models.go",
				FieldRules: []migration.FieldRule{
					{Pattern: "company_id", FieldName: "CompanyRefID", FieldType: "*int64"},
					{Pattern: "metadata", FieldName: "MetadataJSON", FieldType: "[]byte"},
				},
			},
		},
		{
			Pattern: "org_audit_*",
			Exclude: true,
		},
		{
			Pattern: "sqlite_sequence",
			Exclude: true,
		},
	}
}

func resetSchema(db *gorm.DB) {
	tables, err := db.Migrator().GetTables()
	if err != nil {
		panic("failed to get tables: " + err.Error())
	}

	for _, table := range tables {
		if table == "sqlite_sequence" {
			continue
		}
		if err := db.Migrator().DropTable(table); err != nil {
			panic("failed to drop table: " + err.Error())
		}
	}

	sqls := []string{
		"DROP TABLE IF EXISTS mig_sqlite_types;",
		`CREATE TABLE IF NOT EXISTS mig_sqlite_types (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            flag BOOLEAN NOT NULL,
            quantity INTEGER NOT NULL,
            ratio REAL NOT NULL,
            payload BLOB NOT NULL,
            note TEXT NOT NULL,
            created_at DATETIME NOT NULL
        );`,
		"DROP TABLE IF EXISTS org_employees;",
		"DROP TABLE IF EXISTS org_companies;",
		"DROP TABLE IF EXISTS org_audit_logs;",
		`CREATE TABLE org_companies (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            code TEXT NOT NULL,
            name TEXT NOT NULL,
            founded_year INTEGER NOT NULL,
            metadata TEXT NOT NULL,
            created_at DATETIME NOT NULL,
            updated_at DATETIME NOT NULL
        );`,
		`CREATE TABLE org_employees (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            company_id INTEGER NOT NULL,
            full_name TEXT NOT NULL,
            role TEXT NOT NULL,
            metadata TEXT NOT NULL,
            salary REAL NOT NULL,
            hired_at DATETIME NOT NULL,
            departed_at DATETIME,
            FOREIGN KEY(company_id) REFERENCES org_companies(id)
        );`,
	}

	for _, sql := range sqls {
		if err := db.Exec(sql).Error; err != nil {
			panic("failed to reset schema: " + err.Error())
		}
	}
}
