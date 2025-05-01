package types

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/registry"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

/*
AgentCard conveys the top‑level capabilities and metadata exposed by a remote
agent that supports the A2A protocol.
*/
type AgentCard struct {
	Name               string               `json:"name"`
	Description        *string              `json:"description,omitempty"`
	URL                string               `json:"url"`
	Provider           *AgentProvider       `json:"provider,omitempty"`
	Version            string               `json:"version"`
	DocumentationURL   *string              `json:"documentationUrl,omitempty"`
	Capabilities       AgentCapabilities    `json:"capabilities"`
	Authentication     *AgentAuthentication `json:"authentication,omitempty"`
	DefaultInputModes  []string             `json:"defaultInputModes,omitempty"`
	DefaultOutputModes []string             `json:"defaultOutputModes,omitempty"`
	Skills             []AgentSkill         `json:"skills"`
}

func NewAgentCardFromConfig(key string) *AgentCard {
	v := viper.GetViper()

	return &AgentCard{
		Name: v.GetString(fmt.Sprintf("agent.%s.name", key)),
		URL:  v.GetString(fmt.Sprintf("agent.%s.url", key)),
		Provider: &AgentProvider{
			Organization: v.GetString(fmt.Sprintf("agent.%s.provider.organization", key)),
			URL:          utils.Ptr(v.GetString(fmt.Sprintf("agent.%s.provider.url", key))),
		},
		Version:          v.GetString(fmt.Sprintf("agent.%s.version", key)),
		DocumentationURL: utils.Ptr(v.GetString(fmt.Sprintf("agent.%s.documentation_url", key))),
		Capabilities: AgentCapabilities{
			Streaming:              v.GetBool(fmt.Sprintf("agent.%s.capabilities.streaming", key)),
			PushNotifications:      v.GetBool(fmt.Sprintf("agent.%s.capabilities.push_notifications", key)),
			StateTransitionHistory: v.GetBool(fmt.Sprintf("agent.%s.capabilities.state_transition_history", key)),
		},
		Authentication: &AgentAuthentication{
			Schemes:     v.GetStringSlice(fmt.Sprintf("agent.%s.authentication.schemes", key)),
			Credentials: utils.Ptr(v.GetString(fmt.Sprintf("agent.%s.authentication.credentials", key))),
		},
		Skills: []AgentSkill{NewSkillFromConfig(key)},
	}
}

func (card *AgentCard) Tools() map[string]*registry.ToolDescriptor {
	skillTools := map[string]*registry.ToolDescriptor{}

	for _, skill := range card.Skills {
		switch skill.ID {
		case "development":
			log.Info("skill assigned", "agent", card.Name, "skill", skill.Name)
			tool := ToMCPTool(skill)
			skillTools[tool.Name] = tool.ToToolDescriptor()
		}
	}

	return skillTools
}

type AgentProvider struct {
	Organization string  `json:"organization"`
	URL          *string `json:"url,omitempty"`
}

type AgentCapabilities struct {
	Streaming              bool `json:"streaming,omitempty"`
	PushNotifications      bool `json:"pushNotifications,omitempty"`
	StateTransitionHistory bool `json:"stateTransitionHistory,omitempty"`
}

type AgentAuthentication struct {
	Schemes     []string `json:"schemes"`
	Credentials *string  `json:"credentials,omitempty"`
}

type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Examples    []string `json:"examples,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

func NewSkillFromConfig(key string) AgentSkill {
	v := viper.GetViper()

	return AgentSkill{
		ID:          v.GetString(fmt.Sprintf("skills.%s.id", key)),
		Name:        v.GetString(fmt.Sprintf("skills.%s.name", key)),
		Description: utils.Ptr(v.GetString(fmt.Sprintf("skills.%s.description", key))),
		Tags:        v.GetStringSlice(fmt.Sprintf("skills.%s.tags", key)),
		Examples:    v.GetStringSlice(fmt.Sprintf("skills.%s.examples", key)),
		InputModes:  v.GetStringSlice(fmt.Sprintf("skills.%s.input_modes", key)),
		OutputModes: v.GetStringSlice(fmt.Sprintf("skills.%s.output_modes", key)),
	}
}
