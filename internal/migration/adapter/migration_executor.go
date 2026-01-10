package adapter

import (
	"fmt"
	"os"
	"sort"
	"time"

	"gorm.io/gorm"
)

// MigrationExecutor handles the execution of migrations (Up/Down/Status operations).
type MigrationExecutor struct {
	db         *gorm.DB
	migrations map[string]Migration
}

// NewMigrationExecutor creates a new migration executor.
func NewMigrationExecutor(db *gorm.DB, migrations map[string]Migration) *MigrationExecutor {
	return &MigrationExecutor{
		db:         db,
		migrations: migrations,
	}
}

// schemaMigration type is defined in db_adapter.go to avoid duplication

// Up applies pending migrations up to the limit.
func (e *MigrationExecutor) Up(opts UpOptions) error {
	if err := e.ensureSchemaTable(); err != nil {
		return err
	}

	pending, err := e.pendingMigrations()
	if err != nil {
		return err
	}

	if len(pending) == 0 {
		fmt.Fprintln(os.Stdout, "No pending migrations")
		return nil
	}

	if opts.Limit > 0 && opts.Limit < len(pending) {
		pending = pending[:opts.Limit]
	}

	for _, m := range pending {
		if err := e.db.Transaction(func(tx *gorm.DB) error {
			if err := m.Up(tx); err != nil {
				return err
			}
			return e.recordApplied(tx, m.Name)
		}); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.Name, err)
		}
		fmt.Fprintf(os.Stdout, "Applied %s\n", m.Name)
	}

	return nil
}

// Down rolls back applied migrations.
func (e *MigrationExecutor) Down(opts DownOptions) error {
	steps := opts.Steps
	if steps <= 0 {
		steps = 1
	}

	if err := e.ensureSchemaTable(); err != nil {
		return err
	}

	applied, err := e.appliedMigrationsDesc()
	if err != nil {
		return err
	}

	if len(applied) == 0 {
		fmt.Fprintln(os.Stdout, "No applied migrations")
		return nil
	}

	if steps > len(applied) {
		steps = len(applied)
	}

	for i := 0; i < steps; i++ {
		record := applied[i]
		mig, ok := e.migrations[record.Name]
		if !ok {
			return fmt.Errorf("migration %s not registered", record.Name)
		}

		if mig.Down == nil {
			return fmt.Errorf("migration %s has no Down function", record.Name)
		}

		if err := e.db.Transaction(func(tx *gorm.DB) error {
			if err := mig.Down(tx); err != nil {
				return err
			}
			return e.removeApplied(tx, record.Name)
		}); err != nil {
			return fmt.Errorf("rollback migration %s: %w", record.Name, err)
		}

		fmt.Fprintf(os.Stdout, "Rolled back %s\n", record.Name)
	}

	return nil
}

// Status prints the status of all migrations.
func (e *MigrationExecutor) Status(_ StatusOptions) error {
	if err := e.ensureSchemaTable(); err != nil {
		return err
	}

	applied, err := e.appliedMigrationsAsc()
	if err != nil {
		return err
	}

	appliedSet := make(map[string]time.Time, len(applied))
	for _, record := range applied {
		appliedSet[record.Name] = record.AppliedAt
	}

	regs := e.registeredMigrations()

	fmt.Fprintln(os.Stdout, "NAME\tSTATUS\tAPPLIED AT")
	for _, mig := range regs {
		if ts, ok := appliedSet[mig.Name]; ok {
			fmt.Fprintf(os.Stdout, "%s\tapplied\t%s\n", mig.Name, ts.UTC().Format(time.RFC3339))
		} else {
			fmt.Fprintf(os.Stdout, "%s\tpending\t-\n", mig.Name)
		}
	}

	fmt.Fprintf(os.Stdout, "Total: %d | Applied: %d | Pending: %d\n",
		len(regs), len(applied), len(regs)-len(applied))

	return nil
}

func (e *MigrationExecutor) ensureSchemaTable() error {
	// schemaMigration struct is defined in db_adapter.go
	type schemaMigration struct {
		Name      string    `gorm:"primaryKey;size:200"`
		AppliedAt time.Time `gorm:"autoUpdateTime:false"`
	}
	return e.db.AutoMigrate(&schemaMigration{})
}

func (e *MigrationExecutor) recordApplied(tx *gorm.DB, name string) error {
	if len(name) > 150 {
		return fmt.Errorf("migration name exceeds 150 characters: %s", name)
	}

	if tx == nil {
		tx = e.db
	}

	type schemaMigration struct {
		Name      string    `gorm:"primaryKey;size:200"`
		AppliedAt time.Time `gorm:"autoUpdateTime:false"`
	}
	return tx.Create(&schemaMigration{Name: name, AppliedAt: time.Now().UTC()}).Error
}

func (e *MigrationExecutor) removeApplied(tx *gorm.DB, name string) error {
	if tx == nil {
		tx = e.db
	}

	type schemaMigration struct {
		Name      string    `gorm:"primaryKey;size:200"`
		AppliedAt time.Time `gorm:"autoUpdateTime:false"`
	}
	return tx.Delete(&schemaMigration{Name: name}).Error
}

func (e *MigrationExecutor) appliedMigrationsAsc() ([]struct {
	Name      string
	AppliedAt time.Time
}, error) {
	type schemaMigration struct {
		Name      string    `gorm:"primaryKey;size:200"`
		AppliedAt time.Time `gorm:"autoUpdateTime:false"`
	}
	var records []schemaMigration
	if err := e.db.Order("name asc").Find(&records).Error; err != nil {
		return nil, err
	}

	result := make([]struct {
		Name      string
		AppliedAt time.Time
	}, len(records))
	for i, r := range records {
		result[i].Name = r.Name
		result[i].AppliedAt = r.AppliedAt
	}
	return result, nil
}

func (e *MigrationExecutor) appliedMigrationsDesc() ([]struct {
	Name      string
	AppliedAt time.Time
}, error) {
	type schemaMigration struct {
		Name      string    `gorm:"primaryKey;size:200"`
		AppliedAt time.Time `gorm:"autoUpdateTime:false"`
	}
	var records []schemaMigration
	if err := e.db.Order("applied_at desc").Find(&records).Error; err != nil {
		return nil, err
	}

	result := make([]struct {
		Name      string
		AppliedAt time.Time
	}, len(records))
	for i, r := range records {
		result[i].Name = r.Name
		result[i].AppliedAt = r.AppliedAt
	}
	return result, nil
}

func (e *MigrationExecutor) pendingMigrations() ([]Migration, error) {
	applied, err := e.appliedMigrationsAsc()
	if err != nil {
		return nil, err
	}

	appliedSet := make(map[string]struct{}, len(applied))
	for _, record := range applied {
		appliedSet[record.Name] = struct{}{}
	}

	regs := e.registeredMigrations()
	pending := make([]Migration, 0)

	for _, mig := range regs {
		if _, ok := appliedSet[mig.Name]; !ok {
			pending = append(pending, mig)
		}
	}

	return pending, nil
}

func (e *MigrationExecutor) registeredMigrations() []Migration {
	if len(e.migrations) == 0 {
		return nil
	}

	out := make([]Migration, 0, len(e.migrations))
	for _, m := range e.migrations {
		out = append(out, m)
	}

	// Sort by name for deterministic order
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})

	return out
}
