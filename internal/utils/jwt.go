package utils

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims represents the custom claims embedded in the JWT token.
type JWTClaims struct {
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

// GenerateToken generates a new JWT token for a user.
// The token includes the user ID in the claims and expires after the specified duration.
func GenerateToken(userID uint, secret string, expiry time.Duration) (string, error) {
	// Create claims with user ID and standard registered claims
	claims := JWTClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token with secret
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("signing token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the user ID if valid.
// Returns an error if the token is invalid, expired, or malformed.
func ValidateToken(tokenString, secret string) (uint, error) {
	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return 0, fmt.Errorf("parsing token: %w", err)
	}

	// Extract claims
	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return 0, fmt.Errorf("invalid token claims")
	}

	return claims.UserID, nil
}

// ExtractUserID extracts the user ID from a JWT token without full validation.
// This should only be used for logging/debugging purposes.
// For authentication, always use ValidateToken.
func ExtractUserID(tokenString string) (uint, error) {
	token, _, err := jwt.NewParser().ParseUnverified(tokenString, &JWTClaims{})
	if err != nil {
		return 0, fmt.Errorf("parsing token: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return 0, fmt.Errorf("invalid token claims")
	}

	return claims.UserID, nil
}
