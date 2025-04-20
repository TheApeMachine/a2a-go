package ai

import (
	"net/http"
)

/*
authInjectingRoundTripper adds custom headers right before the request is
sent.  Needed because RPCClient hides the underlying *http.Request*.
*/
type authInjectingRoundTripper struct {
	base       http.RoundTripper
	injectFunc func(*http.Request)
}

func (rt authInjectingRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if rt.injectFunc != nil {
		rt.injectFunc(r)
	}
	return rt.base.RoundTrip(r)
}
