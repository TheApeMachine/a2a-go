package examples

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/ai"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
	"github.com/theapemachine/a2a-go/pkg/service"
	"github.com/theapemachine/a2a-go/pkg/types"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

type DeveloperExample struct{}

func NewDeveloperExample() *DeveloperExample {
	return &DeveloperExample{}
}

func (example *DeveloperExample) Run(interactive bool) error {
	var (
		v   = viper.GetViper()
		err error
	)

	devSkill := types.AgentSkill{
		ID:   v.GetString("skills.development.id"),
		Name: v.GetString("skills.development.name"),
		Description: utils.Ptr(
			v.GetString("skills.development.description"),
		),
		Examples:    v.GetStringSlice("skills.development.examples"),
		InputModes:  v.GetStringSlice("skills.development.input_modes"),
		OutputModes: v.GetStringSlice("skills.development.output_modes"),
	}

	agent := ai.NewAgentFromCard(
		&types.AgentCard{
			Name:    "developer",
			Version: "0.0.1",
			Description: utils.Ptr(
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

	var (
		prompt string
		task   types.Task
	)

	if interactive {
		huh.NewInput().
			Title("Prompt?").
			Value(&prompt).
			Run()
	} else {
		prompt = "Develop an echo server in Go, and run it to show it works."
	}

	task = types.Task{
		ID:        uuid.NewString(),
		SessionID: uuid.NewString(),
		History: []types.Message{
			{
				Role: "system",
				Parts: []types.Part{
					{
						Type: types.PartTypeText,
						Text: v.GetString("agent.developer.system"),
					},
				},
			},
			{
				Role: "user",
				Parts: []types.Part{
					{
						Type: types.PartTypeText,
						Text: prompt,
					},
				},
			},
		},
	}

	return spinner.New().Action(func() {
		for {
			if err = client.Call(
				context.Background(), "tasks/send", task, &task,
			); err != nil {
				log.Error("failed to send task", "error", err)
			}

			fmt.Println(task.String())

			for _, message := range task.History {
				if message.Role == "assistant" {
					for _, part := range message.Parts {
						if strings.Contains(strings.ToLower(part.Text), "task complete") {
							return
						}
					}
				}
			}

			for _, artifact := range task.Artifacts {
				for _, part := range artifact.Parts {
					if strings.Contains(strings.ToLower(part.Text), "task complete") {
						return
					}
				}
			}
		}
	}).Run()
}
