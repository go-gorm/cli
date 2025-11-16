package migration

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

// New returns the `gorm migrate` command tree described in the README.
func New() *cobra.Command {
	var mgr Manager

	cmd := &cobra.Command{
		Use:           "migrate",
		Short:         "Manage database migrations and schema changes",
		Long:          strings.TrimSpace("DB→Model reflection, Model→Migration generation, and diff checks."),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&mgr.ModelsDir, "models", defaultModelsDirName, "Directory to place generated models")
	cmd.PersistentFlags().StringVar(&mgr.MigrationsDir, "migrations", defaultMigrationsDirName, "Directory to place generated migration files")

	cmd.AddCommand(
		newInitCmd(mgr),
		newUpCmd(mgr),
		newDownCmd(mgr),
		newStatusCmd(mgr),
		newDiffCmd(mgr),
		newReflectCmd(mgr),
		newCreateCmd(mgr),
	)

	return cmd
}

func newInitCmd(mgr Manager) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:          "init",
		Short:        "Initialize the migrations directories",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := mgr.Init(InitOptions{Force: force})
			if err != nil {
				return err
			}

			cmd.Printf("Initialized migration directory: %s\n", mgr.MigrationsDir)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing files")

	return cmd
}

func newUpCmd(mgr Manager) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:          "up",
		Short:        "Apply pending migrations",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			var flags []string
			if limit > 0 {
				flags = append(flags, fmt.Sprintf("--limit=%d", limit))
			}
			return runProject(cmd, mgr.MigrationsDir, "up", flags)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Number of migrations to apply (default applies all)")

	return cmd
}

func newDownCmd(mgr Manager) *cobra.Command {
	var steps int

	cmd := &cobra.Command{
		Use:          "down",
		Short:        "Rollback previously applied migrations",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := []string{fmt.Sprintf("--steps=%d", steps)}
			return runProject(cmd, mgr.MigrationsDir, "down", flags)
		},
	}

	cmd.Flags().IntVar(&steps, "steps", 1, "Number of migrations to rollback")

	return cmd
}

func newStatusCmd(mgr Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "status",
		Short:        "Show applied and pending migrations",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProject(cmd, mgr.MigrationsDir, "status", nil)
		},
	}

	return cmd
}

func newDiffCmd(mgr Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "diff",
		Short:        "Model ↔ DB diff (read-only)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProject(cmd, mgr.MigrationsDir, "diff", nil)
		},
	}

	return cmd
}

func newReflectCmd(mgr Manager) *cobra.Command {
	var dryRun bool
	var yes bool
	var tables []string

	cmd := &cobra.Command{
		Use:          "reflect",
		Short:        "Reflect DB schema into models (DB → Model)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			subArgs := []string{}
			if dryRun {
				subArgs = append(subArgs, "--dry-run")
			}
			if yes {
				subArgs = append(subArgs, "--yes")
			}
			for _, table := range tables {
				subArgs = append(subArgs, fmt.Sprintf("--table=%s", table))
			}
			return runProject(cmd, mgr.MigrationsDir, "reflect", subArgs)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview generated code without writing to disk")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts")
	cmd.Flags().StringSliceVar(&tables, "table", nil, "Limit generation to specific tables (repeatable)")

	return cmd
}

func newCreateCmd(mgr Manager) *cobra.Command {
	var dryRun bool
	var yes bool
	var auto bool

	cmd := &cobra.Command{
		Use:          "create",
		Short:        "Generate a migration file from models (Model → Migration File)",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			subArgs := []string{name}
			if dryRun {
				subArgs = append(subArgs, "--dry-run")
			}
			if yes {
				subArgs = append(subArgs, "--yes")
			}
			if auto {
				subArgs = append(subArgs, "--auto")
			}
			return runProject(cmd, mgr.MigrationsDir, "create", subArgs)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview migration contents without creating a file")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&auto, "auto", false, "Generate from model/DB diff (requires DB adapter)")

	return cmd
}

func runProject(cmd *cobra.Command, projectDir, subcommand string, args []string) error {
	projectDir = normalizeProjectDir(projectDir)
	absProject, err := filepath.Abs(projectDir)
	if err != nil {
		return err
	}
	migrationsDir := filepath.Join(absProject, defaultMigrationsDirName)
	runner := filepath.Join(migrationsDir, defaultRunnerFileName)
	if _, err := os.Stat(runner); err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
			altRunner := filepath.Join(absProject, defaultRunnerFileName)
			if _, errAlt := os.Stat(altRunner); errAlt == nil {
				migrationsDir = absProject
				runner = altRunner
			} else {
				return ErrNotInitialized
			}
		} else {
			return err
		}
	}

	goArgs := append([]string{"run", ".", subcommand}, args...)
	proc := exec.CommandContext(cmd.Context(), "go", goArgs...)
	proc.Dir = migrationsDir
	proc.Stdout = cmd.OutOrStdout()
	proc.Stderr = cmd.ErrOrStderr()
	proc.Stdin = cmd.InOrStdin()
	proc.Env = os.Environ()
	if err := proc.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil
		}
		return err
	}
	return nil
}

func normalizeProjectDir(value string) string {
	if value == "" {
		return "."
	}
	return filepath.Clean(value)
}
