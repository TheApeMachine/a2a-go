package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"encoding/base64"

	"github.com/tj/assert"
)

func TestPushNotificationSenderAuth_JWKS(t *testing.T) {
	auth, err := NewPushNotificationSenderAuth()
	assert.NoError(t, err)
	assert.NotNil(t, auth)

	// Test JWKS endpoint
	req := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	rec := httptest.NewRecorder()
	auth.JWKSHandler().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	// Verify JWKS response structure
	var jwks jwkSet
	err = json.NewDecoder(rec.Body).Decode(&jwks)
	assert.NoError(t, err)
	assert.Len(t, jwks.Keys, 1)
	assert.Equal(t, "RSA", jwks.Keys[0].Kty)
	assert.Equal(t, "RS256", jwks.Keys[0].Alg)
	assert.Equal(t, "sig", jwks.Keys[0].Use)
	assert.NotEmpty(t, jwks.Keys[0].Kid)
	assert.NotEmpty(t, jwks.Keys[0].N)
	assert.NotEmpty(t, jwks.Keys[0].E)
}

func TestPushNotificationSenderAuth_Send(t *testing.T) {
	auth, err := NewPushNotificationSenderAuth()
	assert.NoError(t, err)

	// Create test server to receive notifications
	var receivedToken string
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get authorization header
		auth := r.Header.Get("Authorization")
		receivedToken = strings.TrimPrefix(auth, "Bearer ")

		// Verify content type
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Read and store raw body
		bodyBytes, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		receivedBody = bodyBytes

		// Compare with expected
		expectedBody := `{"message":"test notification"}`
		assert.JSONEq(t, expectedBody, string(receivedBody))

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Test sending notification
	data := map[string]string{"message": "test notification"}
	err = auth.Send(context.Background(), server.URL, data)
	assert.NoError(t, err)

	// Verify JWT format
	parts := strings.Split(receivedToken, ".")
	assert.Len(t, parts, 3, "JWT should have three parts")

	// Decode JWT header
	headerJSON, err := base64URLDecode(parts[0])
	assert.NoError(t, err)
	var header map[string]interface{}
	err = json.Unmarshal(headerJSON, &header)
	assert.NoError(t, err)
	assert.Equal(t, "RS256", header["alg"])
	assert.Equal(t, "JWT", header["typ"])
	assert.NotEmpty(t, header["kid"])

	// Decode JWT claims
	claimsJSON, err := base64URLDecode(parts[1])
	assert.NoError(t, err)
	var claims map[string]interface{}
	err = json.Unmarshal(claimsJSON, &claims)
	assert.NoError(t, err)
	assert.Equal(t, "a2a‑go", claims["iss"])
	assert.NotEmpty(t, claims["iat"])
	assert.NotEmpty(t, claims["exp"])
}

func TestPushNotificationSenderAuth_VerifyURL(t *testing.T) {
	auth, err := NewPushNotificationSenderAuth()
	assert.NoError(t, err)

	// Test server that returns 200
	validServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodHead, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer validServer.Close()

	// Test server that returns 404
	invalidServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer invalidServer.Close()

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "valid url",
			url:  validServer.URL,
			want: true,
		},
		{
			name: "invalid url",
			url:  invalidServer.URL,
			want: false,
		},
		{
			name: "non-existent url",
			url:  "http://non-existent-url.local",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := auth.VerifyURL(context.Background(), tt.url)
			assert.Equal(t, tt.want, result)
		})
	}
}

// Helper function to decode base64url-encoded strings
func base64URLDecode(s string) ([]byte, error) {
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.StdEncoding.DecodeString(s)
}
