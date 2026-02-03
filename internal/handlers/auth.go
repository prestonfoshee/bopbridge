package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prestonfoshee/bopbridge/internal/config"
	"github.com/prestonfoshee/bopbridge/internal/middleware"
	"github.com/prestonfoshee/bopbridge/internal/services"
	"github.com/prestonfoshee/bopbridge/internal/utils"
)

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	spotifyService services.SpotifyService
	jwtConfig      *config.JWTConfig
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(spotifyService services.SpotifyService, jwtConfig *config.JWTConfig) *AuthHandler {
	return &AuthHandler{
		spotifyService: spotifyService,
		jwtConfig:      jwtConfig,
	}
}

// LoginHandler initiates the Spotify OAuth flow.
// GET /api/v1/auth/login
func (h *AuthHandler) LoginHandler(c *gin.Context) {
	// Generate random state for CSRF protection
	state, err := generateRandomState()
	if err != nil {
		utils.InternalErrorResponse(c, fmt.Errorf("generating state: %w", err))
		return
	}

	// Store state in session/cookie for verification in callback
	c.SetCookie("oauth_state", state, 600, "/", "", false, true) // 10 minutes

	// Get Spotify authorization URL
	authURL := h.spotifyService.GetAuthURL(state)

	// Redirect to Spotify authorization page
	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// CallbackHandler handles the OAuth callback from Spotify.
// GET /api/v1/auth/callback
func (h *AuthHandler) CallbackHandler(c *gin.Context) {
	// Verify state parameter to prevent CSRF attacks
	state := c.Query("state")
	storedState, err := c.Cookie("oauth_state")
	if err != nil || state != storedState {
		utils.BadRequestResponse(c, "Invalid state parameter")
		return
	}

	// Clear state cookie
	c.SetCookie("oauth_state", "", -1, "/", "", false, true)

	// Check for error from Spotify
	if errMsg := c.Query("error"); errMsg != "" {
		utils.BadRequestResponse(c, fmt.Sprintf("Spotify authorization failed: %s", errMsg))
		return
	}

	// Get authorization code
	code := c.Query("code")
	if code == "" {
		utils.BadRequestResponse(c, "Missing authorization code")
		return
	}

	// Exchange code for tokens and create/update user
	user, err := h.spotifyService.ExchangeCode(c.Request.Context(), code)
	if err != nil {
		utils.InternalErrorResponse(c, fmt.Errorf("exchanging code: %w", err))
		return
	}

	// Generate JWT token for the user
	expiryDuration := time.Duration(h.jwtConfig.ExpirationTime) * time.Hour
	if h.jwtConfig.ExpirationTime == 0 {
		expiryDuration = 24 * time.Hour // Default to 24 hours
	}

	token, err := utils.GenerateToken(user.ID, h.jwtConfig.Secret, expiryDuration)
	if err != nil {
		utils.InternalErrorResponse(c, fmt.Errorf("generating JWT: %w", err))
		return
	}

	// Return JWT token and user info
	utils.SuccessResponse(c, http.StatusOK, gin.H{
		"token": token,
		"user":  user,
	})
}

// LogoutHandler handles user logout (client-side token deletion).
// POST /api/v1/auth/logout
func (h *AuthHandler) LogoutHandler(c *gin.Context) {
	// Since we're using stateless JWT, logout is handled client-side
	// by deleting the token. We just return a success message.
	utils.SuccessMessageResponse(c, http.StatusOK, "Logged out successfully", nil)
}

// MeHandler returns the current authenticated user's information.
// GET /api/v1/auth/me
func (h *AuthHandler) MeHandler(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User ID not found in context")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, gin.H{
		"user_id": userID,
	})
}

// generateRandomState generates a random state string for OAuth CSRF protection.
func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
