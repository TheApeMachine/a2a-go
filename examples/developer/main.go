package main

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/ai"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/service"
	"github.com/theapemachine/a2a-go/pkg/types"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

/*
main sets up an example agent that acts as a developer.

The agent has one skill:
- development
*/
func main() {
	var (
		v   = viper.GetViper()
		err error
	)

	devSkill := types.AgentSkill{
		Name: v.GetString("skill.development.name"),
		Description: utils.Ptr(
			v.GetString("skill.development.description"),
		),
		Examples:    v.GetStringSlice("skill.development.examples"),
		InputModes:  v.GetStringSlice("skill.development.input_modes"),
		OutputModes: v.GetStringSlice("skill.development.output_modes"),
	}

	agent := ai.NewAgentFromCard(
		types.AgentCard{
			Name:    "developer",
			Version: "0.0.1",
			Description: utils.Ptr[string](
				"A tool that can execute commands in a Docker container.",
			),
			URL: "http://localhost:3210/agents/docker-exec",
			Provider: &types.AgentProvider{
				Organization: "theapemachine",
				URL:          utils.Ptr("https://github.com/theapemachine"),
			},
			Capabilities: types.AgentCapabilities{
				Streaming:              true,
				PushNotifications:      true,
				StateTransitionHistory: true,
			},
			Skills: []types.AgentSkill{
				devSkill,
			},
		},
	)

	go func() {
		srv := service.NewA2AServer(agent)
		srv.Start()
	}()

	client := jsonrpc.NewRPCClient("http://localhost:3210/rpc")

	task := types.Task{
		ID:        uuid.NewString(),
		SessionID: uuid.NewString(),
		History: []types.Message{
			{
				Role: "user",
				Parts: []types.Part{
					{
						Type: types.PartTypeText,
						Text: "Write an HTTP Echo server in Go. Make sure the code works, and you have tested it.",
					},
				},
			},
		},
	}

	if err = client.Call(
		context.Background(), "tasks/send", task, &task,
	); err != nil {
		log.Fatalf("failed to send task: %v", err)
	}

	log.Printf("task sent: %v", task)
}
