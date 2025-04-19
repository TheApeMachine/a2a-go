package a2a

// Very small pluggable auth helpers.  The goal is to enable demos that want to
// protect their A2AServer with a static API key or a bearer token without
// introducing heavy dependencies.  Real‑world deployments can swap in their
// own http.Handler that speaks OAuth / mTLS etc.

import (
    "net/http"
    "strings"
)

// AuthChecker validates the incoming HTTP request.  Returning false means the
// request is unauthorized.  Implementations should perform any needed logging
// themselves because the middleware only has boolean semantics.
type AuthChecker interface {
    Authorize(r *http.Request) bool
}

// APIKeyAuth checks for header "X-API-Key: <key>".
type APIKeyAuth struct{ Key string }

func (a APIKeyAuth) Authorize(r *http.Request) bool {
    return r.Header.Get("X-API-Key") == a.Key
}

// BearerAuth checks Authorization: Bearer <token> header – static token for
// demo purposes.
type BearerAuth struct{ Token string }

func (b BearerAuth) Authorize(r *http.Request) bool {
    h := r.Header.Get("Authorization")
    if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
        return false
    }
    return strings.TrimSpace(h[7:]) == b.Token
}

// AuthMiddleware wraps h and returns 401 if checker denies the request.
func AuthMiddleware(checker AuthChecker, h http.Handler) http.Handler {
    if checker == nil {
        return h // no auth
    }
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !checker.Authorize(r) {
            w.WriteHeader(http.StatusUnauthorized)
            return
        }
        h.ServeHTTP(w, r)
    })
}
