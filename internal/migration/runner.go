package migration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"gorm.io/cli/gorm/internal/project"
)

// Runner defines the interface for executing migration commands.
// This abstraction allows for easier testing and custom implementations.
type Runner interface {
	// Execute runs the given subcommand with arguments.
	Execute(ctx context.Context, subcommand string, args []string) error

	// Validate checks if the runner is properly configured.
	Validate() error
}

// GoRunner implements Runner using `go run` to execute migrations.
type GoRunner struct {
	MigrationsDir string
	GoCmd         string
	Stdout        interface{ Write([]byte) (int, error) }
	Stderr        interface{ Write([]byte) (int, error) }
	Stdin         interface{ Read([]byte) (int, error) }
}

// NewGoRunner creates a new GoRunner with default settings.
func NewGoRunner(migrationsDir string) *GoRunner {
	return &GoRunner{
		MigrationsDir: migrationsDir,
		GoCmd:         "go",
		Stdout:        os.Stdout,
		Stderr:        os.Stderr,
		Stdin:         os.Stdin,
	}
}

// Execute runs the migration subcommand using `go run`.
func (r *GoRunner) Execute(ctx context.Context, subcommand string, args []string) error {
	if err := r.Validate(); err != nil {
		return err
	}

	goArgs := append([]string{"run", ".", subcommand}, args...)
	proc := exec.CommandContext(ctx, r.GoCmd, goArgs...)
	proc.Dir = project.ResolveRootPath(r.MigrationsDir)
	proc.Stdout = r.Stdout
	proc.Stderr = r.Stderr
	proc.Stdin = r.Stdin
	proc.Env = os.Environ()

	if err := proc.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Migration runner handles its own errors, don't propagate exit errors
			return nil
		}
		return &ErrRunnerFailed{
			Cmd:      fmt.Sprintf("%s %s", subcommand, args),
			ExitCode: 1,
			Output:   err.Error(),
		}
	}

	return nil
}

// Validate checks if the GoRunner is properly configured.
func (r *GoRunner) Validate() error {
	if r.MigrationsDir == "" {
		return &ErrNotInitialized{Dir: "migrations"}
	}

	dir := project.ResolveRootPath(r.MigrationsDir)
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return &ErrNotInitialized{Dir: dir}
	}
	if err != nil {
		return fmt.Errorf("check migrations directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("migrations path is not a directory: %s", dir)
	}

	// Check if main.go exists
	mainFile := dir + "/main.go"
	if _, err := os.Stat(mainFile); os.IsNotExist(err) {
		return &ErrNotInitialized{Dir: dir}
	}

	return nil
}

// MockRunner is a test double for Runner interface.
type MockRunner struct {
	ExecuteFunc  func(ctx context.Context, subcommand string, args []string) error
	ValidateFunc func() error
	Calls        []MockRunnerCall
}

// MockRunnerCall records a call to the MockRunner.
type MockRunnerCall struct {
	Method     string
	Subcommand string
	Args       []string
}

// Execute implements Runner.Execute for testing.
func (m *MockRunner) Execute(ctx context.Context, subcommand string, args []string) error {
	m.Calls = append(m.Calls, MockRunnerCall{
		Method:     "Execute",
		Subcommand: subcommand,
		Args:       args,
	})
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, subcommand, args)
	}
	return nil
}

// Validate implements Runner.Validate for testing.
func (m *MockRunner) Validate() error {
	m.Calls = append(m.Calls, MockRunnerCall{Method: "Validate"})
	if m.ValidateFunc != nil {
		return m.ValidateFunc()
	}
	return nil
}
