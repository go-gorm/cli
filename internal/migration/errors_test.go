package migration

import (
	"testing"
)

func TestErrNotInitialized(t *testing.T) {
	err := ErrNotInitialized{Dir: "/test/migrations"}
	expected := "migration project is not initialized in /test/migrations; run 'gorm migrate init'"

	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestErrMigrationNotFound(t *testing.T) {
	err := ErrMigrationNotFound{Name: "20240101_init"}
	expected := "migration not found: 20240101_init"

	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestErrRunnerFailed(t *testing.T) {
	tests := []struct {
		name     string
		err      ErrRunnerFailed
		contains string
	}{
		{
			name: "without output",
			err: ErrRunnerFailed{
				Cmd:      "up",
				ExitCode: 1,
			},
			contains: "exit 1",
		},
		{
			name: "with output",
			err: ErrRunnerFailed{
				Cmd:      "down",
				ExitCode: 2,
				Output:   "migration failed",
			},
			contains: "migration failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			if !containsSubstring(msg, tt.contains) {
				t.Errorf("error message %q should contain %q", msg, tt.contains)
			}
		})
	}
}

func TestErrInvalidMigration(t *testing.T) {
	err := ErrInvalidMigration{Name: "bad_migration", Reason: "missing Up function"}
	expected := "invalid migration bad_migration: missing Up function"

	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestErrAdapterRequired(t *testing.T) {
	err := ErrAdapterRequired{Operation: "reflect"}
	expected := "operation reflect requires a database adapter"

	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
