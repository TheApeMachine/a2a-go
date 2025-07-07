package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/ai"
	"github.com/theapemachine/a2a-go/pkg/catalog"
	"github.com/theapemachine/a2a-go/pkg/provider"
	"github.com/theapemachine/a2a-go/pkg/service"
	"github.com/theapemachine/a2a-go/pkg/stores/s3"
)

var (
	configFlag string

	agentCmd = &cobra.Command{
		Use:   "agent",
		Short: "Run an A2A agent",
		Long:  longServe,
		RunE: func(cmd *cobra.Command, args []string) error {
			log.SetReportCaller(true)
			log.SetLevel(log.DebugLevel)

			if configFlag == "" {
				return errors.New("config is required")
			}

			v := viper.GetViper()

			skills := make([]a2a.AgentSkill, 0)

			for _, skill := range v.GetStringSlice(
				fmt.Sprintf("agent.%s.skills", configFlag),
			) {
				skills = append(skills, a2a.NewSkillFromConfig(skill))
			}

			minioClient, err := minio.New("minio:9000", &minio.Options{
				Region: "us-east-1",
				Creds: credentials.NewStaticV4(
					os.Getenv("AWS_ACCESS_KEY_ID"),
					os.Getenv("AWS_SECRET_ACCESS_KEY"),
					"",
				),
				Secure: false,
			})

			if err != nil {
				log.Error("failed to create minio client", "error", err)
				return err
			}

			// Ensure the tasks bucket exists
			ctx := context.Background()
			maxRetries := 10
			
			for try := 0; try < maxRetries; try++ {
				exists, err := minioClient.BucketExists(ctx, "tasks")
				if err != nil {
					log.Error("failed to check if tasks bucket exists", "error", err, "attempt", try+1)
					time.Sleep(time.Second * time.Duration(1<<try)) // Exponential backoff: 1s, 2s, 4s, 8s, etc.
					continue
				}

				if exists {
					log.Info("tasks bucket already exists")
					break
				}

				log.Info("creating tasks bucket", "attempt", try+1)
				if err := minioClient.MakeBucket(
					ctx, "tasks", minio.MakeBucketOptions{},
				); err != nil {
					log.Error("failed to create tasks bucket", "error", err, "attempt", try+1)
					if try == maxRetries-1 {
						return fmt.Errorf("failed to create tasks bucket after %d attempts: %w", maxRetries, err)
					}
					time.Sleep(time.Second * time.Duration(1<<try)) // Exponential backoff: 1s, 2s, 4s, 8s, etc.
					continue
				}

				// Successfully created bucket
				log.Info("tasks bucket created successfully")
				break
			}

			card := a2a.NewAgentCardFromConfig(configFlag)
			tm, err := ai.NewTaskManager(
				card,
				ai.WithTaskStore(s3.NewStore(
					s3.NewConn(
						s3.WithClient(minioClient),
					),
				)),
				ai.WithProvider(provider.NewOpenAIProvider(
					provider.WithOpenAIClient(),
				)),
			)

			if err != nil {
				log.Error("failed to create task manager", "error", err)
				return err
			}

			agent, err := ai.NewAgentFromCard(
				card,
				ai.WithCatalogClient(
					catalog.NewCatalogClient(os.Getenv("CATALOG_URL")),
				),
				ai.WithTaskManager(
					tm,
				),
			)

			if err != nil {
				log.Error("failed to create agent", "error", err)
				return err
			}

			return service.NewAgentServer(agent).Start()
		},
	}
)

func init() {
	rootCmd.AddCommand(agentCmd)

	agentCmd.PersistentFlags().StringVarP(&configFlag, "config", "c", "", "Configuration to use")
}

var longServe = `
Serve an A2A agent or MCP server with various configurations.

Examples:
  # Serve an A2A agent with the developer configuration.
  a2a-go agent --config developer
`
