package provider

// import (
// 	"context"
// 	"os"
// 	"time"

// 	"github.com/google/uuid"
// 	"github.com/theapemachine/a2a-go/pkg/sampling"
// 	"github.com/theapemachine/a2a-go/pkg/types"
// )

// // Ensure interface compliance at compile‑time.
// var _ sampling.Manager = (*OpenAISamplingManager)(nil)

// type OpenAISamplingManager struct {
// 	chat *OpenAIProvider
// }

// // NewOpenAISamplingManager returns a manager configured with the default
// // ChatClient.  OPENAI_API_KEY must be present in the environment (the official
// // openai‑go client reads it automatically) otherwise calls will error.
// func NewOpenAISamplingManager(exec ToolExecutor) *OpenAISamplingManager {
// 	oc := NewOpenAIProvider(exec)
// 	// optional: allow override via env var OPENAI_MODEL
// 	if m := os.Getenv("OPENAI_MODEL"); m != "" {
// 		oc.Model = m
// 	}
// 	return &OpenAISamplingManager{chat: oc}
// }

// // CreateMessage executes a blocking completion and wraps the result.
// func (o *OpenAISamplingManager) CreateMessage(ctx context.Context, content string, opts sampling.SamplingOptions) (*sampling.SamplingResult, error) {
// 	msgs := convertSamplingContext(opts.Context)

// 	// Prepend system prompt (if any) as first message with role "user" for now.
// 	if content != "" {
// 		msgs = append([]types.Message{{Role: "user", Parts: []types.Part{{Type: types.PartTypeText, Text: content}}}}, msgs...)
// 	}

// 	start := time.Now()

// 	// Create a task to hold the response
// 	task := &types.Task{History: msgs}

// 	err := o.chat.Complete(ctx, task, nil)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Extract the response from the task's artifacts
// 	var reply string
// 	if len(task.Artifacts) > 0 {
// 		for _, artifact := range task.Artifacts {
// 			for _, part := range artifact.Parts {
// 				if part.Type == types.PartTypeText {
// 					reply += part.Text
// 				}
// 			}
// 		}
// 	}

// 	msg := sampling.Message{
// 		ID:        uuid.NewString(),
// 		Role:      "assistant",
// 		Content:   reply,
// 		CreatedAt: time.Now(),
// 	}
// 	// OpenAI currently does not return token usage via ChatClient wrapper – we
// 	// leave zeros for now.
// 	res := &sampling.SamplingResult{
// 		Message:  msg,
// 		Duration: time.Since(start).Seconds(),
// 	}
// 	return res, nil
// }

// // StreamMessage streams tokens through channel.
// func (o *OpenAISamplingManager) StreamMessage(
// 	ctx context.Context, content string, opts sampling.SamplingOptions,
// ) (<-chan *sampling.SamplingResult, error) {
// 	ch := make(chan *sampling.SamplingResult)

// 	task := &types.Task{History: convertSamplingContext(opts.Context)}

// 	if content != "" {
// 		task.History = append(task.History, types.Message{
// 			Role: "user",
// 			Parts: []types.Part{{
// 				Type: types.PartTypeText,
// 				Text: content,
// 			}},
// 		})
// 	}

// 	go func() {
// 		defer close(ch)

// 		start := time.Now()

// 		err := o.chat.Stream(ctx, task, nil, func(task *types.Task) {
// 			sr := &sampling.SamplingResult{
// 				Message: sampling.Message{
// 					ID:        uuid.NewString(),
// 					Role:      "assistant",
// 					Content:   task.History[len(task.History)-1].Parts[len(task.History[len(task.History)-1].Parts)-1].Text,
// 					CreatedAt: time.Now(),
// 				},
// 				Duration: time.Since(start).Seconds(),
// 			}
// 			ch <- sr
// 		})

// 		if err != nil {
// 			// send an error sentinel? we just close for now.
// 		}
// 	}()

// 	return ch, nil
// }

// func (o *OpenAISamplingManager) GetModelPreferences(
// 	ctx context.Context,
// ) (*sampling.ModelPreferences, error) {
// 	// Not supported – return nil so caller falls back to defaults.
// 	return nil, nil
// }

// func (o *OpenAISamplingManager) UpdateModelPreferences(
// 	ctx context.Context, prefs sampling.ModelPreferences,
// ) error {
// 	// no‑op for now.
// 	return nil
// }

// // convertSamplingContext turns sampling.Context into []a2a.Message.
// func convertSamplingContext(c *sampling.Context) []types.Message {
// 	if c == nil {
// 		return nil
// 	}

// 	out := make([]types.Message, len(c.Messages))

// 	for i, m := range c.Messages {
// 		out[i] = types.Message{Role: m.Role, Parts: []types.Part{{Type: types.PartTypeText, Text: m.Content}}}
// 	}

// 	return out
// }
