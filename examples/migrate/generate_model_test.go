package migrate

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"gorm.io/cli/gorm/internal/migration/adapter"
	"gorm.io/gorm"
)

type modelTestCase struct {
	name       string
	dialect    string
	table      string
	structName string
	goldenDir  string
	ddl        string
}

var modelTests = []modelTestCase{
	{
		name:       "MySQLTypes",
		dialect:    "mysql",
		table:      "mig_mysql_types",
		structName: "MigMysqlType",
		goldenDir:  "mysql",
		ddl: `CREATE TABLE mig_mysql_types (
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
	},
	{
		name:       "PostgresTypes",
		dialect:    "postgres",
		table:      "mig_postgres_types",
		structName: "MigPostgresType",
		goldenDir:  "postgres",
		ddl: `CREATE TABLE IF NOT EXISTS mig_postgres_types (
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
	},
	{
		name:       "SQLiteTypes",
		dialect:    "sqlite",
		table:      "mig_sqlite_types",
		structName: "MigSqliteType",
		goldenDir:  "sqlite",
		ddl: `CREATE TABLE mig_sqlite_types (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            flag BOOLEAN NOT NULL,
            quantity INTEGER NOT NULL,
            ratio REAL NOT NULL,
            payload BLOB NOT NULL,
            note TEXT NOT NULL,
            created_at DATETIME NOT NULL
        );`,
	},
}

func TestGenerateModel(t *testing.T) {
	for _, tc := range modelTests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			db := openTestConnection(t, tc.dialect)
			t.Run("fresh", func(t *testing.T) {
				runFreshModelTest(t, db, tc)
			})
			t.Run("merge", func(t *testing.T) {
				runMergeModelTest(t, db, tc)
			})
		})
	}
}

func runFreshModelTest(t *testing.T, db *gorm.DB, tc modelTestCase) {
	t.Helper()
	dropTables(t, db, tc.table)
	t.Cleanup(func() { dropTables(t, db, tc.table) })
	execSQL(t, db, tc.ddl)

	tempDir := t.TempDir()
	modelsDir := filepath.Join(tempDir, "models")
	cfg := adapter.Config{
		ModelsDir: modelsDir,
		TableRules: []adapter.TableRule{
			{Pattern: tc.table, Config: adapter.TableConfig{}},
		},
	}

	migAdapter := newAdapter(t, db, cfg)
	if err := migAdapter.GenerateModel(adapter.GenerateModelOptions{AutoApprove: true}); err != nil {
		t.Fatalf("GenerateModel failed: %v", err)
	}

	compareWithGolden(t, filepath.Join(modelsDir, tc.table+".go"), goldenPath(tc))
}

func runMergeModelTest(t *testing.T, db *gorm.DB, tc modelTestCase) {
	t.Helper()
	dropTables(t, db, tc.table)
	t.Cleanup(func() { dropTables(t, db, tc.table) })
	execSQL(t, db, tc.ddl)

	tempDir := t.TempDir()
	modelsDir := filepath.Join(tempDir, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		t.Fatalf("mkdir models: %v", err)
	}

	dest := filepath.Join(modelsDir, tc.table+".go")
	partialFromGolden(t, goldenPath(tc), dest, tc.structName, 2)

	cfg := adapter.Config{
		ModelsDir: modelsDir,
		TableRules: []adapter.TableRule{
			{Pattern: tc.table, Config: adapter.TableConfig{}},
		},
	}

	migAdapter := newAdapter(t, db, cfg)
	if err := migAdapter.GenerateModel(adapter.GenerateModelOptions{AutoApprove: true}); err != nil {
		t.Fatalf("GenerateModel merge failed: %v", err)
	}

	compareWithGolden(t, dest, goldenPath(tc))
}

func partialFromGolden(t *testing.T, goldenPath, dest, structName string, remove int) {
	t.Helper()
	src, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, goldenPath, src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse golden: %v", err)
	}
	var target *ast.StructType
	ast.Inspect(file, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == structName {
			if st, ok := ts.Type.(*ast.StructType); ok {
				target = st
				return false
			}
		}
		return true
	})
	if target == nil {
		t.Fatalf("struct %s not found in golden", structName)
	}
	if remove >= len(target.Fields.List) {
		remove = len(target.Fields.List) - 1
	}
	target.Fields.List = target.Fields.List[:len(target.Fields.List)-remove]
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		t.Fatalf("format partial: %v", err)
	}
	if err := os.WriteFile(dest, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write partial: %v", err)
	}
}

func goldenPath(tc modelTestCase) string {
	return filepath.Join("testdata", tc.goldenDir, "models", tc.table+".go")
}
