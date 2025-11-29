package gen

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestNewCommandFlags verifies the command returned by New has the expected flags
// and that running it without the required input flag returns an error.
func TestNewCommandFlags(t *testing.T) {
	cmd := New()
	if cmd == nil {
		t.Fatal("New() returned nil command")
	}

	// Ensure flags exist
	if cmd.Flags().Lookup("typed") == nil {
		t.Fatalf("expected 'typed' flag to be present")
	}
	if cmd.Flags().Lookup("output") == nil {
		t.Fatalf("expected 'output' flag to be present")
	}
	if cmd.Flags().Lookup("input") == nil {
		t.Fatalf("expected 'input' flag to be present")
	}

	// Save original RunE and restore later.
	origRunE := cmd.RunE
	defer func() { cmd.RunE = origRunE }()

	// Replace RunE with a no-op to avoid side effects from file operations.
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		return nil
	}

	// Execute without setting input flag - cobra will check required flags during ParseFlags
	// Use cobra's Flag error handling by calling PreRunE if present then ExecuteC.
	_, err := cmd.ExecuteC()
	if err == nil {
		t.Fatalf("expected error when executing command without required 'input' flag")
	}

	// Now set the required flag and ensure no flag parsing error occurs.
	cmd.SetArgs([]string{"--input", "dummy.go"})
	_, err = cmd.ExecuteC()
	if err != nil {
		t.Fatalf("unexpected error executing command with input set: %v", err)
	}
}

// TestMarkFlagRequiredBehavior ensures cobra.MarkFlagRequired behaves as expected
func TestMarkFlagRequiredBehavior(t *testing.T) {
	cmd := &cobra.Command{Use: "x"}
	cmd.Flags().String("input", "", "input file")
	// MarkFlagRequired returns an error when flag not found; here it should not.
	err := cmd.MarkFlagRequired("input")
	if err != nil {
		t.Fatalf("expected no error from MarkFlagRequired, got: %v", err)
	}
}
