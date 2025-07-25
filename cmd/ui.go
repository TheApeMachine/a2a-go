package cmd

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/theapemachine/a2a-go/pkg/ui"
)

var (
	uiCmd = &cobra.Command{
		Use:   "ui",
		Short: "Run an A2A UI",
		Long:  longUI,
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetReportCaller(true)
			log.SetLevel(log.InfoLevel)

			path := os.Getenv("TEA_LOGFILE")
			if path != "" {
				f, err := tea.LogToFile(path, "layers")
				if err != nil {
					log.Error("could not open logfile:", "error", err)
					os.Exit(1)
				}
				defer f.Close()
			}

			if _, err := tea.NewProgram(ui.New(), tea.WithAltScreen(), tea.WithMouseAllMotion()).Run(); err != nil {
				log.Error("Error while running program:", "error", err)
				os.Exit(1)
			}

			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(uiCmd)
}

var longUI = `
Serve an A2A UI with various configurations.

Examples:
  # Serve an A2A UI with the ui configuration.
  a2a-go ui
`
