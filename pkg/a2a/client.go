package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/charmbracelet/log"
	"github.com/gofiber/fiber/v3"
	fiberClient "github.com/gofiber/fiber/v3/client"
	"github.com/theapemachine/a2a-go/pkg/jsonrpc"
)

/*
Client represents an A2A protocol client.
*/
type Client struct {
	baseURL string
	conn    *fiberClient.Client
}

/*
NewClient creates a new A2A client.
*/
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		conn:    fiberClient.New().SetBaseURL(baseURL),
	}
}

/*
doRequest is a helper method to send a JSON-RPC request and return a jsonrpc.Response.
*/
func (client *Client) doRequest(req jsonrpc.Request) (jsonrpc.Response, error) {
	res, err := client.conn.Post(
		"/rpc",
		fiberClient.Config{
			Header: map[string]string{
				"Content-Type": "application/json",
			},
			Body: req,
		},
	)

	if err != nil {
		return jsonrpc.Response{}, err
	}

	fm := fiber.Map{}
	res.JSON(&fm)

	// Parse error if present
	var jsonErr *jsonrpc.Error
	if errMap, ok := fm["error"].(map[string]interface{}); ok {
		jsonErr = &jsonrpc.Error{
			Code:    int(errMap["code"].(float64)),
			Message: errMap["message"].(string),
		}
	}

	jsonResp := jsonrpc.Response{
		Message: jsonrpc.Message{
			JSONRPC: "2.0",
		},
		Result: fm["result"],
		Error:  jsonErr,
	}

	return jsonResp, nil
}

/*
SendTask sends a task message to the agent.
*/
func (client *Client) SendTask(params TaskSendParams) (jsonrpc.Response, error) {
	buf, err := json.Marshal(params)

	if err != nil {
		log.Error("failed to marshal task send params", "error", err)
		return jsonrpc.Response{}, err
	}

	req := jsonrpc.Request{
		Message: jsonrpc.Message{
			JSONRPC: "2.0",
		},
		Method: "tasks/send",
		Params: buf,
	}

	return client.doRequest(req)
}

/*
GetTask retrieves the status of a task.
*/
func (client *Client) GetTask(params TaskQueryParams) (jsonrpc.Response, error) {
	req := jsonrpc.Request{
		Message: jsonrpc.Message{
			JSONRPC: "2.0",
		},
		Method: "tasks/get",
		Params: params,
	}

	return client.doRequest(req)
}

/*
CancelTask cancels a task.
*/
func (client *Client) CancelTask(params TaskIDParams) (jsonrpc.Response, error) {
	req := jsonrpc.Request{
		Message: jsonrpc.Message{
			JSONRPC: "2.0",
		},
		Method: "tasks/cancel",
		Params: params,
	}

	return client.doRequest(req)
}

/*
SendTaskStreaming sends a task message and streams the response.
*/
func (client *Client) SendTaskStreaming(
	params TaskSendParams, eventChan chan<- interface{},
) error {
	req := jsonrpc.Request{
		Message: jsonrpc.Message{
			JSONRPC: "2.0",
		},
		Method: "tasks/send",
		Params: params,
	}

	res, err := client.conn.Post(
		"/rpc",
		fiberClient.Config{
			Header: map[string]string{
				"Content-Type": "application/json",
				"Accept":       "text/event-stream",
			},
			Body: req,
		},
	)

	if err != nil {
		return err
	}

	body := res.Body()
	reader := bytes.NewReader(body)
	decoder := json.NewDecoder(reader)
	ctx := context.Background()

	for {
		var event SendTaskStreamingResponse
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode event: %w", err)
		}

		if event.Error != nil {
			return fmt.Errorf("A2A error: %s (code: %d)", event.Error.Message, event.Error.Code)
		}

		select {
		case eventChan <- event.Result:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}
