package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/prestonfoshee/bopbridge/internal/models"
	"gorm.io/gorm"
)

// PlaylistRepository defines the interface for playlist data access operations.
// All methods accept context.Context for cancellation and timeout propagation.
type PlaylistRepository interface {
	// Create inserts a new playlist into the database.
	// The playlist's ID will be populated after successful creation.
	Create(ctx context.Context, playlist *models.Playlist) error

	// FindByID retrieves a playlist by its internal database ID.
	// Returns ErrNotFound if no playlist exists with the given ID.
	FindByID(ctx context.Context, id uint) (*models.Playlist, error)

	// FindByUserID retrieves all playlists belonging to a specific user.
	// Returns an empty slice (not an error) if the user has no playlists.
	FindByUserID(ctx context.Context, userID uint) ([]models.Playlist, error)

	// FindBySpotifyID retrieves a playlist by its Spotify playlist ID.
	// Returns ErrNotFound if no playlist exists with the given Spotify ID.
	FindBySpotifyID(ctx context.Context, spotifyPlaylistID string) (*models.Playlist, error)

	// Update saves all changes to an existing playlist record.
	// Returns ErrNotFound if the playlist does not exist.
	Update(ctx context.Context, playlist *models.Playlist) error

	// MarkSynced updates a playlist to indicate it has been synced to Spotify.
	// Sets IsSynced to true and stores the Spotify playlist ID.
	MarkSynced(ctx context.Context, id uint, spotifyPlaylistID string) error

	// Delete removes a playlist from the database by its ID.
	// Returns ErrNotFound if the playlist does not exist.
	Delete(ctx context.Context, id uint) error

	// CountByUserID returns the total number of playlists for a user.
	// Useful for implementing limits or displaying stats.
	CountByUserID(ctx context.Context, userID uint) (int64, error)
}

// playlistRepository is the concrete implementation of PlaylistRepository using GORM.
type playlistRepository struct {
	db *gorm.DB
}

// NewPlaylistRepository creates a new PlaylistRepository with the given database connection.
// The returned interface hides the concrete implementation details.
func NewPlaylistRepository(db *gorm.DB) PlaylistRepository {
	return &playlistRepository{db: db}
}

// Create inserts a new playlist into the database.
func (r *playlistRepository) Create(ctx context.Context, playlist *models.Playlist) error {
	if playlist == nil {
		return ErrInvalidInput
	}

	if err := r.db.WithContext(ctx).Create(playlist).Error; err != nil {
		if isDuplicateKeyError(err) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("creating playlist: %w", err)
	}
	return nil
}

// FindByID retrieves a playlist by its internal database ID.
func (r *playlistRepository) FindByID(ctx context.Context, id uint) (*models.Playlist, error) {
	var playlist models.Playlist
	if err := r.db.WithContext(ctx).First(&playlist, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("finding playlist by ID: %w", err)
	}
	return &playlist, nil
}

// FindByUserID retrieves all playlists belonging to a specific user.
func (r *playlistRepository) FindByUserID(ctx context.Context, userID uint) ([]models.Playlist, error) {
	var playlists []models.Playlist
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&playlists).Error; err != nil {
		return nil, fmt.Errorf("finding playlists by user ID: %w", err)
	}
	return playlists, nil
}

// FindBySpotifyID retrieves a playlist by its Spotify playlist ID.
func (r *playlistRepository) FindBySpotifyID(ctx context.Context, spotifyPlaylistID string) (*models.Playlist, error) {
	var playlist models.Playlist
	if err := r.db.WithContext(ctx).Where("spotify_playlist_id = ?", spotifyPlaylistID).First(&playlist).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("finding playlist by Spotify ID: %w", err)
	}
	return &playlist, nil
}

// Update saves all changes to an existing playlist record.
func (r *playlistRepository) Update(ctx context.Context, playlist *models.Playlist) error {
	if playlist == nil {
		return ErrInvalidInput
	}

	result := r.db.WithContext(ctx).Save(playlist)
	if result.Error != nil {
		if isDuplicateKeyError(result.Error) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("updating playlist: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// MarkSynced updates a playlist to indicate it has been synced to Spotify.
func (r *playlistRepository) MarkSynced(ctx context.Context, id uint, spotifyPlaylistID string) error {
	result := r.db.WithContext(ctx).
		Model(&models.Playlist{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"is_synced":           true,
			"spotify_playlist_id": spotifyPlaylistID,
		})

	if result.Error != nil {
		return fmt.Errorf("marking playlist as synced: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// Delete removes a playlist from the database by its ID.
func (r *playlistRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&models.Playlist{}, id)
	if result.Error != nil {
		return fmt.Errorf("deleting playlist: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// CountByUserID returns the total number of playlists for a user.
func (r *playlistRepository) CountByUserID(ctx context.Context, userID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Playlist{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("counting playlists by user ID: %w", err)
	}
	return count, nil
}

