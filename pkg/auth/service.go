package auth

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Service handles authentication and token management
type Service struct {
	mu            sync.RWMutex
	tokens        map[string]*TokenInfo
	refreshTokens map[string]string
	rateLimiter   *RateLimiter
	signingKey    []byte
}

// TokenInfo represents a JWT token and its metadata
type TokenInfo struct {
	Token        string
	ExpiresAt    time.Time
	RefreshToken string
	Scheme       string
}

// NewService creates a new authentication service
func NewService() *Service {
	return &Service{
		tokens:        make(map[string]*TokenInfo),
		refreshTokens: make(map[string]string),
		rateLimiter:   NewRateLimiter(100, time.Minute), // 100 requests per minute
		signingKey:    []byte("your-secret-key"),
	}
}

func (s *Service) getSigningKey(token *jwt.Token) (any, error) {
	if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
	}
	return s.signingKey, nil
}

// AuthenticateRequest authenticates an HTTP request
func (s *Service) AuthenticateRequest(req *http.Request) error {
	if !s.rateLimiter.Allow() {
		return fmt.Errorf("rate limit exceeded")
	}

	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		return fmt.Errorf("missing authorization header")
	}

	// Extract token from header
	tokenStr := authHeader
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		tokenStr = authHeader[7:]
	}

	// Validate token
	token, err := jwt.Parse(tokenStr, s.getSigningKey)
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	// Check token expiration
	if !token.Valid {
		return fmt.Errorf("token expired")
	}

	return nil
}

// GenerateToken generates a new JWT token
func (s *Service) GenerateToken(scheme string, claims jwt.MapClaims) (*TokenInfo, error) {
	now := time.Now()
	if _, ok := claims["exp"]; !ok {
		claims["exp"] = now.Add(time.Hour).Unix()
	}
	if _, ok := claims["iat"]; !ok {
		claims["iat"] = now.Unix()
	}
	if _, ok := claims["jti"]; !ok {
		claims["jti"] = uuid.NewString()
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString(s.signingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	// Generate refresh token
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": claims["sub"],
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": now.Unix(),
		"jti": uuid.NewString(),
	})
	refreshTokenStr, err := refreshToken.SignedString(s.signingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	tokenInfo := &TokenInfo{
		Token:        tokenStr,
		ExpiresAt:    time.Now().Add(time.Hour),
		RefreshToken: refreshTokenStr,
		Scheme:       scheme,
	}

	s.mu.Lock()
	s.tokens[tokenStr] = tokenInfo
	s.refreshTokens[refreshTokenStr] = tokenStr
	s.mu.Unlock()

	return tokenInfo, nil
}

// RefreshToken refreshes an expired token using a refresh token
func (s *Service) RefreshToken(refreshToken string) (*TokenInfo, error) {
	s.mu.RLock()
	oldToken, exists := s.refreshTokens[refreshToken]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Parse the old token to get claims
	token, err := jwt.Parse(oldToken, s.getSigningKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse old token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Remove old timing claims so new values are generated
	delete(claims, "exp")
	delete(claims, "iat")
	delete(claims, "jti")

	// Generate new token with updated claims
	newTokenInfo, newErr := s.GenerateToken("Bearer", claims)
	if newErr != nil {
		// It's good practice to wrap errors to provide context
		return nil, fmt.Errorf("failed to generate new token during refresh: %w", newErr)
	}

	// Invalidate the old refresh token to prevent its reuse.
	// This is a crucial step for security (refresh token rotation).
	s.mu.Lock()
	delete(s.refreshTokens, refreshToken) // 'refreshToken' is the one originally passed to this function.
	
	// Optionally, also invalidate the old access token associated with the used refresh token.
	// This ensures the entire old session is cleaned up from the service's perspective.
	// The 'oldToken' variable (fetched on line 126) holds the string of the old access token.
	// if oldToken != "" { 
	// 	delete(s.tokens, oldToken)
	// }
	s.mu.Unlock()

	return newTokenInfo, nil
}

// RevokeToken revokes a token and its associated refresh token
func (s *Service) RevokeToken(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokenInfo, exists := s.tokens[token]
	if !exists {
		return fmt.Errorf("token not found")
	}

	delete(s.tokens, token)
	delete(s.refreshTokens, tokenInfo.RefreshToken)
	return nil
}

// GetTokenInfo retrieves token information
func (s *Service) GetTokenInfo(token string) (*TokenInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tokenInfo, exists := s.tokens[token]
	if !exists {
		return nil, fmt.Errorf("token not found")
	}

	return tokenInfo, nil
}
