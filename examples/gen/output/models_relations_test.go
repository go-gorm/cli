package output

import (
	"context"
	"database/sql"
	"testing"

	"gorm.io/cli/gorm/examples/models"
	generated "gorm.io/cli/gorm/examples/output/models"
	"gorm.io/gorm"
)

func TestAssociation_Create_SingleParent(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db)
	u := users[0]

	ctx := context.Background()

	// Create one pet for the single parent
	_, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(
			generated.User.Pets.Create(
				generated.Pet.Name.Set("test-pet"),
			),
		).
		Update(ctx)
	if err != nil {
		t.Fatalf("assoc create single failed: %v", err)
	}

	// Verify pet created and associated
	pets, err := gorm.G[models.Pet](db).
		Where(generated.Pet.Name.Eq("test-pet")).
		Find(ctx)
	if err != nil {
		t.Fatalf("query pets failed: %v", err)
	}
	if len(pets) != 1 {
		t.Fatalf("expected 1 pet, got %d", len(pets))
	}
	if pets[0].UserID == nil || *pets[0].UserID != u.ID {
		t.Fatalf("pet not associated to user %d: %#v", u.ID, pets[0])
	}
}

func TestAssociation_Create_MultipleParents(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db)
	u1, u2 := users[0], users[1]

	ctx := context.Background()

	// Create one pet for each matched parent (two users)
	_, err := gorm.G[models.User](db).
		Where(generated.User.Name.In(u1.Name, u2.Name)).
		Set(
			generated.User.Pets.Create(
				generated.Pet.Name.Set("multi-pet"),
			),
		).
		Update(ctx)
	if err != nil {
		t.Fatalf("assoc create multi failed: %v", err)
	}

	// Verify two pets created with correct names
	pets, err := gorm.G[models.Pet](db).
		Where(generated.Pet.Name.Eq("multi-pet")).
		Find(ctx)
	if err != nil {
		t.Fatalf("query multi-pets failed: %v", err)
	}
	if len(pets) != 2 {
		t.Fatalf("expected 2 pets, got %d", len(pets))
	}
}

func TestAssociation_Update_WithConditions(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db)
	u := users[0]

	// Seed one pet for the user
	if err := db.Create(&models.Pet{Name: "old", UserID: &u.ID}).Error; err != nil {
		t.Fatalf("seed pet failed: %v", err)
	}

	ctx := context.Background()

	// Update the associated pet name where name='old'
	_, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(
			generated.User.Pets.Where(generated.Pet.Name.Eq("old")).Update(
				generated.Pet.Name.Set("new"),
			),
		).
		Update(ctx)
	if err != nil {
		t.Fatalf("assoc update failed: %v", err)
	}

	// Verify update applied
	got, err := gorm.G[models.Pet](db).
		Where(generated.Pet.Name.Eq("new")).
		First(ctx)
	if err != nil {
		t.Fatalf("load updated pet failed: %v", err)
	}
	if got.UserID == nil || *got.UserID != u.ID {
		t.Fatalf("updated pet has wrong association: %#v", got)
	}
}

func TestAssociation_Delete(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db)
	u := users[0]

	// Seed two pets
	if err := db.Create(&[]models.Pet{{Name: "a", UserID: &u.ID}, {Name: "b", UserID: &u.ID}}).Error; err != nil {
		t.Fatalf("seed pets failed: %v", err)
	}

	ctx := context.Background()

	// Delete pet with name 'a'
	_, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(
			generated.User.Pets.Where(generated.Pet.Name.Eq("a")).Delete(),
		).
		Update(ctx)
	if err != nil {
		t.Fatalf("assoc delete failed: %v", err)
	}

	// Verify only pet 'b' remains for the user
	pets, err := gorm.G[models.Pet](db).
		Where(generated.Pet.UserID.Eq(u.ID)).
		Find(ctx)
	if err != nil {
		t.Fatalf("query remaining pets failed: %v", err)
	}
	if len(pets) != 1 || pets[0].UserID == nil || *pets[0].UserID != u.ID {
		t.Fatalf("unexpected remaining pets: %+v", pets)
	}
}

func TestAssociation_Unlink_HasMany(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db)
	u := users[0]

	// Seed two pets
	if err := db.Create(&[]models.Pet{{Name: "u1", UserID: &u.ID}, {Name: "u2", UserID: &u.ID}}).Error; err != nil {
		t.Fatalf("seed pets failed: %v", err)
	}

	ctx := context.Background()

	// Unlink all pets from the user (keep pet rows, set user_id NULL)
	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(
			generated.User.Pets.Unlink(),
		).
		Update(ctx); err != nil {
		t.Fatalf("assoc unlink failed: %v", err)
	}

	// Verify user has no associated pets
	count, err := gorm.G[models.Pet](db).
		Where("user_id IS NULL").
		Count(ctx, "*")
	if err != nil {
		t.Fatalf("count unlinked pets failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 unlinked pets, got %d", count)
	}
}

func TestAssociation_HasOne_Account(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db)
	u := users[0]

	ctx := context.Background()

	// Seed account directly, then exercise association Update/Unlink
	acc := models.Account{Number: "A-001", UserID: sql.NullInt64{Int64: int64(u.ID), Valid: true}}
	if err := db.Create(&acc).Error; err != nil {
		t.Fatalf("seed account failed: %v", err)
	}

	// Update account via association condition
	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(
			generated.User.Account.Where(generated.Account.Number.Eq("A-001")).Update(
				generated.Account.Number.Set("A-002"),
			),
		).
		Update(ctx); err != nil {
		t.Fatalf("has one update failed: %v", err)
	}
	var acc2 models.Account
	if err := db.Where("number = ?", "A-002").First(&acc2).Error; err != nil || acc2.UserID.Int64 != int64(u.ID) {
		t.Fatalf("updated account not found: %v", err)
	}

	// Unlink: set foreign key to NULL but keep account row
	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(generated.User.Account.Unlink()).
		Update(ctx); err != nil {
		t.Fatalf("has one unlink failed: %v", err)
	}
	var acc3 models.Account
	if err := db.Where("number = ?", "A-002").First(&acc3).Error; err != nil {
		t.Fatalf("load unlinked account failed: %v", err)
	}
	if acc3.UserID.Valid {
		t.Fatalf("expected unlinked account user_id NULL, got %#v", acc3.UserID)
	}
}

func TestAssociation_HasOne_Account_Delete(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db)
	u := users[0]
	ctx := context.Background()

	// Create then delete
	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(generated.User.Account.Create(generated.Account.Number.Set("A-DEL"))).
		Update(ctx); err != nil {
		t.Fatalf("create for delete failed: %v", err)
	}
	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(generated.User.Account.Delete()).
		Update(ctx); err != nil {
		t.Fatalf("has one delete failed: %v", err)
	}
	var cnt int64
	if err := db.Model(&models.Account{}).Where("number = ?", "A-DEL").Count(&cnt).Error; err != nil {
		t.Fatalf("count after delete failed: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("expected 0 accounts after delete, got %d", cnt)
	}
}

func TestAssociation_BelongsTo_Company(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db)
	u := users[0]
	ctx := context.Background()

	// Seed company and set user's CompanyID
	comp := models.Company{Name: "Acme"}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("seed company failed: %v", err)
	}
	if err := db.Model(&models.User{}).Where("id = ?", u.ID).Update("company_id", comp.ID).Error; err != nil {
		t.Fatalf("set user company_id failed: %v", err)
	}
	var uu models.User
	if err := db.First(&uu, u.ID).Error; err != nil {
		t.Fatalf("reload user failed: %v", err)
	}

	// Update company via association
	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(generated.User.Company.Where(generated.Company.Name.Eq("Acme")).Update(generated.Company.Name.Set("NewCo"))).
		Update(ctx); err != nil {
		t.Fatalf("belongs to update failed: %v", err)
	}
	var comp2 models.Company
	if err := db.First(&comp2, *uu.CompanyID).Error; err != nil {
		t.Fatalf("reload company failed: %v", err)
	}
	if comp2.Name != "NewCo" {
		t.Fatalf("expected company name NewCo, got %s", comp2.Name)
	}

	// Unlink should set CompanyID NULL and keep company row
	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(generated.User.Company.Unlink()).
		Update(ctx); err != nil {
		t.Fatalf("belongs to unlink failed: %v", err)
	}
	var uu2 models.User
	if err := db.First(&uu2, u.ID).Error; err != nil {
		t.Fatalf("reload user 2 failed: %v", err)
	}
	if uu2.CompanyID != nil {
		t.Fatalf("expected CompanyID NULL after unlink, got %#v", uu2.CompanyID)
	}
	// company row should still exist
	var cnt int64
	if err := db.Model(&models.Company{}).Where("name = ?", "NewCo").Count(&cnt).Error; err != nil {
		t.Fatalf("count company failed: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("expected 1 company remain, got %d", cnt)
	}
}

func TestAssociation_Many2Many_Languages(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db)
	u := users[0]
	ctx := context.Background()

	// Create and link a language
	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(generated.User.Languages.Create(generated.Language.Code.Set("EN"), generated.Language.Name.Set("English"))).
		Update(ctx); err != nil {
		t.Fatalf("m2m create failed: %v", err)
	}
	count := db.Model(&u).Association("Languages").Count()
	if count != 1 {
		t.Fatalf("expected 1 language associated, got %d", count)
	}

	// Update associated language value
	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(generated.User.Languages.Where(generated.Language.Code.Eq("EN")).Update(generated.Language.Name.Set("English-US"))).
		Update(ctx); err != nil {
		t.Fatalf("m2m update failed: %v", err)
	}
	var lang models.Language
	if err := db.Where("code = ?", "EN").First(&lang).Error; err != nil {
		t.Fatalf("load language failed: %v", err)
	}
	if lang.Name != "English-US" {
		t.Fatalf("expected language name updated, got %s", lang.Name)
	}

	// Unlink should remove join row but keep language
	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(generated.User.Languages.Where(generated.Language.Code.Eq("EN")).Unlink()).
		Update(ctx); err != nil {
		t.Fatalf("m2m unlink failed: %v", err)
	}
	count = db.Model(&u).Association("Languages").Count()
	if count != 0 {
		t.Fatalf("expected 0 languages associated after unlink, got %d", count)
	}

	// Link again then Delete; expect join removed, language remains
	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(
			generated.User.Languages.Create(
				generated.Language.Code.Set("EN2"),
				generated.Language.Name.Set("English-2"),
			),
		).
		Update(ctx); err != nil {
		t.Fatalf("relink language failed: %v", err)
	}
	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(generated.User.Languages.Where(generated.Language.Code.Eq("EN2")).Delete()).
		Update(ctx); err != nil {
		t.Fatalf("m2m delete failed: %v", err)
	}
	var cnt int64
	if err := db.Model(&models.Language{}).Where("code = ?", "EN2").Count(&cnt).Error; err != nil {
		t.Fatalf("count EN after delete failed: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("expected language EN2 to remain after delete (join removed), got %d rows", cnt)
	}
}

func TestAssociation_Many2Many_Friends(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db)
	u := users[0]
	ctx := context.Background()

	// Create and link a friend (creates new User row via association)
	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(generated.User.Friends.Create(
			generated.User.Name.Set("friend-a"),
			generated.User.Age.Set(10),
			generated.User.Role.Set("active"),
			generated.User.IsAdult.Set(false),
			generated.User.Score.Set(sql.NullInt64{}),
		)).
		Update(ctx); err != nil {
		t.Fatalf("m2m friends create failed: %v", err)
	}
	count := db.Model(&u).Association("Friends").Count()
	if count != 1 {
		t.Fatalf("expected 1 friend associated, got %d", count)
	}

	// Unlink friend
	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(generated.User.Friends.Where(generated.User.Name.Eq("friend-a")).Unlink()).
		Update(ctx); err != nil {
		t.Fatalf("m2m friends unlink failed: %v", err)
	}
	count = db.Model(&u).Association("Friends").Count()
	if count != 0 {
		t.Fatalf("expected 0 friends after unlink, got %d", count)
	}

	// Delete should remove join row, keep friend row
	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(generated.User.Friends.Where(generated.User.Name.Eq("friend-a")).Delete()).
		Update(ctx); err != nil {
		t.Fatalf("m2m friends delete failed: %v", err)
	}
	var cnt int64
	if err := db.Model(&models.User{}).Where("name = ?", "friend-a").Count(&cnt).Error; err != nil {
		t.Fatalf("count friend-a failed: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("expected friend-a row to remain after delete (join removed), got %d", cnt)
	}
}

// Polymorphic has-one: Pet.Toy (polymorphic: Owner)
func TestAssociation_Polymorphic_Toy_CreateUpdateUnlinkDelete(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db)
	u := users[0]
	// Seed one pet for user
	p := models.Pet{Name: "poly-pet", UserID: &u.ID}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("seed pet failed: %v", err)
	}

	ctx := context.Background()

	// Seed toy directly to ensure stable starting point
	toy := models.Toy{Name: "ball", OwnerID: p.ID, OwnerType: "pets"}
	if err := db.Create(&toy).Error; err != nil {
		t.Fatalf("seed toy failed: %v", err)
	}

	// Update toy name via association condition
	if _, err := gorm.G[models.Pet](db).
		Where(generated.Pet.ID.Eq(p.ID)).
		Set(generated.Pet.Toy.Where(generated.Toy.Name.Eq("ball")).Update(generated.Toy.Name.Set("cube"))).
		Update(ctx); err != nil {
		t.Fatalf("poly update toy failed: %v", err)
	}
	var toy2 models.Toy
	if err := db.Where("owner_id = ? AND name = ?", p.ID, "cube").First(&toy2).Error; err != nil {
		t.Fatalf("load updated toy failed: %v", err)
	}

	// Unlink (zero out foreign keys for polymorphic; row remains)
	if _, err := gorm.G[models.Pet](db).
		Where(generated.Pet.ID.Eq(p.ID)).
		Set(generated.Pet.Toy.Unlink()).
		Update(ctx); err != nil {
		t.Fatalf("poly unlink toy failed: %v", err)
	}
	var toy3 models.Toy
	if err := db.Where("id = ?", toy2.ID).First(&toy3).Error; err != nil {
		t.Fatalf("reload toy after unlink failed: %v", err)
	}
	if toy3.OwnerID != 0 {
		t.Fatalf("expected OwnerID=0 after unlink, got %d", toy3.OwnerID)
	}

	// Seed another toy for delete case
	toy4 := models.Toy{Name: "delme", OwnerID: p.ID, OwnerType: "pets"}
	if err := db.Create(&toy4).Error; err != nil {
		t.Fatalf("seed toy for delete failed: %v", err)
	}
	if _, err := gorm.G[models.Pet](db).
		Where(generated.Pet.ID.Eq(p.ID)).
		Set(generated.Pet.Toy.Delete()).
		Update(ctx); err != nil {
		t.Fatalf("poly delete toy failed: %v", err)
	}
	var cnt int64
	if err := db.Model(&models.Toy{}).Where("id = ?", toy4.ID).Count(&cnt).Error; err != nil {
		t.Fatalf("count toy after delete failed: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("expected toy row deleted, got %d", cnt)
	}
}

// Batch create: has-many via Values when supported.
func TestAssociation_CreateInBatch_HasMany(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db)
	u := users[0]
	ctx := context.Background()

	// Seed two pets (existing), then batch link them to the user via association
	p1 := models.Pet{Name: "bm1"}
	p2 := models.Pet{Name: "bm2"}

	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(generated.User.Pets.CreateInBatch([]models.Pet{p1, p2})).
		Update(ctx); err != nil {
		t.Fatalf("batch link has-many failed: %v", err)
	}

	// Verify both pets now linked to user
	cnt, err := gorm.G[models.Pet](db).
		Where("user_id = ?", u.ID).
		Where(generated.Pet.Name.In("bm1", "bm2")).
		Count(ctx, "*")
	if err != nil {
		t.Fatalf("count linked pets failed: %v", err)
	}
	if cnt != 2 {
		t.Fatalf("expected 2 linked pets, got %d", cnt)
	}
}

// Batch create: many2many via Values when supported.
func TestAssociation_CreateInBatch_Many2Many(t *testing.T) {
	db := setupTestDB(t)
	users := seedUsers(t, db)
	u := users[0]
	ctx := context.Background()

	// Seed two languages (existing), then batch link them to the user
	if err := db.Create(&[]models.Language{{Code: "B1", Name: "B1"}, {Code: "B2", Name: "B2"}}).Error; err != nil {
		t.Fatalf("seed languages failed: %v", err)
	}
	if _, err := gorm.G[models.User](db).
		Where(generated.User.ID.Eq(u.ID)).
		Set(generated.User.Languages.CreateInBatch([]models.Language{{Code: "B1"}, {Code: "B2"}})).
		Update(ctx); err != nil {
		t.Fatalf("batch link many2many failed: %v", err)
	}

	// Count associated languages
	count := db.Model(&u).Association("Languages").Count()
	if count != 2 {
		t.Fatalf("expected 2 languages associated after batch create, got %d", count)
	}
}
