package transfer

import (
	"context"
	"encoding/json"
	"net/http"
)

// AuthMiddleware creates a middleware that validates authentication headers and tokens
func AuthMiddleware(authClient *AuthClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Validate headers
			token, err := ValidateHeaders(r)
			if err != nil {
				writeErrorJSON(w, http.StatusUnauthorized, err.Error())
				return
			}

			// Verify token with login service
			user, err := authClient.VerifyToken(r.Context(), token)
			if err != nil {
				status := http.StatusUnauthorized
				if authClient.loginServiceURL == "" {
					status = http.StatusServiceUnavailable
				}
				writeErrorJSON(w, status, "token verification failed: "+err.Error())
				return
			}

			// Store user in request context
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// PublicEndpoint middleware allows unauthenticated access to specific endpoints
func PublicEndpoint(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// writeErrorJSON writes JSON error response
func writeErrorJSON(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
