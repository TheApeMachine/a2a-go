package a2a

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/viper"
	"github.com/theapemachine/a2a-go/pkg/tools"
	"github.com/theapemachine/a2a-go/pkg/utils"
)

type AgentAuthentication struct {
	// Schemes is a list of supported authentication schemes
	Schemes []string `json:"schemes"`
	// Credentials for authentication. Can be a string (e.g., token) or null if not required initially
	Credentials *string `json:"credentials,omitempty"`
}

// AgentCapabilities describes the capabilities of an agent
type AgentCapabilities struct {
	// Streaming indicates if the agent supports streaming responses
	Streaming bool `json:"streaming,omitempty"`
	// PushNotifications indicates if the agent supports push notification mechanisms
	PushNotifications bool `json:"pushNotifications,omitempty"`
	// StateTransitionHistory indicates if the agent supports providing state transition history
	StateTransitionHistory bool `json:"stateTransitionHistory,omitempty"`
}

// AgentProvider represents the provider or organization behind an agent
type AgentProvider struct {
	// Organization is the name of the organization providing the agent
	Organization string `json:"organization"`
	// URL associated with the agent provider
	URL *string `json:"url,omitempty"`
}

// AgentSkill defines a specific skill or capability offered by an agent
type AgentSkill struct {
	// ID is the unique identifier for the skill
	ID string `json:"id"`
	// Name is the human-readable name of the skill
	Name string `json:"name"`
	// Description is an optional description of the skill
	Description *string `json:"description,omitempty"`
	// Tags is an optional list of tags associated with the skill for categorization
	Tags []string `json:"tags,omitempty"`
	// Examples is an optional list of example inputs or use cases for the skill
	Examples []string `json:"examples,omitempty"`
	// InputModes is an optional list of input modes supported by this skill
	InputModes []string `json:"inputModes,omitempty"`
	// OutputModes is an optional list of output modes supported by this skill
	OutputModes []string `json:"outputModes,omitempty"`
}

func (skill *AgentSkill) ToMCPTool() (*mcp.Tool, error) {
	return tools.Aquire(skill.ID)
}

// AgentCard represents the metadata card for an agent
type AgentCard struct {
	// Name is the name of the agent
	Name string `json:"name"`
	// Description is an optional description of the agent
	Description *string `json:"description,omitempty"`
	// URL is the base URL endpoint for interacting with the agent
	URL string `json:"url"`
	// Provider is information about the provider of the agent
	Provider *AgentProvider `json:"provider,omitempty"`
	// Version is the version identifier for the agent or its API
	Version string `json:"version"`
	// DocumentationURL is an optional URL pointing to the agent's documentation
	DocumentationURL *string `json:"documentationUrl,omitempty"`
	// Capabilities are the capabilities supported by the agent
	Capabilities AgentCapabilities `json:"capabilities"`
	// Authentication details required to interact with the agent
	Authentication *AgentAuthentication `json:"authentication,omitempty"`
	// DefaultInputModes are the default input modes supported by the agent
	DefaultInputModes []string `json:"defaultInputModes,omitempty"`
	// DefaultOutputModes are the default output modes supported by the agent
	DefaultOutputModes []string `json:"defaultOutputModes,omitempty"`
	// Skills is the list of specific skills offered by the agent
	Skills []AgentSkill `json:"skills"`
}

func (card *AgentCard) Tools() []*mcp.Tool {
	tools := make([]*mcp.Tool, len(card.Skills))

	for _, skill := range card.Skills {
		tool, err := skill.ToMCPTool()

		if err != nil {
			log.Error("failed to aquire tool", "error", err)
			continue
		}

		tools = append(tools, tool)
	}

	return tools
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

func (card *AgentCard) String() string {
	var sb strings.Builder

	// Styles
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("212")).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")).
		Bold(true)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("99")).
		Bold(true)

	// Indentation and box-drawing chars
	indent := "   "
	bullet := "│ "

	// Agent Card Header
	sb.WriteString(headerStyle.Render("Agent Card") + "\n")
	sb.WriteString(bullet + labelStyle.Render("Name: ") + valueStyle.Render(card.Name) + "\n")
	if card.Description != nil {
		sb.WriteString(bullet + labelStyle.Render("Description: ") + valueStyle.Render(*card.Description) + "\n")
	}
	sb.WriteString(bullet + labelStyle.Render("URL: ") + valueStyle.Render(card.URL) + "\n")
	sb.WriteString(bullet + labelStyle.Render("Version: ") + valueStyle.Render(card.Version) + "\n")

	// Provider Section
	if card.Provider != nil {
		sb.WriteString("\n" + sectionStyle.Render("Provider") + "\n")
		sb.WriteString(bullet + labelStyle.Render("Organization: ") + valueStyle.Render(card.Provider.Organization) + "\n")
		if card.Provider.URL != nil {
			sb.WriteString(bullet + labelStyle.Render("URL: ") + valueStyle.Render(*card.Provider.URL) + "\n")
		}
	}

	// Capabilities Section
	sb.WriteString("\n" + sectionStyle.Render("Capabilities") + "\n")
	sb.WriteString(bullet + labelStyle.Render("Streaming: ") + valueStyle.Render(fmt.Sprintf("%v", card.Capabilities.Streaming)) + "\n")
	sb.WriteString(bullet + labelStyle.Render("Push Notifications: ") + valueStyle.Render(fmt.Sprintf("%v", card.Capabilities.PushNotifications)) + "\n")
	sb.WriteString(bullet + labelStyle.Render("State Transition History: ") + valueStyle.Render(fmt.Sprintf("%v", card.Capabilities.StateTransitionHistory)) + "\n")

	// Authentication Section
	if card.Authentication != nil {
		sb.WriteString("\n" + sectionStyle.Render("Authentication") + "\n")
		sb.WriteString(bullet + labelStyle.Render("Schemes: ") + valueStyle.Render(strings.Join(card.Authentication.Schemes, ", ")) + "\n")
		if card.Authentication.Credentials != nil {
			sb.WriteString(bullet + labelStyle.Render("Credentials: ") + valueStyle.Render("*****") + "\n")
		}
	}

	// Skills Section
	if len(card.Skills) > 0 {
		sb.WriteString("\n" + sectionStyle.Render("Skills") + "\n")
		for i, skill := range card.Skills {
			sb.WriteString(bullet + labelStyle.Render(fmt.Sprintf("Skill %d", i+1)) + "\n")
			sb.WriteString(bullet + indent + labelStyle.Render("ID: ") + valueStyle.Render(skill.ID) + "\n")
			sb.WriteString(bullet + indent + labelStyle.Render("Name: ") + valueStyle.Render(skill.Name) + "\n")
			if skill.Description != nil {
				sb.WriteString(bullet + indent + labelStyle.Render("Description: ") + valueStyle.Render(*skill.Description) + "\n")
			}
			if len(skill.Tags) > 0 {
				sb.WriteString(bullet + indent + labelStyle.Render("Tags: ") + valueStyle.Render(strings.Join(skill.Tags, ", ")) + "\n")
			}
			if len(skill.InputModes) > 0 {
				sb.WriteString(bullet + indent + labelStyle.Render("Input Modes: ") + valueStyle.Render(strings.Join(skill.InputModes, ", ")) + "\n")
			}
			if len(skill.OutputModes) > 0 {
				sb.WriteString(bullet + indent + labelStyle.Render("Output Modes: ") + valueStyle.Render(strings.Join(skill.OutputModes, ", ")) + "\n")
			}
		}
	}

	return sb.String()
}
