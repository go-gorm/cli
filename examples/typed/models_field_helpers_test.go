package examples

import (
	"context"
	"testing"

	"gorm.io/cli/gorm/examples/models"
	generated "gorm.io/cli/gorm/examples/typed/models"
	"gorm.io/cli/gorm/field"
	"gorm.io/cli/gorm/typed"
)

func TestFieldHelpers_MultipleConditions_FindIntoSlice(t *testing.T) {
	db := setupTestDB(t)
	seedUsers(t, db)

	// Multiple conditions: age > 18 AND role = "active"
	var usersOver18Active []models.User
	if err := typed.G[models.User](db).
		Where(
			field.QueryInterface(generated.User.Age.Gt(18)),
			field.QueryInterface(generated.User.Role.Eq("active")),
		).
		Scan(context.Background(), &usersOver18Active); err != nil {
		t.Fatalf("Scan(ctx, &slice) failed: %v", err)
	}
	if len(usersOver18Active) != 1 {
		t.Fatalf("expected 1 user (age>18 & active), got %d", len(usersOver18Active))
	}
}

func TestFieldHelpers_Update_UsingHelpers(t *testing.T) {
	db := setupTestDB(t)
	seedUsers(t, db)

	// Update using helper-based WHERE: set role from "pending" -> "active"
	if _, err := typed.G[models.User](db).
		Where(generated.User.Role.Eq("pending")).
		Update(context.Background(), "role", "active"); err != nil {
		t.Fatalf("Update using helpers failed: %v", err)
	}

	// Verify: no more pending
	pending, err := typed.G[models.User](db).
		Where(generated.User.Role.Eq("pending")).
		Find(context.Background())
	if err != nil {
		t.Fatalf("verify pending query failed: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after update, got %d", len(pending))
	}

	// And active count increased to 4
	active, err := typed.G[models.User](db).
		Where(generated.User.Role.Eq("active")).
		Find(context.Background())
	if err != nil {
		t.Fatalf("verify active query failed: %v", err)
	}
	if len(active) != 4 {
		t.Fatalf("expected 4 active after update, got %d", len(active))
	}
}

func TestFieldHelpers_Update_WithSetAssignments(t *testing.T) {
	db := setupTestDB(t)
	seedUsers(t, db)

	ctx := context.Background()

	// Use Set(assignments).Update(ctx) to flip all pending users to active
	rows, err := typed.G[models.User](db).
		Where(generated.User.Role.Eq("pending")).
		Set(
			generated.User.Role.Set("active"),
		).
		Update(ctx)
	if err != nil {
		t.Fatalf("Set().Update(ctx) failed: %v", err)
	}
	if rows != 2 {
		t.Fatalf("expected to update 2 rows, got %d", rows)
	}

	// Verify all users are now active (originally 2 active + 2 pending)
	active, err := typed.G[models.User](db).
		Where(generated.User.Role.Eq("active")).
		Count(ctx, "*")
	if err != nil {
		t.Fatalf("count active after Set().Update failed: %v", err)
	}
	if active != 4 {
		t.Fatalf("expected 4 active users after update, got %d", active)
	}
}

func TestFieldHelpers_Create_WithSetAssignments(t *testing.T) {
	db := setupTestDB(t)

	ctx := context.Background()

	// Create a new user using Set(assignments)
	err := typed.G[models.User](db).
		Set(
			generated.User.Name.Set("newuser"),
			generated.User.Age.Set(25),
			generated.User.Role.Set("active"),
		).
		Create(ctx)
	if err != nil {
		t.Fatalf("Create with Set() failed: %v", err)
	}

	// Verify user was created
	users, err := typed.G[models.User](db).
		Where(generated.User.Name.Eq("newuser")).
		Find(ctx)
	if err != nil {
		t.Fatalf("query new user failed: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 new user, got %d", len(users))
	}
	if users[0].Age != 25 || users[0].Role != "active" {
		t.Errorf("user mismatch: %+v", users[0])
	}
}

