package migration

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"gorm.io/cli/gorm/internal/project"
)

// New returns the `gorm migrate` command tree described in the README.
func New() *cobra.Command {
	mgr := &Manager{}

	cmd := &cobra.Command{
		Use:           "migrate",
		Short:         "Manage database migrations and schema changes",
		Long:          strings.TrimSpace("DB→Model reflection, Model→Migration generation, and diff checks."),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&mgr.ModelsDir, "models", defaultModelsDirName, "Directory to place generated models")
	cmd.PersistentFlags().StringVar(&mgr.MigrationsDir, "migrations", defaultMigrationsDirName, "Directory to place generated migration files")
	cmd.PersistentFlags().StringVar(&mgr.GoCmd, "go", "go", "Go command to run migration runner")

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

func newInitCmd(mgr *Manager) *cobra.Command {
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

func newUpCmd(mgr *Manager) *cobra.Command {
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
			return mgr.runRunner(cmd, "up", flags)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 0, "Number of migrations to apply (default applies all)")

	return cmd
}

func newDownCmd(mgr *Manager) *cobra.Command {
	var steps int

	cmd := &cobra.Command{
		Use:          "down",
		Short:        "Rollback previously applied migrations",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			flags := []string{fmt.Sprintf("--steps=%d", steps)}
			return mgr.runRunner(cmd, "down", flags)
		},
	}

	cmd.Flags().IntVar(&steps, "steps", 1, "Number of migrations to rollback")

	return cmd
}

func newStatusCmd(mgr *Manager) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "status",
		Short:        "Show applied and pending migrations",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return mgr.runRunner(cmd, "status", nil)
		},
	}

	return cmd
}

func newDiffCmd(mgr *Manager) *cobra.Command {
	var generated bool
	cmd := &cobra.Command{
		Use:          "diff",
		Short:        "Model ↔ DB diff (read-only)",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := mgr.generateDiffFile()
			if err != nil {
				return err
			}
			cleanup := cleanupDiffArtifact(path)
			if cleanup != nil {
				defer cleanup()
			}
			var subArgs []string
			if generated {
				subArgs = append(subArgs, "--generated-file")
			}
			return mgr.runRunner(cmd, "diff", subArgs)
		},
	}
	cmd.Flags().BoolVar(&generated, "generated-file", false, "internal flag used for diff helper generation")
	_ = cmd.Flags().MarkHidden("generated-file")

	return cmd
}

func newReflectCmd(mgr *Manager) *cobra.Command {
	var dryRun bool
	var yes bool

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
			return mgr.runRunner(cmd, "reflect", subArgs)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview generated code without writing to disk")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts")

	return cmd
}

func newCreateCmd(mgr *Manager) *cobra.Command {
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
			var cleanup func()
			if dryRun {
				subArgs = append(subArgs, "--dry-run")
			}
			if yes {
				subArgs = append(subArgs, "--yes")
			}
			if auto {
				subArgs = append(subArgs, "--auto")
				path, err := mgr.generateDiffFile()
				if err != nil {
					return err
				}
				cleanup = cleanupDiffArtifact(path)
			}
			if cleanup != nil {
				defer cleanup()
			}
			return mgr.runRunner(cmd, "create", subArgs)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview migration contents without creating a file")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&auto, "auto", false, "Generate from model/DB diff (requires DB adapter)")

	return cmd
}

func (mgr *Manager) runRunner(cmd *cobra.Command, subcommand string, args []string) error {
	goArgs := append([]string{"run", ".", subcommand}, args...)
	proc := exec.CommandContext(cmd.Context(), mgr.GoCmd, goArgs...)
	proc.Dir = project.ResolveRootPath(mgr.MigrationsDir)
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

func cleanupDiffArtifact(path string) func() {
	if path == "" {
		return nil
	}
	return func() {
		if os.Getenv("GORM_KEEP_DIFF_FILE") == "1" {
			return
		}
		_ = os.Remove(path)
	}
}
