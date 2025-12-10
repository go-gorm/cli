package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gorm.io/cli/gorm/internal/gen"
)

// Version can be set at build time via -ldflags "-X main.Version=x.y.z"
var Version = "v0.2.4"

func main() {
	rootCmd := &cobra.Command{
		Use:   "gorm",
		Short: "GORM CLI Tool",
	}

	rootCmd.AddCommand(gen.New())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of gorm-cli",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("gorm-cli version %s\n", Version)
		},
	}
}
