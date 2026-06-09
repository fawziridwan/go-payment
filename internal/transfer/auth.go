package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AuthenticatedUser represents the authenticated user context
type AuthenticatedUser struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// LoginServiceResponse represents the response from Node.js login service
type LoginServiceResponse struct {
	Success    bool `json:"success"`
	StatusCode int  `json:"statusCode"`
	Data       struct {
		User  AuthenticatedUser `json:"user"`
		Token string            `json:"token"`
	} `json:"data"`
	Message string `json:"message"`
}

// TokenCacheEntry represents a cached token with metadata
type TokenCacheEntry struct {
	User      AuthenticatedUser
	ExpiresAt time.Time
}

// AuthClient handles authentication with Node.js login service
type AuthClient struct {
	loginServiceURL string
	httpClient      *http.Client
	tokenCache      *sync.Map
	cacheTTL        time.Duration
}

// NewAuthClient creates a new authentication client
func NewAuthClient(loginServiceURL string, cacheTTL time.Duration) *AuthClient {
	if cacheTTL == 0 {
		cacheTTL = 1 * time.Hour
	}

	return &AuthClient{
		loginServiceURL: loginServiceURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		tokenCache: &sync.Map{},
		cacheTTL:   cacheTTL,
	}
}

// VerifyToken verifies the token with the login service or cache
func (ac *AuthClient) VerifyToken(ctx context.Context, token string) (*AuthenticatedUser, error) {
	if token == "" {
		return nil, errors.New("token is required")
	}

	tokenHash := hashToken(token)

	// Check cache first
	if cachedEntry, ok := ac.tokenCache.Load(tokenHash); ok {
		entry := cachedEntry.(TokenCacheEntry)
		if time.Now().Before(entry.ExpiresAt) {
			return &entry.User, nil
		}
		// Expired, remove from cache
		ac.tokenCache.Delete(tokenHash)
	}

	// Token not in cache or expired, verify with login service
	user, err := ac.verifyWithLoginService(ctx, token)
	if err != nil {
		return nil, err
	}

	// Cache the token
	ac.tokenCache.Store(tokenHash, TokenCacheEntry{
		User:      *user,
		ExpiresAt: time.Now().Add(ac.cacheTTL),
	})

	return user, nil
}

// verifyWithLoginService calls the Node.js login service to verify the token
func (ac *AuthClient) verifyWithLoginService(ctx context.Context, token string) (*AuthenticatedUser, error) {
	if ac.loginServiceURL == "" {
		return nil, errors.New("login service URL not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ac.loginServiceURL+"/api/v1/verify-token", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ac.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to verify token with login service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login service returned status %d: %s", resp.StatusCode, string(body))
	}

	var loginResp LoginServiceResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return nil, fmt.Errorf("failed to parse login service response: %w", err)
	}

	if !loginResp.Success {
		return nil, errors.New("token verification failed")
	}

	return &loginResp.Data.User, nil
}

// ClearCache clears the token cache
func (ac *AuthClient) ClearCache() {
	ac.tokenCache.Range(func(key, value interface{}) bool {
		ac.tokenCache.Delete(key)
		return true
	})
}

// hashToken creates a hash of the token for cache keys
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// ContextKey for storing user in request context
type ContextKey string

const UserContextKey ContextKey = "authenticated_user"

// GetUserFromContext retrieves the authenticated user from request context
func GetUserFromContext(ctx context.Context) (*AuthenticatedUser, error) {
	user, ok := ctx.Value(UserContextKey).(*AuthenticatedUser)
	if !ok {
		return nil, errors.New("user not found in context")
	}
	return user, nil
}

// ValidateHeaders checks for required authentication headers
func ValidateHeaders(r *http.Request) (token string, err error) {
	// Check service-caller-name header
	callerName := r.Header.Get("service-caller-name")
	if callerName != "supernova-in" {
		return "", fmt.Errorf("invalid or missing service-caller-name header")
	}

	// Extract token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("authorization header is required")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", errors.New("invalid authorization header format")
	}

	token = parts[1]
	if token == "" {
		return "", errors.New("token is required")
	}

	return token, nil
}
