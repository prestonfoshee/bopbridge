package repository

import (
	"errors"
	"strings"
)

// Common repository errors for consistent error handling across layers.
// Services can use errors.Is() to check for these specific error types
// and handle them appropriately (e.g., return 404 for ErrNotFound).
var (
	// ErrNotFound indicates that the requested record does not exist in the database.
	ErrNotFound = errors.New("record not found")

	// ErrDuplicateKey indicates a unique constraint violation (e.g., duplicate email or Spotify ID).
	ErrDuplicateKey = errors.New("duplicate key violation")

	// ErrInvalidInput indicates that the provided input failed validation.
	ErrInvalidInput = errors.New("invalid input")
)

// isDuplicateKeyError checks if the error is a PostgreSQL unique constraint violation.
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// PostgreSQL unique violation error contains "duplicate key" or error code 23505
	errStr := err.Error()
	return strings.Contains(errStr, "duplicate key") || strings.Contains(errStr, "23505")
}
