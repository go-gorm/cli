package examples

import (
	"context"
	"testing"
	"time"

	"gorm.io/cli/gorm/examples/models"
	"gorm.io/gorm"
)

func TestUserQueries(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db, models.User{Name: "@name", Age: 28, Role: "special"})

	t.Run("Test GetByID", func(t *testing.T) {
		query := Query[models.User](db)
		// Find the special user we appended
		var special models.User
		for _, u := range users {
			if u.Name == "@name" {
				special = u
				break
			}
		}
		user, err := query.GetByID(context.Background(), int(special.ID))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// The generated SQL has a hardcoded "@name" condition, so it should match the user with name "@name"
		if user.Name != "@name" {
			t.Errorf("expected user '@name', got: %+v", user)
		}
	})

	t.Run("Test FilterWithColumn", func(t *testing.T) {
		query := Query[models.User](db)
		u, err := query.FilterWithColumn(context.Background(), "role", "special")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if u.Role != "special" {
			t.Errorf("expected role 'special', got: %+v", u)
		}
	})

	t.Run("Test QueryWith", func(t *testing.T) {
		query := Query[models.User](db)
		result, err := query.QueryWith(context.Background(), models.User{Name: "dan"})
		if err != nil || result.Name != "dan" {
			t.Errorf("expected one 'dan', got: %+v, error: %v", result, err)
		}
	})

	t.Run("Test Filter", func(t *testing.T) {
		query := Query[models.User](db)
		// The filter method requires Name != "" && Age > 0 to add conditions
		filters := []models.User{
			{Name: "alice", Age: 20, Role: "active"},
			{Name: "dan", Age: 40, Role: "pending"},
		}
		results, err := query.Filter(context.Background(), filters)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 users, got: %d", len(results))
		}
	})

	t.Run("Test FilterByNameAndAge", func(t *testing.T) {
		query := Query[models.User](db)
		result := query.FilterByNameAndAge(context.Background(), "alice", 20)
		if result == nil {
			t.Error("expected a valid query result, got nil")
		}
	})

	t.Run("Test FilterWithTime", func(t *testing.T) {
		query := Query[models.User](db)
		start := time.Now().Add(-1 * time.Hour)
		end := time.Now().Add(1 * time.Hour)
		results, err := query.FilterWithTime(context.Background(), start, end)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(results) != len(users) {
			t.Errorf("expected %d users, got: %d", len(users), len(results))
		}
	})

	t.Run("Test UpdateInfo", func(t *testing.T) {
		query := Query[models.User](db)
		// Pick any user and set Age to 40; is_adult should be true
		u := users[0]
		if err := query.UpdateInfo(context.Background(), models.User{Name: u.Name, Age: 40}, int(u.ID)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, err := gorm.G[models.User](db).Where("id = ?", u.ID).First(context.Background())
		if err != nil {
			t.Fatalf("failed to load updated user: %v", err)
		}
		if !got.IsAdult || got.Age != 40 {
			t.Errorf("expected age=40 and is_adult=true, got: %+v", got)
		}
	})
}