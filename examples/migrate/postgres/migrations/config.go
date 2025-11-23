package main

import (
	"os"

	"gorm.io/cli/gorm/migration"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var dsn = "user=gorm password=gorm dbname=gorm_cli host=localhost port=9920 sslmode=disable TimeZone=Asia/Shanghai"

func init() {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		dsn = v
	}

	var err error
	DB, err = gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	resetSchema(DB)

	tablesRules = []migration.TableRule{
		{Pattern: "mig_postgres_types"},
		{
			Pattern: "mig_postgres_companies",
			Config: migration.TableConfig{
				OutputPath: "models/org_models.go",
				FieldRules: []migration.FieldRule{
					{Pattern: "id", FieldName: "ID", FieldType: "int64"},
					{Pattern: "code", FieldName: "Code", FieldType: "string"},
					{Pattern: "name", FieldName: "CompanyName", FieldType: "string", Tags: map[string]string{"json": "company_name"}},
					{Pattern: "founded_year", FieldName: "FoundedYear", FieldType: "int"},
					{Pattern: "metadata", FieldName: "MetadataJSON", FieldType: "[]byte"},
					{Pattern: "created_at", FieldName: "CreatedAt", FieldType: "time.Time"},
					{Pattern: "updated_at", FieldName: "UpdatedAt", FieldType: "time.Time"},
				},
			},
		},
		{
			Pattern: "mig_postgres_employees",
			Config: migration.TableConfig{
				OutputPath: "models/org_models.go",
				FieldRules: []migration.FieldRule{
					{Pattern: "id", FieldName: "ID", FieldType: "int64"},
					{Pattern: "company_id", FieldName: "CompanyRefID", FieldType: "int64"},
					{Pattern: "full_name", FieldName: "FullName", FieldType: "string"},
					{Pattern: "role", FieldName: "Role", FieldType: "string"},
					{Pattern: "metadata", FieldName: "MetadataBlob", FieldType: "[]byte"},
					{Pattern: "salary", FieldName: "Salary", FieldType: "float64"},
					{Pattern: "hired_at", FieldName: "HiredAt", FieldType: "time.Time"},
					{Pattern: "departed_at", FieldName: "DepartedAt", FieldType: "*time.Time"},
				},
			},
		},
		{
			Pattern: "mig_postgres_audit_*",
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
		if err := db.Migrator().DropTable(table); err != nil {
			panic("failed to drop table: " + err.Error())
		}
	}

	sqls := []string{
		`CREATE TABLE IF NOT EXISTS mig_postgres_types (
            id BIGSERIAL PRIMARY KEY,
            flag BOOLEAN NOT NULL,
            counter SMALLINT NOT NULL,
            score INTEGER NOT NULL,
            balance BIGINT NOT NULL,
            ratio REAL NOT NULL,
            price DOUBLE PRECISION NOT NULL,
            payload BYTEA NOT NULL,
            name TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL,
            expires_at TIMESTAMPTZ NULL
        );`,
		`CREATE TABLE mig_postgres_companies (
            id BIGSERIAL PRIMARY KEY,
            code TEXT NOT NULL,
            name TEXT NOT NULL,
            founded_year INTEGER NOT NULL,
            metadata JSONB NOT NULL,
            created_at TIMESTAMPTZ NOT NULL,
            updated_at TIMESTAMPTZ NOT NULL
        );`,
		`CREATE TABLE mig_postgres_employees (
            id BIGSERIAL PRIMARY KEY,
            company_id BIGINT NOT NULL REFERENCES mig_postgres_companies(id),
            full_name TEXT NOT NULL,
            role TEXT NOT NULL,
            metadata JSONB NOT NULL,
            salary NUMERIC(12,2) NOT NULL,
            hired_at TIMESTAMPTZ NOT NULL,
            departed_at TIMESTAMPTZ NULL
        );`,
	}

	for _, sql := range sqls {
		if err := db.Exec(sql).Error; err != nil {
			panic("failed to reset schema: " + err.Error())
		}
	}
}
