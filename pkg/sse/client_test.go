package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewClient(t *testing.T) {
	Convey("Given a URL", t, func() {
		url := "http://example.com/events"

		Convey("When creating a new client", func() {
			client := NewClient(url)

			Convey("It should initialize correctly", func() {
				So(client.URL, ShouldEqual, url)
				So(client.Headers, ShouldNotBeNil)
				So(client.Metrics, ShouldNotBeNil)
				So(client.reconnectChan, ShouldNotBeNil)
				So(client.stopChan, ShouldNotBeNil)
			})
		})
	})
}

func TestSubscribeWithContext(t *testing.T) {
	Convey("Given an SSE server", t, func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("data: test\n\n"))
			w.(http.Flusher).Flush()
		}))
		defer server.Close()

		client := NewClient(server.URL)

		Convey("When subscribing to events", func() {
			eventCh := make(chan *Event, 1)
			errCh := make(chan error, 1)

			// Create a context with a reasonable timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// Start subscription in a goroutine
			go func() {
				err := client.SubscribeWithContext(ctx, "", func(event *Event) {
					select {
					case eventCh <- event:
					case <-ctx.Done():
					}
				})
				errCh <- err
			}()

			// Wait for either an event or an error
			var receivedEvent *Event
			var err error

			select {
			case receivedEvent = <-eventCh:
				// Got an event, cancel the context to stop the subscription
				cancel()
			case err = <-errCh:
				// Got an error
			case <-ctx.Done():
				err = ctx.Err()
			}

			Convey("It should receive events", func() {
				So(err, ShouldBeNil)
				So(receivedEvent, ShouldNotBeNil)
				So(string(receivedEvent.Data), ShouldEqual, "test")
			})
		})
	})
}

func TestReconnect(t *testing.T) {
	Convey("Given an SSE server that closes connections", t, func() {
		var connCount int
		var mu sync.Mutex
		serverReady := make(chan struct{}, 1)
		serverDone := make(chan struct{})

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			connCount++
			currentConn := connCount
			mu.Unlock()

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)

			// First connection: send one event and close
			if currentConn == 1 {
				w.Write([]byte("data: test1\n\n"))
				w.(http.Flusher).Flush()
				serverReady <- struct{}{}
				return
			}

			// Second connection: send event and keep connection open
			w.Write([]byte("data: test2\n\n"))
			w.(http.Flusher).Flush()
			serverReady <- struct{}{}
			<-serverDone
		}))
		defer server.Close()

		client := NewClient(server.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		Convey("When connection is lost", func() {
			eventCh := make(chan *Event, 2)
			errCh := make(chan error, 1)

			// Start subscription in a goroutine
			go func() {
				err := client.SubscribeWithContext(ctx, "", func(event *Event) {
					select {
					case eventCh <- event:
					case <-ctx.Done():
					}
				})
				errCh <- err
			}()

			// Wait for first event
			var firstEvent *Event
			select {
			case firstEvent = <-eventCh:
				// Got first event
			case <-time.After(2 * time.Second):
				t.Fatal("timeout waiting for first event")
			}

			// Wait for server to be ready after first connection
			<-serverReady

			// Verify first event
			So(firstEvent, ShouldNotBeNil)
			So(string(firstEvent.Data), ShouldEqual, "test1")

			// Wait for second event (from auto-reconnect)
			var secondEvent *Event
			select {
			case secondEvent = <-eventCh:
				// Got second event
			case <-time.After(5 * time.Second):
				t.Fatal("timeout waiting for second event")
			}

			// Wait for server to be ready after second connection
			<-serverReady

			Convey("It should reconnect and continue receiving events", func() {
				mu.Lock()
				finalConnCount := connCount
				mu.Unlock()
				So(finalConnCount, ShouldEqual, 2)
				So(secondEvent, ShouldNotBeNil)
				So(string(secondEvent.Data), ShouldEqual, "test2")
			})

			// Signal server to close
			close(serverDone)
		})
	})
}

func TestClose(t *testing.T) {
	Convey("Given a connected SSE client", t, func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("data: test\n\n"))
		}))
		defer server.Close()

		client := NewClient(server.URL)

		Convey("When closing the client", func() {
			err := client.Close()

			Convey("It should close successfully", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}
