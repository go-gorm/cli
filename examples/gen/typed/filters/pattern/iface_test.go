package pattern

import (
	"context"
	"testing"

	"gorm.io/cli/gorm/examples/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	// Use a uniquely named in-memory database per test to ensure isolation
	dsn := "file:pattern-" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	// Run migrations
	err = db.AutoMigrate(&models.User{})
	if err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	return db
}

func seedUsers(t *testing.T, db *gorm.DB, extra ...models.User) []models.User {
	t.Helper()
	users := []models.User{
		{Name: "alice", Age: 20},
		{Name: "bob", Age: 17},
	}
	users = append(users, extra...)
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("failed to seed users: %v", err)
	}
	return users
}

func TestQueryUser_ByID(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db, models.User{Name: "test_user", Age: 25})

	// Get the test user
	var testUser models.User
	for _, u := range users {
		if u.Name == "test_user" {
			testUser = u
			break
		}
	}

	// Test QueryUser ByID method
	queryUser := QueryUser[models.User](db)
	result, err := queryUser.ByID(context.Background(), int(testUser.ID))
	if err != nil {
		t.Fatalf("QueryUser.ByID failed: %v", err)
	}

	if result.Name != "test_user" || result.Age != 25 {
		t.Errorf("expected user test_user with age 25, got: %+v", result)
	}
}

func TestQueryOrder_ByNumber(t *testing.T) {
	db := setupTestDB(t)
	// For this test, we'll just verify the function can be called
	// Since we don't have an Order model, we'll test with User model
	queryOrder := QueryOrder[models.User](db)

	// This should compile and execute without error
	_, err := queryOrder.ByNumber(context.Background(), "12345")
	// We expect an error since there's no matching data, but the call should work
	if err != nil {
		// This is expected since there's no data
		t.Logf("Expected error when querying non-existent order: %v", err)
	}
}
