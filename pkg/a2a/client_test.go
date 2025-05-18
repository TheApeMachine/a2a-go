package a2a

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSendTaskStreaming(t *testing.T) {
	Convey("Given an RPC server that streams JSON lines", t, func() {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			// send two events then close
			enc := json.NewEncoder(w)
			_ = enc.Encode(SendTaskStreamingResponse{Result: map[string]any{"step": 1}})
			_ = enc.Encode(SendTaskStreamingResponse{Result: map[string]any{"step": 2}})
		}))
		defer srv.Close()

		client := NewClient(srv.URL)
		ch := make(chan any, 2)
		err := client.SendTaskStreaming(TaskSendParams{}, ch)
		So(err, ShouldBeNil)
		So(<-ch, ShouldResemble, map[string]any{"step": float64(1)})
		So(<-ch, ShouldResemble, map[string]any{"step": float64(2)})
	})
}
