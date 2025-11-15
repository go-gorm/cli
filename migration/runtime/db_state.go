package runtime

import (
	"time"
)

type schemaMigration struct {
	Name      string    `gorm:"primaryKey;size:255"`
	AppliedAt time.Time `gorm:"autoUpdateTime:false"`
}

func (schemaMigration) TableName() string {
	return "schema_migrations"
}

func (a *DBAdapter) recordApplied(name string) error {
	return a.db.Create(&schemaMigration{Name: name, AppliedAt: time.Now().UTC()}).Error
}

func (a *DBAdapter) removeApplied(name string) error {
	return a.db.Delete(&schemaMigration{Name: name}).Error
}

func (a *DBAdapter) appliedMigrationsAsc() ([]schemaMigration, error) {
	var records []schemaMigration
	if err := a.db.Order("name asc").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (a *DBAdapter) appliedMigrationsDesc() ([]schemaMigration, error) {
	var records []schemaMigration
	if err := a.db.Order("applied_at desc").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (a *DBAdapter) pendingMigrations() ([]Migration, error) {
	applied, err := a.appliedMigrationsAsc()
	if err != nil {
		return nil, err
	}
	appliedSet := make(map[string]struct{}, len(applied))
	for _, record := range applied {
		appliedSet[record.Name] = struct{}{}
	}
	regs := registeredMigrations()
	pending := make([]Migration, 0)
	for _, mig := range regs {
		if _, ok := appliedSet[mig.Name]; !ok {
			pending = append(pending, mig)
		}
	}
	return pending, nil
}
