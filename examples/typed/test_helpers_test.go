package examples

import (
	"os"
	"testing"

	"gorm.io/cli/gorm/examples/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	// Use a uniquely named in-memory database per test to ensure isolation
	dsn := "file:query-" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	// Run migrations (include associations and join tables)
	err = db.AutoMigrate(&models.User{}, &models.Account{}, &models.Pet{}, &models.Toy{}, &models.Company{}, &models.Language{})
	if err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	if os.Getenv("DEBUG") == "true" {
		db = db.Debug()
	}

	return db
}

// seedUsers inserts a default set of users and any additional ones provided.
func seedUsers(t *testing.T, db *gorm.DB, extra ...models.User) []models.User {
	t.Helper()
	users := []models.User{
		{Name: "alice", Age: 20, Role: "active", IsAdult: true},
		{Name: "bob", Age: 17, Role: "active", IsAdult: false},
		{Name: "cathy", Age: 30, Role: "pending", IsAdult: true},
		{Name: "dan", Age: 40, Role: "pending", IsAdult: true},
	}
	users = append(users, extra...)
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("failed to seed users: %v", err)
	}
	return users
}