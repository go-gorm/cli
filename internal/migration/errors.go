package migration

import "fmt"

// ErrNotInitialized is returned when the migrations folder has not been bootstrapped yet.
type ErrNotInitialized struct {
	Dir string
}

func (e ErrNotInitialized) Error() string {
	return fmt.Sprintf("migration project is not initialized in %s; run 'gorm migrate init'", e.Dir)
}

// ErrMigrationNotFound is returned when a migration cannot be found.
type ErrMigrationNotFound struct {
	Name string
}

func (e ErrMigrationNotFound) Error() string {
	return fmt.Sprintf("migration not found: %s", e.Name)
}

// ErrRunnerFailed is returned when the migration runner process fails.
type ErrRunnerFailed struct {
	Cmd      string
	ExitCode int
	Output   string
}

func (e ErrRunnerFailed) Error() string {
	if e.Output != "" {
		return fmt.Sprintf("migration runner failed (exit %d): %s\nOutput: %s", e.ExitCode, e.Cmd, e.Output)
	}
	return fmt.Sprintf("migration runner failed (exit %d): %s", e.ExitCode, e.Cmd)
}

// ErrInvalidMigration is returned when a migration is invalid.
type ErrInvalidMigration struct {
	Name   string
	Reason string
}

func (e ErrInvalidMigration) Error() string {
	return fmt.Sprintf("invalid migration %s: %s", e.Name, e.Reason)
}

// ErrAdapterRequired is returned when an operation requires a DB adapter but none is provided.
type ErrAdapterRequired struct {
	Operation string
}

func (e ErrAdapterRequired) Error() string {
	return fmt.Sprintf("operation %s requires a database adapter", e.Operation)
}
