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
				log.Error("client.SendTask failed", "error", err)
				return err
			}

			// Check for JSON-RPC level error returned by the server
			if response.Error != nil {
				log.Error("RPC error from server", "code", response.Error.Code, "message", response.Error.Message, "data", response.Error.Data)
				return fmt.Errorf("server error: %s (code: %d)", response.Error.Message, response.Error.Code)
			}

			// If response.Error is nil, then response.Result should contain the actual result.
			if response.Result == nil {
				log.Error("RPC success, but server returned a nil result")
				return fmt.Errorf("server returned success but with a nil result")
			}

			var resultBytes []byte
			var marshalErr error

			// Attempt to convert response.Result to []byte for unmarshalling
			// Original code expected []byte, so try type assertion first.
			resultBytes, ok := response.Result.([]byte)
			if !ok {
				// If not already []byte, assume it might be a map[string]interface{} or similar
				// and try to marshal it to JSON bytes.
				log.Warn("response.Result is not []byte; attempting to marshal for unmarshalling", "type", fmt.Sprintf("%T", response.Result))
				resultBytes, marshalErr = json.Marshal(response.Result)
				if marshalErr != nil {
					log.Error("failed to marshal response.Result for unmarshalling", "error", marshalErr, "result_type", fmt.Sprintf("%T", response.Result))
					return fmt.Errorf("cannot process result of type %T: %w", response.Result, marshalErr)
				}
			}

			// Convert the response result to a Task
			task := &a2a.Task{}
			if err := json.Unmarshal(resultBytes, task); err != nil {
				log.Error("failed to unmarshal task response from resultBytes", "error", err)
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
