package cmd

import (
	"os"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
	"github.com/theapemachine/a2a-go/pkg/service"
)

var (
	slackCmd = &cobra.Command{
		Use:   "slack",
		Short: "Run the Slack service",
		Long:  longSlack,
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetReportCaller(true)
			log.SetLevel(log.InfoLevel)

			appToken := os.Getenv("SLACK_APP_TOKEN")
			botToken := os.Getenv("SLACK_BOT_TOKEN")

			if appToken == "" || botToken == "" {
				return cmd.Help()
			}

			return service.NewSlackService(appToken, botToken).Run()
		},
	}
)

func init() {
	rootCmd.AddCommand(slackCmd)
}

var longSlack = `
Serve the Slack integration service.

This service requires SLACK_APP_TOKEN and SLACK_BOT_TOKEN environment variables.

Examples:
  # Serve the Slack service.
  SLACK_APP_TOKEN=xapp-... SLACK_BOT_TOKEN=xoxb-... a2a-go slack
`
