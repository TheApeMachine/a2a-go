package types

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

func TestPartValidate(t *testing.T) {
	tests := []struct {
		name        string
		part        Part
		expectError bool
	}{
		{
			name: "Valid text part",
			part: Part{
				Type: PartTypeText,
				Text: "Hello",
			},
			expectError: false,
		},
		{
			name: "Invalid text part - empty text",
			part: Part{
				Type: PartTypeText,
				Text: "",
			},
			expectError: true,
		},
		{
			name: "Multiple fields populated",
			part: Part{
				Type: PartTypeText,
				Text: "Hello",
				Data: map[string]any{"key": "value"},
			},
			expectError: true,
		},
		{
			name: "Valid file part",
			part: Part{
				Type: PartTypeFile,
				File: &FilePart{
					URI: "https://example.com",
				},
			},
			expectError: false,
		},
		{
			name: "Invalid file part - nil",
			part: Part{
				Type: PartTypeFile,
				File: nil,
			},
			expectError: true,
		},
		{
			name: "Valid data part",
			part: Part{
				Type: PartTypeData,
				Data: map[string]any{"key": "value"},
			},
			expectError: false,
		},
		{
			name: "Invalid data part - nil",
			part: Part{
				Type: PartTypeData,
				Data: nil,
			},
			expectError: true,
		},
		{
			name: "Invalid part type",
			part: Part{
				Type: "invalid",
				Text: "Hello",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.part.Validate()
			
			if tt.expectError && err == nil {
				t.Errorf("Expected an error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestFilePartValidate(t *testing.T) {
	tests := []struct {
		name        string
		filePart    FilePart
		expectError bool
	}{
		{
			name: "Valid file part with URI",
			filePart: FilePart{
				URI: "https://example.com",
			},
			expectError: false,
		},
		{
			name: "Valid file part with bytes",
			filePart: FilePart{
				Bytes: "base64data",
			},
			expectError: false,
		},
		{
			name: "Invalid file part - both URI and bytes",
			filePart: FilePart{
				URI:   "https://example.com",
				Bytes: "base64data",
			},
			expectError: true,
		},
		{
			name: "Invalid file part - neither URI nor bytes",
			filePart: FilePart{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.filePart.Validate()
			
			if tt.expectError && err == nil {
				t.Errorf("Expected an error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
