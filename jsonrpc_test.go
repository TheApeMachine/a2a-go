package a2a

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestJSONRPCServerClientRoundTrip(t *testing.T) {
    srv := NewRPCServer()

    // Register echo method.
    srv.Register("echo", func(ctx context.Context, params json.RawMessage) (interface{}, *rpcError) {
        var v string
        if err := json.Unmarshal(params, &v); err != nil {
            return nil, errInvalidParams
        }
        return v, nil
    })

    ts, errTS := newTestServer(srv)
    if errTS != nil {
        t.Skip("network disabled in environment; skipping test")
    }
    defer ts.Close()

    client := &RPCClient{Endpoint: ts.URL}

    var out string
    if err := client.Call(context.Background(), "echo", "hello", &out); err != nil {
        t.Fatalf("call failed: %v", err)
    }
    if out != "hello" {
        t.Fatalf("unexpected result: %s", out)
    }

    // Test error path – invalid method
    err := client.Call(context.Background(), "does.not.exist", nil, nil)
    if err == nil {
        t.Fatalf("expected error for unknown method")
    }
}

func TestJSONRPCServerHandlerReturnsError(t *testing.T) {
    srv := NewRPCServer()
    srv.Register("fail", func(ctx context.Context, params json.RawMessage) (interface{}, *rpcError) {
        return nil, &rpcError{Code: 123, Message: "boom"}
    })

    ts, errTS := newTestServer(srv)
    if errTS != nil {
        t.Skip("network disabled in environment; skipping test")
    }
    defer ts.Close()

    client := &RPCClient{Endpoint: ts.URL}
    err := client.Call(context.Background(), "fail", nil, nil)
    if err == nil || err.Error() != "boom" {
        t.Fatalf("expected boom error, got %v", err)
    }
}

// newTestServer wraps httptest.NewServer but converts the panic that is thrown
// when the environment forbids listening on sockets into a regular error so
// the caller can gracefully skip the test.
func newTestServer(h http.Handler) (*httptest.Server, error) {
    var srv *httptest.Server
    var err error
    func() {
        defer func() {
            if r := recover(); r != nil {
                err = fmt.Errorf("listener not permitted: %v", r)
            }
        }()
        srv = httptest.NewServer(h)
    }()
    return srv, err
}
