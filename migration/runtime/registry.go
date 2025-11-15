package runtime

import (
	"sort"
	"sync"

	"gorm.io/gorm"
)

// Migration represents a named schema change.
type Migration struct {
	Name string
	Up   func(tx *gorm.DB) error
	Down func(tx *gorm.DB) error
}

var (
	registryMu sync.Mutex
	registry   = make(map[string]Migration)
)

// RegisterMigration records a migration to be picked up by adapters.
func RegisterMigration(m Migration) {
	if m.Name == "" {
		panic("migration runtime: migration must have a name")
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[m.Name] = m
}

// registeredMigrations returns sorted migrations.
func registeredMigrations() []Migration {
	registryMu.Lock()
	defer registryMu.Unlock()
	if len(registry) == 0 {
		return nil
	}
	out := make([]Migration, 0, len(registry))
	for _, m := range registry {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func migrationByName(name string) (Migration, bool) {
	registryMu.Lock()
	defer registryMu.Unlock()
	m, ok := registry[name]
	return m, ok
}
