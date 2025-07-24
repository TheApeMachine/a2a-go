package cmd

import (
	"github.com/spf13/cobra"
	"github.com/theapemachine/a2a-go/pkg/service"
)

var (
	webhookCmd = &cobra.Command{
		Use:   "webhook",
		Short: "Run the webhook server",
		Long:  longWebhook,
		RunE: func(cmd *cobra.Command, args []string) error {
			return service.NewWebhookServer().Start()
		},
	}
)

func init() {
	rootCmd.AddCommand(webhookCmd)
}

var longWebhook = `
Serve the webhook server.

Examples:
  # Serve the webhook server on port 3210.
  a2a-go webhook
` 