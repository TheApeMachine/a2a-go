package cmd

import (
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/theapemachine/a2a-go/examples"
)

type Example interface {
	Run(interactive bool) error
}

var (
	interactive bool

	exampleCmd = &cobra.Command{
		Use:   "example",
		Short: "Run an example agent",
		Long:  longExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetReportCaller(true)
			log.SetCallerOffset(0)
			log.SetLevel(log.DebugLevel)

			switch args[0] {
			case "developer":
				return examples.NewDeveloperExample().Run(interactive)
			}

			log.Error("unknown example", "example", args[0])
			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(exampleCmd)

	exampleCmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "run the agent in interactive mode")
}

var longExample = `
Run an example agent with various configurations.

Examples:
  a2a-go example developer
`
