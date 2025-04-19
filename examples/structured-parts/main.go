// structured-parts illustrates how to include rich structured JSON payloads
// in an A2A message by using PartTypeData.
//
//	go run ./examples/structured-parts
package main

import (
	"encoding/json"
	"fmt"

	"github.com/theapemachine/a2a-go/pkg/tools"
	"github.com/theapemachine/a2a-go/pkg/types"
)

func main() {
	// Build a structured FORM part using the new helper.
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"date":    map[string]interface{}{"type": "string", "format": "date"},
			"amount":  map[string]interface{}{"type": "number"},
			"purpose": map[string]interface{}{"type": "string"},
		},
		"required": []string{"date", "amount", "purpose"},
	}

	formPart := tools.NewFormPart(schema, nil, "Please fill out the missing fields.")

	status := tools.NewInputRequiredStatus("", formPart)

	task := types.Task{
		ID:     "task‑123",
		Status: status,
	}

	b, _ := json.MarshalIndent(task, "", "  ")
	fmt.Println(string(b))
}
