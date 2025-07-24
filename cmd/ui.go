package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/theapemachine/a2a-go/pkg/ui"
)

var (
	uiCmd = &cobra.Command{
		Use:   "ui",
		Short: "Run an A2A UI",
		Long:  longUI,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := os.Getenv("TEA_LOGFILE")
			if path != "" {
				f, err := tea.LogToFile(path, "layers")
				if err != nil {
					fmt.Println("could not open logfile:", err)
					os.Exit(1)
				}
				defer f.Close()
			}

			if _, err := tea.NewProgram(ui.New(), tea.WithAltScreen(), tea.WithMouseAllMotion()).Run(); err != nil {
				fmt.Println("Error while running program:", err)
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
