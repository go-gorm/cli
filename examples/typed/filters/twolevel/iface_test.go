package twolevel

import (
	"context"
	"testing"

	"gorm.io/cli/gorm/examples/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	// Use a uniquely named in-memory database per test to ensure isolation
	dsn := "file:twolevel-" + t.Name() + "?mode=memory&cache=shared"
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

func TestI1_ByID(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db, models.User{Name: "test", Age: 25})

	// Get the test user
	var testUser models.User
	for _, u := range users {
		if u.Name == "test" {
			testUser = u
			break
		}
	}

	// Test I1 ByID method
	i1 := I1[models.User](db)
	result, err := i1.ByID(context.Background(), int(testUser.ID))
	if err != nil {
		t.Fatalf("I1.ByID failed: %v", err)
	}

	if result.Name != "test" || result.Age != 25 {
		t.Errorf("expected user test with age 25, got: %+v", result)
	}
}

func TestI2_ByID2(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db, models.User{Name: "test2", Age: 30})

	// Get the test user
	var testUser models.User
	for _, u := range users {
		if u.Name == "test2" {
			testUser = u
			break
		}
	}

	// Test I2 ByID2 method
	i2 := I2[models.User](db)
	result, err := i2.ByID2(context.Background(), int(testUser.ID))
	if err != nil {
		t.Fatalf("I2.ByID2 failed: %v", err)
	}

	if result.Name != "test2" || result.Age != 30 {
		t.Errorf("expected user test2 with age 30, got: %+v", result)
	}
}