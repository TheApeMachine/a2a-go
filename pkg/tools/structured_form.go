package tools

// Convenience helpers for building and inspecting "form" structured Data
// parts as used by the Python reimbursement‑agent sample.  They do NOT extend
// the public A2A specification – they merely provide ergonomic sugar around
// the existing Part type whose Data field can carry arbitrary JSON objects.

import (
	"encoding/json"

	"github.com/theapemachine/a2a-go/pkg/types"
)

// FormPayload is the conventional structure the samples use inside the Data
// part when an agent requests the client to fill out a form.
// See README in samples for the shape.
type FormPayload struct {
	Type         string         `json:"type"`         // always "form"
	Form         map[string]any `json:"form"`         // JSON schema
	FormData     map[string]any `json:"form_data"`    // initial values
	Instructions string         `json:"instructions"` // optional
}

// NewFormPart returns a Part of type Data with the inner payload conforming to
// the FormPayload convention.
func NewFormPart(schema map[string]any, data map[string]any, instructions string) types.Part {
	if schema == nil {
		schema = map[string]any{}
	}
	if data == nil {
		data = map[string]any{}
	}
	return types.Part{
		Type: types.PartTypeData,
		Data: map[string]any{
			"type":         "form",
			"form":         schema,
			"form_data":    data,
			"instructions": instructions,
		},
	}
}

// IsFormPart inspects p and returns the decoded FormPayload plus true if it
// matches the expected structure.
func IsFormPart(p types.Part) (FormPayload, bool) {
	if p.Type != types.PartTypeData || p.Data == nil {
		return FormPayload{}, false
	}
	t, ok := p.Data["type"].(string)
	if !ok || t != "form" {
		return FormPayload{}, false
	}
	// marshal then unmarshal to map onto struct conveniently
	b, _ := json.Marshal(p.Data)
	var fp FormPayload
	_ = json.Unmarshal(b, &fp)
	return fp, true
}

// NewInputRequiredStatus constructs a TaskStatus with state=input-required and
// a message containing either a plain text prompt or a form part.
func NewInputRequiredStatus(prompt string, form types.Part) types.TaskStatus {
	msg := types.Message{Role: "agent"}
	if form.Type != "" {
		msg.Parts = []types.Part{form}
	} else {
		msg.Parts = []types.Part{{Type: types.PartTypeText, Text: prompt}}
	}
	return types.TaskStatus{State: types.TaskStateInputReq, Message: &msg}
}
