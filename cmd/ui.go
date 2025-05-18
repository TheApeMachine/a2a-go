package cmd

import (

	// Import standard log for fallback if charmbracelet/log setup fails for file.

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log" // This is the charmbracelet logger
	"github.com/spf13/cobra"
	"github.com/theapemachine/a2a-go/pkg/ui"
)

var (
	uiCmd = &cobra.Command{
		Use:   "ui",
		Short: "Run an A2A UI",
		Long:  longUI,
		RunE: func(cmd *cobra.Command, args []string) error {
			// f, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

			// if err != nil {
			// 	log.Error("failed to open debug log file", "error", err)
			// 	return err
			// }

			// log.SetOutput(f)
			log.SetLevel(log.DebugLevel)
			log.SetReportCaller(true)
			// defer f.Close()

			if _, err := tea.NewProgram(
				safeApp{App: ui.NewApp("http://catalog:3210")},
				tea.WithAltScreen(),
			).Run(); err != nil {
				log.Error("failed to run program", "error", err)
				return err
			}

			return nil
		},
	}
)

func init() {
	rootCmd.AddCommand(uiCmd)
}

// Create a wrapper around our App that uses SafeUpdate
type safeApp struct {
	*ui.App
}

func (s safeApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Use our SafeUpdate method instead of the regular Update
	model, cmd := s.App.SafeUpdate(msg)

	// Convert the returned model back to safeApp
	if app, ok := model.(*ui.App); ok {
		return safeApp{App: app}, cmd
	}

	return model, cmd
}

var longUI = `
Serve an A2A UI with various configurations.

Examples:
  # Serve an A2A UI with the ui configuration.
  a2a-go ui
`
