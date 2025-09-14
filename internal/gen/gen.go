package gen

import (
	"fmt"

	"github.com/spf13/cobra"
)

var defaultOutPath = "./g"

func New() *cobra.Command {
	var typed bool
	var input, output string

	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Generate GORM query code from raw SQL interfaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			g := Generator{
				Typed:   typed,
				Files:   map[string]*File{},
				outPath: output,
			}

			err := g.Process(input)
			if err != nil {
				return fmt.Errorf("error processing %s: %v", input, err)
			}

			err = g.Gen()
			if err != nil {
				return fmt.Errorf("error render template got error: %v", err)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&typed, "typed", "t", false, "Generated Typed API")
	cmd.Flags().StringVarP(&output, "output", "o", defaultOutPath, "Directory to place generated code")
	cmd.Flags().StringVarP(&input, "input", "i", "", "Path to Go interface file with raw SQL annotations")
	cmd.MarkFlagRequired("input")

	return cmd
}
