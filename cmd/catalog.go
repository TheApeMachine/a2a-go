package cmd

import (
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/theapemachine/a2a-go/pkg/service"
)

var (
	catalogCmd = &cobra.Command{
		Use:   "catalog",
		Short: "Run the agent catalog",
		Long:  longCatalog,
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetReportCaller(true)
			log.SetLevel(log.InfoLevel)

			return service.NewCatalogServer().Run()
		},
	}
)

func init() {
	rootCmd.AddCommand(catalogCmd)
}

var longCatalog = `
Serve the agent catalog.

Examples:
  # Serve the agent catalog on port 3210.
  a2a-go catalog
`
