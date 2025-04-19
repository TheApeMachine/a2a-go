package a2a

import (
    "encoding/json"
    "reflect"
    "strings"
    "testing"
)

func TestAgentCardJSONRoundTrip(t *testing.T) {
    desc := "test agent"
    docURL := "https://example.com/docs"
    provURL := "https://example.com"
    credential := "token123"

    card := AgentCard{
        Name:        "TestAgent",
        Description: &desc,
        URL:         "https://agent.example.com",
        Provider: &AgentProvider{
            Organization: "Example Org",
            URL:          &provURL,
        },
        Version:          "1.2.3",
        DocumentationURL: &docURL,
        Capabilities: AgentCapabilities{
            Streaming:         true,
            PushNotifications: true,
        },
        Authentication: &AgentAuthentication{
            Schemes:     []string{"Bearer"},
            Credentials: &credential,
        },
        DefaultInputModes:  []string{"text/plain"},
        DefaultOutputModes: []string{"application/json"},
        Skills: []AgentSkill{{
            ID:   "skill-1",
            Name: "Echo",
            Tags: []string{"test", "echo"},
        }},
    }

    data, err := json.Marshal(card)
    if err != nil {
        t.Fatalf("marshal failed: %v", err)
    }

    // Unmarshal back and compare
    var card2 AgentCard
    if err := json.Unmarshal(data, &card2); err != nil {
        t.Fatalf("unmarshal failed: %v", err)
    }

    if !reflect.DeepEqual(card, card2) {
        t.Fatalf("round‑trip mismatch\nwant: %+v\n got: %+v", card, card2)
    }
}

func TestPartMarshalling(t *testing.T) {
    textPart := Part{Type: PartTypeText, Text: "hello"}
    b, err := json.Marshal(textPart)
    if err != nil {
        t.Fatalf("marshal textPart: %v", err)
    }

    // Should contain "\"text\"" type discriminator and "text":"hello"
    if !containsJSONField(b, "type", "text") || !containsJSONField(b, "text", "hello") {
        t.Fatalf("incorrect json: %s", string(b))
    }

    // File part
    uri := "https://example.com/file.txt"
    filePart := Part{
        Type: PartTypeFile,
        File: &FilePart{URI: uri},
    }
    b, err = json.Marshal(filePart)
    if err != nil {
        t.Fatalf("marshal filePart: %v", err)
    }
    if !containsJSONField(b, "type", "file") || !containsJSONField(b, "uri", uri) {
        t.Fatalf("incorrect json: %s", string(b))
    }
}

// helper: crude check if "\"name\":value" exists in json bytes
func containsJSONField(b []byte, name, value string) bool {
    needle := "\"" + name + "\":"
    if value != "" {
        needle += "\"" + value + "\""
    }
    return jsonContains(b, needle)
}

func jsonContains(b []byte, substr string) bool {
    return strings.Contains(string(b), substr)
}
