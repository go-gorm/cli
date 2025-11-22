package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	adapter "gorm.io/cli/gorm/internal/migration/adapter"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	defaultMySQLDSN    = "gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8&parseTime=True&loc=Local"
	defaultPostgresDSN = "user=gorm password=gorm dbname=gorm host=localhost port=9920 sslmode=disable TimeZone=Asia/Shanghai"
)

func openTestConnection(t *testing.T, dialect string) *gorm.DB {
	t.Helper()
	dsn := resolveDSN(dialect)
	var (
		db  *gorm.DB
		err error
	)

	switch dialect {
	case "mysql":
		if dsn == "" {
			dsn = defaultMySQLDSN
		}
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case "postgres":
		if dsn == "" {
			dsn = defaultPostgresDSN
		}
		db, err = gorm.Open(postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true,
		}), &gorm.Config{})
	case "sqlite":
		if dsn == "" {
			dsn = filepath.Join(t.TempDir(), "gorm-migrate.db")
		}
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		if err == nil {
			db.Exec("PRAGMA foreign_keys = ON")
		}
	default:
		t.Fatalf("unsupported dialect %s", dialect)
	}

	if err != nil {
		t.Skipf("skip %s migrate test: %v", dialect, err)
	}

	switch debug := os.Getenv("DEBUG"); debug {
	case "true":
		db.Logger = db.Logger.LogMode(logger.Info)
	case "false":
		db.Logger = db.Logger.LogMode(logger.Silent)
	}

	return db
}

func resolveDSN(dialect string) string {
	// Allow per-dialect overrides first
	if val := os.Getenv(strings.ToUpper(dialect) + "_DSN"); val != "" {
		return val
	}
	if strings.EqualFold(os.Getenv("GORM_DIALECT"), dialect) {
		if val := os.Getenv("GORM_DSN"); val != "" {
			return val
		}
	}
	return ""
}

func dropTables(t *testing.T, db *gorm.DB, tables ...string) {
	t.Helper()
	dialect := db.Dialector.Name()
	for _, table := range tables {
		if strings.TrimSpace(table) == "" {
			continue
		}
		stmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", quoteIdentifier(dialect, table))
		if dialect == "postgres" {
			stmt += " CASCADE"
		}
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("drop table %s: %v", table, err)
		}
	}
}

func quoteIdentifier(dialect, ident string) string {
	switch dialect {
	case "postgres":
		return fmt.Sprintf("\"%s\"", ident)
	default:
		return fmt.Sprintf("`%s`", ident)
	}
}

func execSQL(t *testing.T, db *gorm.DB, statements ...string) {
	t.Helper()
	for _, stmt := range statements {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("exec sql %q: %v", stmt, err)
		}
	}
}

func newAdapter(t *testing.T, db *gorm.DB, cfg adapter.Config) adapter.Adapter {
	t.Helper()
	migAdapter, err := adapter.NewDBAdapter(db, cfg)
	if err != nil {
		t.Fatalf("NewDBAdapter: %v", err)
	}
	return migAdapter
}

func compareWithGolden(t *testing.T, generatedPath, goldenPath string) {
	t.Helper()
	generated, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("read generated file %s: %v", generatedPath, err)
	}
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden file %s: %v", goldenPath, err)
	}
	if string(generated) != string(golden) {
		t.Fatalf("generated file %s does not match golden %s\n--- generated ---\n%s\n--- golden ---\n%s", generatedPath, goldenPath, generated, golden)
	}
}
