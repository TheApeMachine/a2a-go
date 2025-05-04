package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/catalog"
)

var (
	uiCmd = &cobra.Command{
		Use:   "ui",
		Short: "Run an A2A UI",
		Long:  longUI,
		RunE: func(cmd *cobra.Command, args []string) error {
			catalogClient := catalog.NewCatalogClient("http://catalog:3210")

			var (
				agentCards []a2a.AgentCard
				err        error
				attempts   = 1
			)

			for len(agentCards) == 0 {
				if agentCards, err = catalogClient.GetAgents(); err != nil {
					log.Error("failed to get agents", "error", err)
				}

				time.Sleep(time.Duration(attempts) * time.Second)
				attempts += 1
			}

			options := make([]huh.Option[string], 0)

			for _, card := range agentCards {
				options = append(options, huh.NewOption(card.Name, card.Name))
			}

			var (
				agent    string
				prompt   string
				selected *a2a.AgentCard
			)

			form := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Choose your agent").
						Options(options...).
						Value(&agent),
				),

				// Gather some final details about the order.
				huh.NewGroup(
					huh.NewInput().
						Title("Prompt").
						Value(&prompt),
				),
			)

			if err := form.Run(); err != nil {
				return err
			}

			for _, card := range agentCards {
				if card.Name == agent {
					selected = &card
					break
				}
			}

			agentClient := a2a.NewClient(selected.URL)

			response, err := agentClient.SendTask(a2a.TaskSendParams{
				ID:        uuid.New().String(),
				SessionID: uuid.New().String(),
				Message:   *a2a.NewTextMessage("user", prompt),
			})

			if err != nil {
				log.Error("failed to send task", "error", err)
				return err
			}

			// Convert the response result to a Task
			task := &a2a.Task{}
			if err := json.Unmarshal(response.Result.([]byte), task); err != nil {
				log.Error("failed to unmarshal task response", "error", err)
				return err
			}

			// Print the task history
			for _, message := range task.History {
				fmt.Println(message.String())
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
