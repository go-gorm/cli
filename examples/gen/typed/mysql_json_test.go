package typed

import (
	"context"
	"os"
	"testing"

	"gorm.io/cli/gorm/examples/models"
	generated "gorm.io/cli/gorm/examples/typed/models"
	"gorm.io/cli/gorm/typed"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// setupMySQLTestDB opens a MySQL connection using MYSQL_DSN and migrates schemas.
func setupMySQLTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "gorm:gorm@tcp(127.0.0.1:9910)/gorm?parseTime=true&charset=utf8mb4&loc=Local"
	if os.Getenv("MYSQL_DSN") != "" {
		dsn = os.Getenv("MYSQL_DSN")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect mysql: %v", err)
	}

	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("failed to migrate mysql: %v", err)
	}

	// Clean users table
	if err := db.Exec("DELETE FROM users").Error; err != nil {
		t.Fatalf("failed to clean users: %v", err)
	}
	return db
}

func TestMySQL_JSONEqual_ProfileVIP(t *testing.T) {
	// Skip if MYSQL_DSN is not set
	if os.Getenv("MYSQL_DSN") == "" {
		t.Skip("MYSQL_DSN not set, skipping MySQL test")
	}

	db := setupMySQLTestDB(t)

	// Insert a user with JSON profile
	u := models.User{Name: "vip_user", Age: 23, Role: "active", IsAdult: true, Profile: `{"vip": true}`}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to insert vip_user: %v", err)
	}

	// Query using dialect-aware Equal on JSON path
	got, err := typed.G[models.User](db).
		Where(generated.User.Profile.Equal("$.vip", true)).
		Take(context.Background())
	if err != nil {
		t.Fatalf("mysql json equal take failed: %v", err)
	}
	if got.Name != "vip_user" {
		t.Fatalf("expected vip_user, got %+v", got)
	}
}
