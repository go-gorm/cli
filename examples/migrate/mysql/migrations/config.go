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
		{Pattern: "mig_mysql_types"},
		{
			Pattern: "mig_mysql_companies",
			Config: migration.TableConfig{
				OutputPath: "models/org_models.go",
				FieldRules: []migration.FieldRule{
					{Pattern: "id", FieldName: "ID", FieldType: "uint64"},
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
			Pattern: "mig_mysql_employees",
			Config: migration.TableConfig{
				OutputPath: "models/org_models.go",
				FieldRules: []migration.FieldRule{
					{Pattern: "id", FieldName: "ID", FieldType: "uint64"},
					{Pattern: "company_id", FieldName: "CompanyRefID", FieldType: "uint64"},
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
			Pattern: "mig_mysql_audit_*",
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
		`CREATE TABLE mig_mysql_types (
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
		`CREATE TABLE mig_mysql_companies (
            id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
            code VARCHAR(64) NOT NULL,
            name VARCHAR(128) NOT NULL,
            founded_year INT NOT NULL,
            metadata JSON NOT NULL,
            created_at DATETIME(3) NOT NULL,
            updated_at DATETIME(3) NOT NULL,
            PRIMARY KEY (id)
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
		`CREATE TABLE mig_mysql_employees (
            id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
            company_id BIGINT UNSIGNED NOT NULL,
            full_name VARCHAR(128) NOT NULL,
            role VARCHAR(64) NOT NULL,
            metadata JSON NOT NULL,
            salary DECIMAL(10,2) NOT NULL,
            hired_at DATETIME(3) NOT NULL,
            departed_at DATETIME(3) NULL,
            PRIMARY KEY (id),
            FOREIGN KEY (company_id) REFERENCES mig_mysql_companies(id)
        ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`,
	}

	for _, sql := range sqls {
		if err := db.Exec(sql).Error; err != nil {
			panic("failed to reset schema: " + err.Error())
		}
	}
}
