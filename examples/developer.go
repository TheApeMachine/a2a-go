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

/*
DeveloperExample is a naive implementation of a developer agent.
It is used to demonstrate the capabilities of combining A2A with MCP.

You need to have a running Docker daemon for this to work, as the agent will
use a Docker container as a tool to have a working environment.
*/
type DeveloperExample struct {
	devSkill types.AgentSkill
	agent    *ai.Agent
	client   *jsonrpc.RPCClient
	task     types.Task
}

/*
NewDeveloperExample creates a new DeveloperExample instance.
*/
func NewDeveloperExample() *DeveloperExample {
	return &DeveloperExample{}
}

/*
Initialize the DeveloperExample instance, by setting up the agent,
and any skills it needs.

Skills are defined in the A2A spec, and are used to describe the capabilities
of the agent, which in turn will map to the tools it can use.

To get a better understanding of how skills work, have a look at
types/card.go, specifically the Tools() method of the AgentCard type.
*/
func (example *DeveloperExample) Initialize(v *viper.Viper) {
	example.devSkill = types.AgentSkill{
		ID:   v.GetString("skills.development.id"),
		Name: v.GetString("skills.development.name"),
		Description: utils.Ptr(
			v.GetString("skills.development.description"),
		),
		Examples:    v.GetStringSlice("skills.development.examples"),
		InputModes:  v.GetStringSlice("skills.development.input_modes"),
		OutputModes: v.GetStringSlice("skills.development.output_modes"),
	}

	example.agent = ai.NewAgentFromCard(
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
				example.devSkill,
			},
		},
	)

	// Use the client to communicate with the agent. We are no longer
	// calling methods on the agent directly, but rather through the
	// client, which follows the A2A protocol.
	example.client = jsonrpc.NewRPCClient("http://localhost:3210/rpc")
}

func (example *DeveloperExample) Run(interactive bool) error {
	var (
		v      = viper.GetViper()
		prompt string
	)

	example.Initialize(v)

	// Start the agent as a service, so it can be used by the client.
	// We use a goroutine here to avoid blocking the main thread in
	// the example, but a more likely scenario would be to start the
	// service using the CLI serve method, in which case you would not
	// do this, as you want it to be blocking. Have a look at the
	// docker-compose.yml at the root of the repository for an example.
	go func() {
		srv := service.NewA2AServer(example.agent)
		srv.Start()
	}()

	prompt = "Develop an echo server in Go, and run it to show it works."

	if interactive {
		huh.NewInput().
			Title("Prompt?").
			Value(&prompt).
			Run()
	}

	example.setTask(prompt)

	example.agent.SetNotifier(func(task *types.Task) {
		fmt.Print(
			task.History[len(task.History)-1].Parts[len(task.History[len(task.History)-1].Parts)-1].Text,
		)
	})

	return spinner.New().Action(func() {
		example.processTask(example.client)
	}).Run()
}

func (example *DeveloperExample) processTask(
	client *jsonrpc.RPCClient,
) {
	var err error

	for {
		if err = client.Call(
			context.Background(), "tasks/sendSubscribe", example.task, &example.task,
		); err != nil {
			log.Error("failed to send task", "error", err)
		}

		fmt.Println(example.task.String())

		if example.isTaskComplete(&example.task) {
			return
		}
	}
}

func (example *DeveloperExample) setTask(prompt string) {
	v := viper.GetViper()

	example.task = types.Task{
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
}

func (example *DeveloperExample) isTaskComplete(task *types.Task) bool {
	return example.checkHistory(task) || example.checkArtifacts(task)
}

func (example *DeveloperExample) checkHistory(task *types.Task) bool {
	for _, message := range task.History {
		if message.Role == "assistant" {
			for _, part := range message.Parts {
				if strings.Contains(strings.ToLower(part.Text), "task complete") {
					return true
				}
			}
		}
	}

	return false
}

func (example *DeveloperExample) checkArtifacts(task *types.Task) bool {
	for _, artifact := range task.Artifacts {
		for _, part := range artifact.Parts {
			if strings.Contains(strings.ToLower(part.Text), "task complete") {
				return true
			}
		}
	}

	return false
}
