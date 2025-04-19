package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tj/assert"
)

func TestAPIKeyAuth(t *testing.T) {
	auth := APIKeyAuth{Key: "test-key"}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := AuthMiddleware(auth, handler)

	tests := []struct {
		name       string
		apiKey     string
		wantStatus int
	}{
		{
			name:       "valid api key",
			apiKey:     "test-key",
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid api key",
			apiKey:     "wrong-key",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "missing api key",
			apiKey:     "",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.apiKey != "" {
				req.Header.Set("X-API-Key", tt.apiKey)
			}

			rec := httptest.NewRecorder()
			middleware.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestBearerAuth(t *testing.T) {
	auth := BearerAuth{Token: "test-token"}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := AuthMiddleware(auth, handler)

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer test-token",
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid bearer token",
			authHeader: "Bearer wrong-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "missing bearer prefix",
			authHeader: "test-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty auth header",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "case insensitive bearer",
			authHeader: "bearer test-token",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rec := httptest.NewRecorder()
			middleware.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestAuthMiddlewareWithNilChecker(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	middleware := AuthMiddleware(nil, handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "should pass through when checker is nil")
}
