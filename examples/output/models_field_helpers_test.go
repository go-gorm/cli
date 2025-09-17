package output

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"gorm.io/cli/gorm/examples"
	"gorm.io/cli/gorm/examples/models"
	generated "gorm.io/cli/gorm/examples/output/models"
	"gorm.io/cli/gorm/field"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestFieldHelpers_MultipleConditions_FindIntoSlice(t *testing.T) {
	db := setupTestDB(t)
	seedUsers(t, db)

	// Multiple conditions: age > 18 AND role = "active"
	var usersOver18Active []models.User
	if err := gorm.G[models.User](db).
		Where(generated.User.Age.Gt(18), generated.User.Role.Eq("active")).
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
	if _, err := gorm.G[models.User](db).
		Where(generated.User.Role.Eq("pending")).
		Update(context.Background(), "role", "active"); err != nil {
		t.Fatalf("Update using helpers failed: %v", err)
	}

	// Verify: no more pending
	pending, err := gorm.G[models.User](db).
		Where(generated.User.Role.Eq("pending")).
		Find(context.Background())
	if err != nil {
		t.Fatalf("verify pending query failed: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending after update, got %d", len(pending))
	}

	// And active count increased to 4
	active, err := gorm.G[models.User](db).
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
	rows, err := gorm.G[models.User](db).
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
	active, err := gorm.G[models.User](db).
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

	// Create a user using Set(assignments).Create(ctx)
	if err := gorm.G[models.User](db).
		Set(
			generated.User.Name.Set("set_user"),
			generated.User.Age.Set(29),
			generated.User.Role.Set("active"),
			generated.User.IsAdult.Set(true),
			// also create with a nullable value set
			generated.User.Score.Set(sql.NullInt64{Int64: 99, Valid: true}),
		).
		Create(ctx); err != nil {
		t.Fatalf("Set().Create(ctx) failed: %v", err)
	}

	// Verify the new record was inserted with expected values
	got, err := gorm.G[models.User](db).
		Where(generated.User.Name.Eq("set_user")).
		First(ctx)
	if err != nil {
		t.Fatalf("failed to load created user: %v", err)
	}
	if got.Name != "set_user" || got.Age != 29 || got.Role != "active" || !got.IsAdult || !got.Score.Valid || got.Score.Int64 != 99 {
		t.Fatalf("unexpected created user values: %+v", got)
	}
}

func TestFieldHelpers_Update_WithSetExpr(t *testing.T) {
	db := setupTestDB(t)
	seedUsers(t, db)

	ctx := context.Background()

	// Increment bob's age by 1 using SetExpr
	rows, err := gorm.G[models.User](db).
		Where(generated.User.Name.Eq("bob")).
		Set(
			generated.User.Age.SetExpr(clause.Expr{SQL: "age + ?", Vars: []any{1}}),
		).
		Update(ctx)
	if err != nil {
		t.Fatalf("SetExpr().Update failed: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected to update 1 row, got %d", rows)
	}

	got, err := gorm.G[models.User](db).
		Where(generated.User.Name.Eq("bob")).
		First(ctx)
	if err != nil {
		t.Fatalf("failed to load updated user: %v", err)
	}
	if got.Age != 18 { // originally 17 in seed
		t.Fatalf("expected bob age=18 after increment, got %d", got.Age)
	}
}

func TestFieldHelpers_Update_Combined_ZeroAndExpr(t *testing.T) {
	db := setupTestDB(t)
	seedUsers(t, db)

	ctx := context.Background()

	// Combine zero-value updates and SQL expression in a single Set(...).Update(ctx)
	rows, err := gorm.G[models.User](db).
		Where(generated.User.Name.Eq("cathy")).
		Set(
			generated.User.Role.Set(""),                                             // zero string
			generated.User.IsAdult.Set(false),                                       // zero bool
			generated.User.Score.Set(sql.NullInt64{}),                               // NULL
			generated.User.Age.SetExpr(clause.Expr{SQL: "age + ?", Vars: []any{2}}), // expr
		).
		Update(ctx)
	if err != nil {
		t.Fatalf("combined zero+expr update failed: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected to update 1 row, got %d", rows)
	}

	got, err := gorm.G[models.User](db).
		Where(generated.User.Name.Eq("cathy")).
		First(ctx)
	if err != nil {
		t.Fatalf("failed to load updated user: %v", err)
	}
	if got.Role != "" || got.IsAdult != false || got.Age != 32 || got.Score.Valid {
		t.Fatalf("unexpected values after combined update: %+v", got)
	}
}

func TestFieldHelpers_Update_WithIncr(t *testing.T) {
	db := setupTestDB(t)
	seedUsers(t, db)

	ctx := context.Background()

	// Use Incr expression directly (GORM Set supports Assigner)
	rows, err := gorm.G[models.User](db).
		Where(generated.User.Name.Eq("bob")).
		Set(
			generated.User.Age.Incr(3),
		).
		Update(ctx)
	if err != nil {
		t.Fatalf("Set(Incr()).Update failed: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected to update 1 row, got %d", rows)
	}

	got, err := gorm.G[models.User](db).
		Where(generated.User.Name.Eq("bob")).
		First(ctx)
	if err != nil {
		t.Fatalf("failed to load updated user: %v", err)
	}
	if got.Age != 20 { // originally 17 in seed
		t.Fatalf("expected bob age=20 after Incr(3), got %d", got.Age)
	}

	_ = got
}

func TestFieldHelpers_Update_StringUpper_Assigner(t *testing.T) {
	db := setupTestDB(t)
	seedUsers(t, db)

	ctx := context.Background()

	rows, err := gorm.G[models.User](db).
		Where(generated.User.Name.Eq("alice")).
		Set(
			generated.User.Name.Upper(),
		).
		Update(ctx)
	if err != nil {
		t.Fatalf("Set(Upper()).Update failed: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected to update 1 row, got %d", rows)
	}

	got, err := gorm.G[models.User](db).
		Where(generated.User.Name.Eq("ALICE")).
		First(ctx)
	if err != nil {
		t.Fatalf("failed to load updated user: %v", err)
	}
	if got.Name != "ALICE" {
		t.Fatalf("expected name ALICE, got %s", got.Name)
	}
}

func TestFieldHelpers_Update_ZeroValues_WithSetAssignments(t *testing.T) {
	db := setupTestDB(t)
	seedUsers(t, db)

	ctx := context.Background()

	// Update a specific user to zero values explicitly
	rows, err := gorm.G[models.User](db).
		Where(generated.User.Name.Eq("alice")).
		Set(
			generated.User.Age.Set(0),                 // int zero
			generated.User.IsAdult.Set(false),         // bool zero
			generated.User.Role.Set(""),               // string zero
			generated.User.Score.Set(sql.NullInt64{}), // NULL (zero value for NullInt64)
		).
		Update(ctx)
	if err != nil {
		t.Fatalf("Set().Update to zero values failed: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected to update 1 row, got %d", rows)
	}

	// Verify zero values persisted
	got, err := gorm.G[models.User](db).
		Where(generated.User.Name.Eq("alice")).
		First(ctx)
	if err != nil {
		t.Fatalf("failed to load updated user: %v", err)
	}
	if got.Age != 0 || got.IsAdult != false || got.Role != "" || got.Score.Valid {
		t.Fatalf("expected zero values persisted, got: %+v", got)
	}
}

func TestFieldHelpers_Count(t *testing.T) {
	db := setupTestDB(t)
	seedUsers(t, db)

	// Expect 2 active users from default seed
	cnt, err := gorm.G[models.User](db).
		Where(generated.User.Role.Eq("active")).
		Count(context.Background(), "*")
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if cnt != 2 {
		t.Fatalf("expected 2 active users, got %d", cnt)
	}
}

func TestFieldHelpers_Delete(t *testing.T) {
	db := setupTestDB(t)
	seedUsers(t, db)

	// Delete pendings
	rows, err := gorm.G[models.User](db).
		Where(generated.User.Role.Eq("pending")).
		Delete(context.Background())
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if rows != 2 {
		t.Fatalf("expected to delete 2 rows, got %d", rows)
	}

	// Verify none pending left
	pending, err := gorm.G[models.User](db).
		Where(generated.User.Role.Eq("pending")).
		Find(context.Background())
	if err != nil {
		t.Fatalf("verify query failed: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected 0 pending, got %d", len(pending))
	}
}

// Test that generated field types match expectations at compile time.
func TestGeneratedModels_FieldTypes(t *testing.T) {
	// User (exact wrapper types, in struct order)
	var (
		_ field.Number[uint]          = generated.User.ID
		_ field.Time                  = generated.User.CreatedAt
		_ field.Time                  = generated.User.UpdatedAt
		_ field.Field[gorm.DeletedAt] = generated.User.DeletedAt
		_ field.String                = generated.User.Name
		_ field.Number[int]           = generated.User.Age
		_ field.Time                  = generated.User.Birthday
		_ field.Field[sql.NullInt64]  = generated.User.Score
		_ field.Time                  = generated.User.LastLogin
		_ field.Number[int]           = generated.User.CompanyID
		_ field.Number[uint]          = generated.User.ManagerID
		_ field.String                = generated.User.Role
		_ field.Bool                  = generated.User.IsAdult
		_ examples.JSON               = generated.User.Profile

		// Associations
		_ field.Struct[models.Account] = generated.User.Account
		_ field.Slice[models.Pet]      = generated.User.Pets
		_ field.Slice[models.Toy]      = generated.User.Toys
		_ field.Struct[models.Company] = generated.User.Company
		_ field.Struct[models.User]    = generated.User.Manager
		_ field.Slice[models.User]     = generated.User.Team
		_ field.Slice[models.Language] = generated.User.Languages
		_ field.Slice[models.User]     = generated.User.Friends

		// Account
		_ field.Number[uint]          = generated.Account.ID
		_ field.Time                  = generated.Account.CreatedAt
		_ field.Time                  = generated.Account.UpdatedAt
		_ field.Field[gorm.DeletedAt] = generated.Account.DeletedAt
		_ field.Field[sql.NullInt64]  = generated.Account.UserID
		_ field.String                = generated.Account.Number

		// Pet
		_ field.Number[uint]          = generated.Pet.ID
		_ field.Time                  = generated.Pet.CreatedAt
		_ field.Time                  = generated.Pet.UpdatedAt
		_ field.Field[gorm.DeletedAt] = generated.Pet.DeletedAt
		_ field.Number[uint]          = generated.Pet.UserID
		_ field.String                = generated.Pet.Name
		_ field.Struct[models.Toy]    = generated.Pet.Toy

		// Toy
		_ field.Number[uint]          = generated.Toy.ID
		_ field.Time                  = generated.Toy.CreatedAt
		_ field.Time                  = generated.Toy.UpdatedAt
		_ field.Field[gorm.DeletedAt] = generated.Toy.DeletedAt
		_ field.String                = generated.Toy.Name
		_ field.Number[uint]          = generated.Toy.OwnerID
		_ field.String                = generated.Toy.OwnerType

		// Company
		_ field.Number[int] = generated.Company.ID
		_ field.String      = generated.Company.Name

		// Language
		_ field.String = generated.Language.Code
		_ field.String = generated.Language.Name

		// CreditCard
		_ field.Number[uint]          = generated.CreditCard.ID
		_ field.Time                  = generated.CreditCard.CreatedAt
		_ field.Time                  = generated.CreditCard.UpdatedAt
		_ field.Field[gorm.DeletedAt] = generated.CreditCard.DeletedAt
		_ field.String                = generated.CreditCard.Number
	)
}

// helper to extract Column from common clause expressions
func getColumnFromExpr(expr clause.Expression) clause.Column {
	switch v := expr.(type) {
	case clause.Eq:
		return v.Column.(clause.Column)
	case clause.Neq:
		return v.Column.(clause.Column)
	case clause.Gt:
		return v.Column.(clause.Column)
	case clause.Gte:
		return v.Column.(clause.Column)
	case clause.Lt:
		return v.Column.(clause.Column)
	case clause.Lte:
		return v.Column.(clause.Column)
	case clause.IN:
		return v.Column.(clause.Column)
	case clause.Expr:
		if len(v.Vars) > 0 {
			if c, ok := v.Vars[0].(clause.Column); ok {
				return c
			}
		}
	}
	return clause.Column{}
}

func TestGeneratedModels_ColumnNames(t *testing.T) {
	if col := getColumnFromExpr(generated.User.ID.Eq(1)); col.Name != "id" {
		t.Fatalf("User.ID column = %q, want %q", col.Name, "id")
	}
	if col := getColumnFromExpr(generated.User.Name.Eq("n")); col.Name != "name" {
		t.Fatalf("User.Name column = %q, want %q", col.Name, "name")
	}
	if col := getColumnFromExpr(generated.User.Age.Gt(10)); col.Name != "age" {
		t.Fatalf("User.Age column = %q, want %q", col.Name, "age")
	}
	if col := getColumnFromExpr(generated.User.Birthday.IsNull()); col.Name != "birthday" {
		t.Fatalf("User.Birthday column = %q, want %q", col.Name, "birthday")
	}
	if col := getColumnFromExpr(generated.User.Score.IsNull()); col.Name != "score" {
		t.Fatalf("User.Score column = %q, want %q", col.Name, "score")
	}
	if col := getColumnFromExpr(generated.User.LastLogin.IsNull()); col.Name != "last_login" {
		t.Fatalf("User.LastLogin column = %q, want %q", col.Name, "last_login")
	}
	if col := getColumnFromExpr(generated.User.CompanyID.Eq(1)); col.Name != "company_id" {
		t.Fatalf("User.CompanyID column = %q, want %q", col.Name, "company_id")
	}
	if col := getColumnFromExpr(generated.User.ManagerID.Eq(1)); col.Name != "manager_id" {
		t.Fatalf("User.ManagerID column = %q, want %q", col.Name, "manager_id")
	}
	if col := getColumnFromExpr(generated.User.IsAdult.Eq(true)); col.Name != "is_adult" {
		t.Fatalf("User.IsAdult column = %q, want %q", col.Name, "is_adult")
	}
	if col := getColumnFromExpr(generated.User.DeletedAt.IsNull()); col.Name != "deleted_at" {
		t.Fatalf("User.DeletedAt column = %q, want %q", col.Name, "deleted_at")
	}

	if col := getColumnFromExpr(generated.Account.UserID.IsNull()); col.Name != "user_id" {
		t.Fatalf("Account.UserID column = %q, want %q", col.Name, "user_id")
	}
	if col := getColumnFromExpr(generated.Account.Number.Eq("n")); col.Name != "number" {
		t.Fatalf("Account.Number column = %q, want %q", col.Name, "number")
	}
	if col := getColumnFromExpr(generated.Account.RewardPoints.IsNull()); col.Name != "reward_points" {
		t.Fatalf("Account.RewardPoints column = %q, want %q", col.Name, "reward_points")
	}
	if col := getColumnFromExpr(generated.Account.LastUsedAt.IsNull()); col.Name != "last_used_at" {
		t.Fatalf("Account.LastUsedAt column = %q, want %q", col.Name, "last_used_at")
	}
	if col := getColumnFromExpr(generated.Pet.UserID.Eq(1)); col.Name != "user_id" {
		t.Fatalf("Pet.UserID column = %q, want %q", col.Name, "user_id")
	}
	if col := getColumnFromExpr(generated.Pet.Name.Eq("n")); col.Name != "name" {
		t.Fatalf("Pet.Name column = %q, want %q", col.Name, "name")
	}
	if col := getColumnFromExpr(generated.Toy.OwnerID.Eq(1)); col.Name != "owner_id" {
		t.Fatalf("Toy.OwnerID column = %q, want %q", col.Name, "owner_id")
	}
	if col := getColumnFromExpr(generated.Toy.OwnerType.Eq("t")); col.Name != "owner_type" {
		t.Fatalf("Toy.OwnerType column = %q, want %q", col.Name, "owner_type")
	}
	if col := getColumnFromExpr(generated.Company.ID.Eq(1)); col.Name != "id" {
		t.Fatalf("Company.ID column = %q, want %q", col.Name, "id")
	}
	if col := getColumnFromExpr(generated.Company.Name.Eq("n")); col.Name != "name" {
		t.Fatalf("Company.Name column = %q, want %q", col.Name, "name")
	}
	if col := getColumnFromExpr(generated.Language.Code.Eq("c")); col.Name != "code" {
		t.Fatalf("Language.Code column = %q, want %q", col.Name, "code")
	}
	if col := getColumnFromExpr(generated.Language.Name.Eq("n")); col.Name != "name" {
		t.Fatalf("Language.Name column = %q, want %q", col.Name, "name")
	}
}

func TestFieldHelpers_NullTypes_DBChecks(t *testing.T) {
	db := setupTestDB(t)
	// Seed default users (Score/LastLogin are NULL by default)
	seedUsers(t, db)

	ctx := context.Background()

	// Initially, all default users should have LastLogin IS NULL
	cnt, err := gorm.G[models.User](db).
		Where(generated.User.LastLogin.IsNull()).
		Count(ctx, "*")
	if err != nil {
		t.Fatalf("count LastLogin IS NULL failed: %v", err)
	}
	if cnt != 4 {
		t.Fatalf("expected 4 users with LastLogin IS NULL, got %d", cnt)
	}

	// Insert extra users with non-null values for Null fields
	now := time.Now()
	extra := []models.User{
		{Name: "eva", Age: 25, Role: "active", IsAdult: true, LastLogin: sql.NullTime{Time: now, Valid: true}},
		{Name: "frank", Age: 22, Role: "pending", IsAdult: true, Score: sql.NullInt64{Int64: 50, Valid: true}},
	}
	if err := db.Create(&extra).Error; err != nil {
		t.Fatalf("failed to insert extra users: %v", err)
	}

	// Query users with LastLogin IS NOT NULL -> expect 1 (eva)
	cntNotNull, err := gorm.G[models.User](db).
		Where(generated.User.LastLogin.IsNotNull()).
		Count(ctx, "*")
	if err != nil {
		t.Fatalf("count LastLogin IS NOT NULL failed: %v", err)
	}
	if cntNotNull != 1 {
		t.Fatalf("expected 1 user with LastLogin IS NOT NULL, got %d", cntNotNull)
	}

	// Update a seeded user to set a non-null Score
	if _, err := gorm.G[models.User](db).
		Where(generated.User.Name.Eq("bob")).
		Update(ctx, "score", sql.NullInt64{Int64: 88, Valid: true}); err != nil {
		t.Fatalf("failed to update score for bob: %v", err)
	}

	// Query users with Score IS NOT NULL -> expect 2 (frank, bob)
	cntScoreNotNull, err := gorm.G[models.User](db).
		Where(generated.User.Score.IsNotNull()).
		Count(ctx, "*")
	if err != nil {
		t.Fatalf("count Score IS NOT NULL failed: %v", err)
	}
	if cntScoreNotNull != 2 {
		t.Fatalf("expected 2 users with Score IS NOT NULL, got %d", cntScoreNotNull)
	}

	// Test Account null fields as well
	acc := models.Account{Number: "A1", RewardPoints: sql.NullInt64{Int64: 10, Valid: true}}
	if err := db.Create(&acc).Error; err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	// RewardPoints NOT NULL -> expect 1
	cntRP, err := gorm.G[models.Account](db).
		Where(generated.Account.RewardPoints.IsNotNull()).
		Count(ctx, "*")
	if err != nil {
		t.Fatalf("count RewardPoints IS NOT NULL failed: %v", err)
	}
	if cntRP != 1 {
		t.Fatalf("expected 1 account with RewardPoints IS NOT NULL, got %d", cntRP)
	}

	// Set LastUsedAt for the same account
	if _, err := gorm.G[models.Account](db).
		Where(generated.Account.Number.Eq("A1")).
		Update(ctx, "last_used_at", sql.NullTime{Time: now, Valid: true}); err != nil {
		t.Fatalf("failed to update account last_used_at: %v", err)
	}
	cntLUA, err := gorm.G[models.Account](db).
		Where(generated.Account.LastUsedAt.IsNotNull()).
		Count(ctx, "*")
	if err != nil {
		t.Fatalf("count LastUsedAt IS NOT NULL failed: %v", err)
	}
	if cntLUA != 1 {
		t.Fatalf("expected 1 account with LastUsedAt IS NOT NULL, got %d", cntLUA)
	}
}

// SQLite JSON1 extension compatibility test: filter users by JSON attribute in Profile.
func TestCustomFieldAsJSON(t *testing.T) {
	db := setupTestDB(t)
	seedUsers(t, db)

	expr := generated.User.Profile.Contains(`{"vip":true}`)
	e, ok := expr.(clause.Expr)
	if !ok {
		t.Fatalf("expected clause.Expr, got %T", expr)
	}
	if e.SQL != "JSON_CONTAINS(?, ?)" {
		t.Fatalf("unexpected SQL for JSON contains: %q", e.SQL)
	}
	if len(e.Vars) != 2 {
		t.Fatalf("expected 2 vars, got %d", len(e.Vars))
	}
	if col, ok := e.Vars[0].(clause.Column); !ok || col.Name != "profile" {
		t.Fatalf("expected first var to be clause.Column{Name:'profile'}, got %#v", e.Vars[0])
	}

	// Insert a user with a JSON profile marking vip=true
	u := models.User{Name: "vip_user", Age: 23, Role: "active", IsAdult: true, Profile: `{"vip": true}`}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("failed to insert vip_user: %v", err)
	}

	ctx := context.Background()
	// Use the JSON field helper's SQLiteEqual to filter by Profile.vip == 1
	got, err := gorm.G[models.User](db).
		Where(generated.User.Profile.Equal("$.vip", 1)).
		Take(ctx)
	if err != nil {
		// If JSON1 extension is unavailable, skip the test gracefully
		if strings.Contains(strings.ToLower(err.Error()), "no such function: json_extract") {
			t.Skip("sqlite build does not include JSON1; skipping")
		}
		t.Fatalf("json filter find failed: %v", err)
	}
	if got.Name != "vip_user" {
		t.Fatalf("expected to get vip_user, got %+v", got)
	}
}
