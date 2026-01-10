package migration

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestGoRunner_Validate(t *testing.T) {
	tests := []struct {
		name          string
		migrationsDir string
		wantErr       bool
		errContains   string
	}{
		{
			name:          "empty migrations dir",
			migrationsDir: "",
			wantErr:       true,
			errContains:   "not initialized",
		},
		{
			name:          "non-existent directory",
			migrationsDir: "/non/existent/path",
			wantErr:       true,
			errContains:   "not initialized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewGoRunner(tt.migrationsDir)
			err := r.Validate()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestMockRunner(t *testing.T) {
	t.Run("records calls", func(t *testing.T) {
		mock := &MockRunner{}

		_ = mock.Validate()
		_ = mock.Execute(context.Background(), "up", []string{"--limit=1"})

		if len(mock.Calls) != 2 {
			t.Errorf("expected 2 calls, got %d", len(mock.Calls))
		}

		if mock.Calls[0].Method != "Validate" {
			t.Errorf("expected first call to be Validate, got %s", mock.Calls[0].Method)
		}

		if mock.Calls[1].Method != "Execute" {
			t.Errorf("expected second call to be Execute, got %s", mock.Calls[1].Method)
		}

		if mock.Calls[1].Subcommand != "up" {
			t.Errorf("expected subcommand 'up', got %s", mock.Calls[1].Subcommand)
		}
	})

	t.Run("custom execute func", func(t *testing.T) {
		var capturedCmd string
		mock := &MockRunner{
			ExecuteFunc: func(ctx context.Context, subcommand string, args []string) error {
				capturedCmd = subcommand
				return nil
			},
		}

		_ = mock.Execute(context.Background(), "status", nil)

		if capturedCmd != "status" {
			t.Errorf("expected captured cmd 'status', got %s", capturedCmd)
		}
	})
}

type mockWriter struct {
	buf bytes.Buffer
}

func (w *mockWriter) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}
