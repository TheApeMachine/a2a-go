package cmd

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/theapemachine/a2a-go/pkg/kubrick"
	"github.com/theapemachine/a2a-go/pkg/kubrick/components/spinner"
	"github.com/theapemachine/a2a-go/pkg/kubrick/layouts"
	"github.com/theapemachine/a2a-go/pkg/logging"
)

var (
	uiCmd = &cobra.Command{
		Use:   "ui",
		Short: "Run an A2A UI",
		Long:  longUI,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := logging.Init("a2a-ui.log"); err != nil {
				fmt.Printf("Failed to initialize logger: %v\n", err)
				// Decide if you want to exit or continue without file logging
				// For now, we'll proceed, and logs will go to stdout if GlobalLogger is nil.
			}
			defer logging.Close()

			logging.Log("Starting UI command")

			var wg sync.WaitGroup

			fmt.Println("Starting UI")

			wg.Add(1)
			app, err := kubrick.NewApp(
				kubrick.WithScreen(
					layouts.NewGridLayout(
						layouts.WithRows(1),
						layouts.WithColumns(1),
						layouts.WithSpacing(1),
						layouts.WithComponents(
							spinner.NewSpinner(),
						),
					),
				),
			)
			if err != nil {
				// Handle or return the error appropriately
				// For now, let's just return it, though logging might be better.
				return err
			}

			app.WithContext(cmd.Context())

			tick := time.Second / 60

			go func() {
				defer wg.Done()

				for {
					select {
					case <-cmd.Context().Done():
						logging.Log("UI command context done. Exiting copy loop.")
						return
					case <-time.Tick(tick):
						logging.Log("Tick: Before app.Write(\"X\\n\")")
						nWritten, errWrite := app.Write([]byte("X\n")) // Write X and a newline to the app (which should go to artifact)
						if errWrite != nil {
							logging.Log("Tick: app.Write error: %v", errWrite)
						}
						logging.Log("Tick: app.Write nWritten: %d", nWritten)

						logging.Log("Tick: Before io.Copy(os.Stdout, app)")
						nCopied, errCopy := io.Copy(os.Stdout, app)
						if errCopy != nil {
							logging.Log("Tick: io.Copy error: %v", errCopy)
						}
						logging.Log("Tick: io.Copy nCopied: %d", nCopied)
					}
				}
			}()

			logging.Log("UI command: Waiting on wg")
			wg.Wait()
			logging.Log("UI command: wg done. Closing app.")
			return app.Close() // Ensure the app is closed and return its error if any

			// catalogClient := catalog.NewCatalogClient("http://catalog:3210")

			// var (
			// 	agentCards []a2a.AgentCard
			// 	err        error
			// 	attempts   = 1
			// )

			// for len(agentCards) == 0 {
			// 	if agentCards, err = catalogClient.GetAgents(); err != nil {
			// 		log.Error("failed to get agents", "error", err)
			// 	}

			// 	time.Sleep(time.Duration(attempts) * time.Second)
			// 	attempts += 1
			// }

			// options := make([]huh.Option[string], 0)

			// for _, card := range agentCards {
			// 	options = append(options, huh.NewOption(card.Name, card.Name))
			// }

			// var (
			// 	agent    string
			// 	prompt   string
			// 	selected *a2a.AgentCard
			// )

			// form := huh.NewForm(
			// 	huh.NewGroup(
			// 		huh.NewSelect[string]().
			// 			Title("Choose your agent").
			// 			Options(options...).
			// 			Value(&agent),
			// 	),

			// 	// Gather some final details about the order.
			// 	huh.NewGroup(
			// 		huh.NewInput().
			// 			Title("Prompt").
			// 			Value(&prompt),
			// 	),
			// )

			// if err := form.Run(); err != nil {
			// 	return err
			// }

			// for _, card := range agentCards {
			// 	if card.Name == agent {
			// 		selected = &card
			// 		break
			// 	}
			// }

			// agentClient := a2a.NewClient(selected.URL)

			// response, err := agentClient.SendTask(a2a.TaskSendParams{
			// 	ID:        uuid.New().String(),
			// 	SessionID: uuid.New().String(),
			// 	Message:   *a2a.NewTextMessage("user", prompt),
			// })

			// if err != nil {
			// 	log.Error("client.SendTask failed", "error", err)
			// 	return err
			// }

			// // Check for JSON-RPC level error returned by the server
			// if response.Error != nil {
			// 	log.Error("RPC error from server", "code", response.Error.Code, "message", response.Error.Message, "data", response.Error.Data)
			// 	return fmt.Errorf("server error: %s (code: %d)", response.Error.Message, response.Error.Code)
			// }

			// // If response.Error is nil, then response.Result should contain the actual result.
			// if response.Result == nil {
			// 	log.Error("RPC success, but server returned a nil result")
			// 	return fmt.Errorf("server returned success but with a nil result")
			// }

			// var resultBytes []byte
			// var marshalErr error

			// // Attempt to convert response.Result to []byte for unmarshalling
			// // Original code expected []byte, so try type assertion first.
			// resultBytes, ok := response.Result.([]byte)
			// if !ok {
			// 	// If not already []byte, assume it might be a map[string]interface{} or similar
			// 	// and try to marshal it to JSON bytes.
			// 	log.Warn("response.Result is not []byte; attempting to marshal for unmarshalling", "type", fmt.Sprintf("%T", response.Result))
			// 	resultBytes, marshalErr = json.Marshal(response.Result)
			// 	if marshalErr != nil {
			// 		log.Error("failed to marshal response.Result for unmarshalling", "error", marshalErr, "result_type", fmt.Sprintf("%T", response.Result))
			// 		return fmt.Errorf("cannot process result of type %T: %w", response.Result, marshalErr)
			// 	}
			// }

			// // Convert the response result to a Task
			// task := &a2a.Task{}
			// if err := json.Unmarshal(resultBytes, task); err != nil {
			// 	log.Error("failed to unmarshal task response from resultBytes", "error", err)
			// 	return err
			// }

			// // Print the task history
			// for _, message := range task.History {
			// 	fmt.Println(message.String())
			// }

			// return nil
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
