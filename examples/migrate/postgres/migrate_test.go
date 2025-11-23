package migrate_test

import (
	"os"
	"testing"

	testutil "gorm.io/cli/gorm/examples/migrate/internal/testutil"
)

func TestMigrate(t *testing.T) {
	projectDir, _ := os.Getwd()

	testutil.RunMigrateTests(t, projectDir)
}
