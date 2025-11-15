package migration

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// New returns the `gorm migrate` command tree described in the README.
func New() *cobra.Command {
	var mgr Manager

	cmd := &cobra.Command{
		Use:          "migrate",
		Short:        "Manage database migrations and schema changes",
		Long:         strings.TrimSpace("Run schema diff powered migrations.\n Initialize ./migrations/main.go and add timestamped Go migration files."),
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVar(&mgr.ModelsDir, "models", defaultModelsDirName, "Directory to place generated models")
	cmd.PersistentFlags().StringVar(&mgr.MigrationsDir, "migrations", defaultMigrationsDirName, "Directory to place generated migration files")

	cmd.AddCommand(
		newInitCmd(mgr),
		newUpCmd(&mgr.MigrationsDir),
		newDownCmd(&mgr.MigrationsDir),
		newStatusCmd(&mgr.MigrationsDir),
		newDiffCmd(&mgr.MigrationsDir),
		newGenCmd(&mgr.MigrationsDir),
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

func newUpCmd(projectDir *string) *cobra.Command {
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
			return runProject(cmd, *projectDir, "up", flags)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Number of migrations to apply (default applies all)")

	return cmd
}

func newDownCmd(projectDir *string) *cobra.Command {
	var steps int

	cmd := &cobra.Command{
		Use:          "down",
		Short:        "Rollback previously applied migrations",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := []string{fmt.Sprintf("--steps=%d", steps)}
			return runProject(cmd, *projectDir, "down", flags)
		},
	}

	cmd.Flags().IntVar(&steps, "steps", 1, "Number of migrations to rollback")

	return cmd
}

func newStatusCmd(projectDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "status",
		Short:        "Show applied and pending migrations",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProject(cmd, *projectDir, "status", nil)
		},
	}

	return cmd
}

func newDiffCmd(projectDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "diff",
		Short:        "Inspect differences between models and the database",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProject(cmd, *projectDir, "diff", nil)
		},
	}

	return cmd
}

func newGenCmd(projectDir *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "gen",
		Short:        "Generate models or migration files",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newGenModelCmd(projectDir))
	cmd.AddCommand(newGenMigrationCmd(projectDir))

	return cmd
}

func newGenModelCmd(projectDir *string) *cobra.Command {
	var dryRun bool
	var yes bool
	var tables []string

	cmd := &cobra.Command{
		Use:          "model",
		Short:        "Generate or update GORM models from the database schema",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			subArgs := []string{"model"}
			if dryRun {
				subArgs = append(subArgs, "--dry-run")
			}
			if yes {
				subArgs = append(subArgs, "--yes")
			}
			for _, table := range tables {
				subArgs = append(subArgs, fmt.Sprintf("--table=%s", table))
			}
			return runProject(cmd, *projectDir, "gen", subArgs)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview generated code without writing to disk")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts")
	cmd.Flags().StringSliceVar(&tables, "table", nil, "Limit generation to specific tables (repeatable)")

	return cmd
}

func newGenMigrationCmd(projectDir *string) *cobra.Command {
	var name string
	var dryRun bool
	var yes bool

	cmd := &cobra.Command{
		Use:          "migration",
		Short:        "Generate a Go migration file from model diffs",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			subArgs := []string{"migration", fmt.Sprintf("--name=%s", name)}
			if dryRun {
				subArgs = append(subArgs, "--dry-run")
			}
			if yes {
				subArgs = append(subArgs, "--yes")
			}
			return runProject(cmd, *projectDir, "gen", subArgs)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Descriptive migration name (e.g. add_users_table)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview migration contents without creating a file")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts")
	_ = cmd.MarkFlagRequired("name")

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
		if errors.Is(err, fs.ErrNotExist) {
			return ErrNotInitialized
		}
		return err
	}

	goArgs := append([]string{"run", ".", subcommand}, args...)
	proc := exec.CommandContext(cmd.Context(), "go", goArgs...)
	proc.Dir = migrationsDir
	proc.Stdout = cmd.OutOrStdout()
	proc.Stderr = cmd.ErrOrStderr()
	proc.Stdin = cmd.InOrStdin()
	proc.Env = os.Environ()
	return proc.Run()
}

func normalizeProjectDir(value string) string {
	if value == "" {
		return "."
	}
	return filepath.Clean(value)
}
