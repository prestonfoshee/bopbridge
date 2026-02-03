package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prestonfoshee/bopbridge/internal/utils"
)

const (
	// AuthorizationHeader is the HTTP header name for authorization
	AuthorizationHeader = "Authorization"
	
	// BearerPrefix is the prefix for Bearer token authentication
	BearerPrefix = "Bearer "
	
	// UserIDKey is the context key for storing user ID
	UserIDKey = "user_id"
)

// AuthMiddleware creates a middleware that validates JWT tokens and extracts user ID.
// It expects the token to be in the Authorization header with Bearer scheme.
// If the token is valid, the user ID is stored in the Gin context under "user_id" key.
func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get Authorization header
		authHeader := c.GetHeader(AuthorizationHeader)
		if authHeader == "" {
			utils.UnauthorizedResponse(c, "Missing authorization header")
			c.Abort()
			return
		}

		// Check if it starts with "Bearer "
		if !strings.HasPrefix(authHeader, BearerPrefix) {
			utils.UnauthorizedResponse(c, "Invalid authorization header format. Expected: Bearer <token>")
			c.Abort()
			return
		}

		// Extract token
		tokenString := strings.TrimPrefix(authHeader, BearerPrefix)
		if tokenString == "" {
			utils.UnauthorizedResponse(c, "Missing token")
			c.Abort()
			return
		}

		// Validate token and extract user ID
		userID, err := utils.ValidateToken(tokenString, jwtSecret)
		if err != nil {
			utils.UnauthorizedResponse(c, "Invalid or expired token")
			c.Abort()
			return
		}

		// Store user ID in context for handlers to use
		c.Set(UserIDKey, userID)

		// Continue to next handler
		c.Next()
	}
}

// GetUserID extracts the user ID from the Gin context.
// This should be called from handlers that are protected by AuthMiddleware.
// Returns the user ID and a boolean indicating if it was found.
func GetUserID(c *gin.Context) (uint, bool) {
	userID, exists := c.Get(UserIDKey)
	if !exists {
		return 0, false
	}

	id, ok := userID.(uint)
	return id, ok
}

// RequireAuth is a convenience function that combines AuthMiddleware with error handling.
// It's used to protect routes that require authentication.
func RequireAuth(jwtSecret string) gin.HandlerFunc {
	return AuthMiddleware(jwtSecret)
}
