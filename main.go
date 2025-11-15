package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gorm.io/cli/gorm/internal/gen"
	"gorm.io/cli/gorm/internal/migration"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "gorm",
		Short: "GORM CLI Tool",
	}

	rootCmd.AddCommand(gen.New())
	rootCmd.AddCommand(migration.New())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
