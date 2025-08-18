package a2a

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// getSystemPromptForAgentName looks up agent.<name>.system as either a string or []string.
// Falls back to agent.<lowercased-name>.system and to a generic default if empty.
func getSystemPromptForAgentName(agentName string) string {
	v := viper.GetViper()

	// Try exact key first
	if msgs := v.GetStringSlice(fmt.Sprintf("agent.%s.system", agentName)); len(msgs) > 0 {
		return strings.Join(msgs, "\n")
	}
	if s := v.GetString(fmt.Sprintf("agent.%s.system", agentName)); strings.TrimSpace(s) != "" {
		return s
	}

	// Try lowercased name
	lower := strings.ToLower(agentName)
	if msgs := v.GetStringSlice(fmt.Sprintf("agent.%s.system", lower)); len(msgs) > 0 {
		return strings.Join(msgs, "\n")
	}
	if s := v.GetString(fmt.Sprintf("agent.%s.system", lower)); strings.TrimSpace(s) != "" {
		return s
	}

	// Fallback generic
	return ""
}
