package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
	"gorm.io/cli/gorm/internal/gen"
)

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

func getVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}

	return "dev"
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version of gorm-cli",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("gorm-cli version %s\n", getVersion())
		},
	}
}
