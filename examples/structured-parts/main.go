// structured-parts illustrates how to include rich structured JSON payloads
// in an A2A message by using PartTypeData.
//
//   go run ./examples/structured-parts
package main

import (
    "encoding/json"
    "fmt"

    a2a "github.com/theapemachine/a2a-go"
)

func main() {
    // Build a structured FORM part using the new helper.
    schema := map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "date": map[string]interface{}{"type": "string", "format": "date"},
            "amount": map[string]interface{}{"type": "number"},
            "purpose": map[string]interface{}{"type": "string"},
        },
        "required": []string{"date", "amount", "purpose"},
    }

    formPart := a2a.NewFormPart(schema, nil, "Please fill out the missing fields.")

    status := a2a.NewInputRequiredStatus("", formPart)

    task := a2a.Task{
        ID: "task‑123",
        Status: status,
    }

    b, _ := json.MarshalIndent(task, "", "  ")
    fmt.Println(string(b))
}
