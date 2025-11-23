package main

import (
	"os"

	"gorm.io/cli/gorm/migration"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var dsn = "gorm:gorm@tcp(127.0.0.1:9910)/gorm_cli?parseTime=true&charset=utf8mb4&loc=Local"

func init() {
	if v := os.Getenv("MYSQL_DSN"); v != "" {
		dsn = v
	}

	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	resetSchema(DB)

	tablesRules = []migration.TableRule{
		{Pattern: "data_types"},
		{
			Pattern: "companies",
			Config: migration.TableConfig{
				OutputPath: "models/org_models.go",
				FieldRules: []migration.FieldRule{
					{Pattern: "name", FieldName: "CompanyName", Tags: map[string]string{"json": "company_name"}},
					{Pattern: "metadata", FieldName: "MetadataJSON", FieldType: "[]byte"},
				},
			},
		},
		{
			Pattern: "employees",
			Config: migration.TableConfig{
				OutputPath: "models/org_models.go",
				FieldRules: []migration.FieldRule{
					{Pattern: "employees.company_id", FieldName: "CompanyRefID", FieldType: "uint64"},
					{Pattern: "employees.full_name", FieldName: "FullName"},
					{Pattern: "employees.metadata", FieldName: "MetadataBlob", FieldType: "[]byte"},
					{Pattern: "employees.hired_at", FieldName: "HiredAt", FieldType: "time.Time"},
					{Pattern: "employees.departed_at", FieldName: "DepartedAt", FieldType: "*time.Time"},
					{Pattern: "employees.role", Tags: map[string]string{"json": "{{.DBName}}"}},
				},
			},
		},
		{
			Pattern: "employee_profiles",
			Config: migration.TableConfig{
				OutputPath: "models/hr/profiles.go",
				FieldRules: []migration.FieldRule{
					{Pattern: "employee_profiles.avatar_url", FieldName: "AvatarURL"},
					{Pattern: "employee_profiles.phone", FieldType: "sql.NullString", Imports: []string{"database/sql"}},
				},
			},
		},
		{
			Pattern: "employee_*",
			Config: migration.TableConfig{
				FieldRules: []migration.FieldRule{
					{Pattern: "*.metadata", FieldName: "MetadataJSON", FieldType: "[]byte"},
					{Pattern: "*.created_at", Tags: map[string]string{"json": "{{.DBName}}"}},
					{Pattern: "*.updated_at", Tags: map[string]string{"json": "{{.DBName}}"}},
				},
			},
		},
		{
			Pattern: "audit_*",
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
		`CREATE TABLE data_types (
            id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
            tiny_signed TINYINT NOT NULL,
            tiny_unsigned TINYINT UNSIGNED NOT NULL,
            small_signed SMALLINT NOT NULL,
            small_unsigned SMALLINT UNSIGNED NOT NULL,
            int_signed INT NOT NULL,
            int_unsigned INT UNSIGNED NOT NULL,
            big_signed BIGINT NOT NULL,
            float_col FLOAT NOT NULL,
            double_col DOUBLE NOT NULL,
            char_col VARCHAR(191) NOT NULL,
            text_col TEXT NOT NULL,
            blob_col BLOB NOT NULL,
            binary_col VARBINARY(128) NOT NULL,
            created_at DATETIME(3) NOT NULL,
            PRIMARY KEY (id)
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE companies (
            id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
            code VARCHAR(64) NOT NULL,
            name VARCHAR(128) NOT NULL,
            founded_year INT NOT NULL,
            metadata JSON NOT NULL,
            created_at DATETIME(3) NOT NULL,
            updated_at DATETIME(3) NOT NULL,
            PRIMARY KEY (id)
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE employees (
            id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
            company_id BIGINT UNSIGNED NOT NULL,
            full_name VARCHAR(128) NOT NULL,
            role VARCHAR(64) NOT NULL,
            metadata JSON NOT NULL,
            salary DECIMAL(10,2) NOT NULL,
            hired_at DATETIME(3) NOT NULL,
            departed_at DATETIME(3) NULL,
            created_at DATETIME(3) NOT NULL,
            updated_at DATETIME(3) NOT NULL,
            PRIMARY KEY (id),
            FOREIGN KEY (company_id) REFERENCES companies(id)
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE employee_profiles (
            id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
            employee_id BIGINT UNSIGNED NOT NULL,
            phone VARCHAR(32) NULL,
            avatar_url VARCHAR(255) NOT NULL,
            metadata JSON NOT NULL,
            created_at DATETIME(3) NOT NULL,
            updated_at DATETIME(3) NOT NULL,
            PRIMARY KEY (id),
            FOREIGN KEY (employee_id) REFERENCES employees(id)
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE audit_logs (
            id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
            table_name VARCHAR(128) NOT NULL,
            payload JSON NOT NULL,
            created_at DATETIME(3) NOT NULL,
            PRIMARY KEY (id)
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
	}

	for _, sql := range sqls {
		if err := db.Exec(sql).Error; err != nil {
			panic("failed to reset schema: " + err.Error())
		}
	}
}
