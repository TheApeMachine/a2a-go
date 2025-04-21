package jsonrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/theapemachine/a2a-go/pkg/errors"
	"github.com/theapemachine/a2a-go/pkg/types"
)

type RPCServer struct {
	agent types.IdentifiableTaskManager
}

func NewRPCServer(agent types.IdentifiableTaskManager) *RPCServer {
	return &RPCServer{
		agent: agent,
	}
}

func (s *RPCServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST supported", http.StatusMethodNotAllowed)
		return
	}

	var (
		body []byte
		err  error
	)

	if body, err = io.ReadAll(r.Body); err != nil {
		respondError(w, nil, errors.ErrParseError)
		return
	}

	// Support batch requests if the first byte is '['
	body = bytes.TrimSpace(body)

	if len(body) == 0 {
		respondError(w, nil, errors.ErrInvalidRequest)
		return
	}

	if body[0] == '[' {
		var batch []RPCRequest

		if err = json.Unmarshal(body, &batch); err != nil {
			respondError(w, nil, errors.ErrParseError)
			return
		}

		var responses []RPCResponse

		for _, req := range batch {
			resp := s.handle(r.Context(), &req)

			// Notifications have no ID – skip sending a response.
			if len(req.ID) != 0 {
				responses = append(responses, resp)
			}
		}

		if len(responses) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if err = json.NewEncoder(w).Encode(responses); err != nil {
			respondError(w, nil, errors.ErrParseError)
			return
		}

		return
	}

	var req RPCRequest

	if err = json.Unmarshal(body, &req); err != nil {
		respondError(w, nil, errors.ErrParseError)
		return
	}

	resp := s.handle(r.Context(), &req)

	// Notification – no ID → no response.
	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err = json.NewEncoder(w).Encode(resp); err != nil {
		respondError(w, nil, errors.ErrParseError)
		return
	}
}

func (srv *RPCServer) handle(ctx context.Context, req *RPCRequest) RPCResponse {
	if req.JSONRPC != "2.0" {
		return newErrorResponse(req.ID, errors.ErrInvalidRequest)
	}

	switch req.Method {
	case "tasks/send":
		var params types.Task

		if err := json.Unmarshal(req.Params, &params); err != nil {
			return newErrorResponse(req.ID, errors.ErrInvalidParams)
		}

		task, rpcErr := srv.agent.SendTask(ctx, params)

		if rpcErr != nil {
			return newErrorResponse(req.ID, rpcErr)
		}

		return RPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  task,
		}

	case "tasks/get":
		var params types.TaskGetParams

		if err := json.Unmarshal(req.Params, &params); err != nil {
			return newErrorResponse(req.ID, errors.ErrInvalidParams)
		}

		task, rpcErr := srv.agent.GetTask(ctx, params.ID, 10)

		if rpcErr != nil {
			return newErrorResponse(req.ID, rpcErr)
		}

		return RPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  task,
		}
	case "tasks/cancel":
		var params types.TaskCancelParams

		if err := json.Unmarshal(req.Params, &params); err != nil {
			return newErrorResponse(req.ID, errors.ErrInvalidParams)
		}

		task, rpcErr := srv.agent.CancelTask(ctx, params.ID)

		if rpcErr != nil {
			return newErrorResponse(req.ID, rpcErr)
		}

		return RPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  task,
		}

	case "tasks/stream":
		var params types.Task

		if err := json.Unmarshal(req.Params, &params); err != nil {
			return newErrorResponse(req.ID, errors.ErrInvalidParams)
		}

		task, rpcErr := srv.agent.StreamTask(ctx, params)

		if rpcErr != nil {
			return newErrorResponse(req.ID, rpcErr)
		}

		return RPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  task,
		}

	case "tasks/resubscribe":
		var params types.TaskResubscribeParams

		if err := json.Unmarshal(req.Params, &params); err != nil {
			return newErrorResponse(req.ID, errors.ErrInvalidParams)
		}

		task, rpcErr := srv.agent.ResubscribeTask(ctx, params.ID, 10)

		if rpcErr != nil {
			return newErrorResponse(req.ID, rpcErr)
		}

		return RPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  task,
		}

	case "tasks/pushNotification":
		var params types.TaskPushNotificationParams

		if err := json.Unmarshal(req.Params, &params); err != nil {
			return newErrorResponse(req.ID, errors.ErrInvalidParams)
		}

		pushNotification := types.TaskPushNotificationConfig{
			ID: params.ID,
		}

		task, rpcErr := srv.agent.SetPushNotification(ctx, pushNotification)

		if rpcErr != nil {
			return newErrorResponse(req.ID, rpcErr)
		}

		return RPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  task,
		}

	case "tasks/getPushNotification":
		var params types.TaskPushNotificationParams

		if err := json.Unmarshal(req.Params, &params); err != nil {
			return newErrorResponse(req.ID, errors.ErrInvalidParams)
		}

		pushNotification, rpcErr := srv.agent.GetPushNotification(ctx, params.ID)

		if rpcErr != nil {
			return newErrorResponse(req.ID, rpcErr)
		}

		return RPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  pushNotification,
		}
	}

	return newErrorResponse(req.ID, errors.ErrMethodNotFound)
}

func newErrorResponse(id json.RawMessage, e *errors.RpcError) RPCResponse {
	// Ensure mandatory Code/Message.
	if e == nil {
		e = errors.ErrInternal
	}

	return RPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   e,
	}
}

func respondError(w http.ResponseWriter, id json.RawMessage, e *errors.RpcError) {
	if err := json.NewEncoder(w).Encode(newErrorResponse(id, e)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
