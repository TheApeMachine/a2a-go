package stores

import (
	"context"

	"github.com/theapemachine/a2a-go/pkg/a2a"
	"github.com/theapemachine/a2a-go/pkg/errors"
)

/*
TaskStore provides a streaming interface for task data using a connection.
It implements io.ReadWriteCloser to ensure compatibility with other streaming components.
*/
type TaskStore interface {
	Get(context.Context, string, int) ([]a2a.Task, *errors.RpcError)
	Subscribe(context.Context, string, chan a2a.Task) *errors.RpcError
	Create(context.Context, *a2a.Task, ...string) *errors.RpcError
	Update(context.Context, *a2a.Task, ...string) *errors.RpcError
	Delete(context.Context, string) *errors.RpcError
	Cancel(context.Context, string) *errors.RpcError
}
