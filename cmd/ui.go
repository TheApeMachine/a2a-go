package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/theapemachine/a2a-go/pkg/client"
	"github.com/theapemachine/a2a-go/pkg/types"
)

var (
	uiCmd = &cobra.Command{
		Use:   "ui",
		Short: "Run an A2A UI",
		Long:  longUI,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create UI agent client
			uiCard := types.AgentCard{
				Name:    "User Interface Agent",
				Version: "0.1.0",
				URL:     "http://ui:3210",
			}
			agentClient := client.NewAgentClient(uiCard)

			// Main input loop
			for {
				fmt.Print("\nEnter a message: ")
				in := bufio.NewReader(os.Stdin)
				message, err := in.ReadString('\n')
				if err != nil {
					return err
				}

				message = strings.TrimSpace(message)
				if message == "exit" {
					break
				}

				// Send message and stream updates
				err = agentClient.StreamTask(message, func(task types.Task) {
					// Print any new artifacts
					for _, artifact := range task.Artifacts {
						for _, part := range artifact.Parts {
							if part.Type == types.PartTypeText {
								fmt.Printf("\nAgent: %s\n", part.Text)
							}
						}
					}
				})

				if err != nil {
					fmt.Printf("Error: %v\n", err)
					continue
				}
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
