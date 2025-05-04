package ai

import (
	"net/http"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestAuthInjectingRoundTripper(t *testing.T) {
	Convey("Given an auth injecting round tripper", t, func() {
		// Create a mock RoundTripper that just returns a fixed response
		mockTransport := &mockRoundTripper{
			response: &http.Response{
				StatusCode: 200,
			},
		}

		// Create an inject function that adds an Authorization header
		injectFunc := func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer test-token")
		}

		// Create the auth injecting round tripper
		authTripper := authInjectingRoundTripper{
			base:       mockTransport,
			injectFunc: injectFunc,
		}

		Convey("When making a request", func() {
			req, _ := http.NewRequest("GET", "http://example.com", nil)
			resp, err := authTripper.RoundTrip(req)

			Convey("Then it should call the base RoundTripper", func() {
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, 200)
			})

			Convey("And it should inject the auth headers", func() {
				So(mockTransport.lastRequest, ShouldNotBeNil)
				So(mockTransport.lastRequest.Header.Get("Authorization"), ShouldEqual, "Bearer test-token")
			})
		})

		Convey("When the inject function is nil", func() {
			authTripper.injectFunc = nil
			req, _ := http.NewRequest("GET", "http://example.com", nil)
			resp, err := authTripper.RoundTrip(req)

			Convey("Then it should still call the base RoundTripper", func() {
				So(err, ShouldBeNil)
				So(resp.StatusCode, ShouldEqual, 200)
			})

			Convey("And it should not modify the request", func() {
				So(mockTransport.lastRequest, ShouldNotBeNil)
				So(mockTransport.lastRequest.Header.Get("Authorization"), ShouldEqual, "")
			})
		})
	})
}

// mockRoundTripper is a mock implementation of http.RoundTripper for testing
type mockRoundTripper struct {
	response    *http.Response
	lastRequest *http.Request
	err         error
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.lastRequest = req // Store the request for later inspection
	return m.response, m.err
}
