package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/prestonfoshee/bopbridge/internal/models"
	"gorm.io/gorm"
)

// UserRepository defines the interface for user data access operations.
// All methods accept context.Context for cancellation and timeout propagation.
type UserRepository interface {
	// Create inserts a new user into the database.
	// Returns ErrDuplicateKey if a user with the same SpotifyID already exists.
	Create(ctx context.Context, user *models.User) error

	// FindByID retrieves a user by their internal database ID.
	// Returns ErrNotFound if no user exists with the given ID.
	FindByID(ctx context.Context, id uint) (*models.User, error)

	// FindBySpotifyID retrieves a user by their Spotify ID.
	// Returns ErrNotFound if no user exists with the given Spotify ID.
	FindBySpotifyID(ctx context.Context, spotifyID string) (*models.User, error)

	// Update saves all changes to an existing user record.
	// Returns ErrNotFound if the user does not exist.
	Update(ctx context.Context, user *models.User) error

	// UpdateTokens updates only the OAuth tokens and expiry for a user.
	// This is a common operation during token refresh and is optimized
	// to avoid loading/saving the full user record.
	UpdateTokens(ctx context.Context, id uint, accessToken, refreshToken string, expiry time.Time) error

	// Delete removes a user from the database by their ID.
	// Returns ErrNotFound if the user does not exist.
	Delete(ctx context.Context, id uint) error
}

// userRepository is the concrete implementation of UserRepository using GORM.
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new UserRepository with the given database connection.
// The returned interface hides the concrete implementation details.
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// Create inserts a new user into the database.
func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	if user == nil {
		return ErrInvalidInput
	}

	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		// Check for unique constraint violation (PostgreSQL error code)
		if isDuplicateKeyError(err) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("creating user: %w", err)
	}
	return nil
}

// FindByID retrieves a user by their internal database ID.
func (r *userRepository) FindByID(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("finding user by ID: %w", err)
	}
	return &user, nil
}

// FindBySpotifyID retrieves a user by their Spotify ID.
func (r *userRepository) FindBySpotifyID(ctx context.Context, spotifyID string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).Where("spotify_id = ?", spotifyID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("finding user by Spotify ID: %w", err)
	}
	return &user, nil
}

// Update saves all changes to an existing user record.
func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	if user == nil {
		return ErrInvalidInput
	}

	result := r.db.WithContext(ctx).Save(user)
	if result.Error != nil {
		if isDuplicateKeyError(result.Error) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("updating user: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// UpdateTokens updates only the OAuth tokens and expiry for a user.
func (r *userRepository) UpdateTokens(ctx context.Context, id uint, accessToken, refreshToken string, expiry time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"token_expiry":  expiry,
		})

	if result.Error != nil {
		return fmt.Errorf("updating user tokens: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// Delete removes a user from the database by their ID.
func (r *userRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&models.User{}, id)
	if result.Error != nil {
		return fmt.Errorf("deleting user: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

