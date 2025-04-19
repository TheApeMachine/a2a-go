package cmd

import (
	"github.com/spf13/cobra"
)

var (
	serveCmd = &cobra.Command{
		Use:   "serve [hub|agent|tool]",
		Short: "Run Caramba services",
		Long:  longServe,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(serveCmd)
}

var longServe = `
Serve an A2A agent or tool.
`
