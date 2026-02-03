package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/prestonfoshee/bopbridge/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TrackRepository defines the interface for track data access operations.
// All methods accept context.Context for cancellation and timeout propagation.
type TrackRepository interface {
	// Create inserts a new track into the database.
	// Returns ErrDuplicateKey if a track with the same SpotifyTrackID already exists for this user.
	Create(ctx context.Context, track *models.Track) error

	// BulkUpsert inserts or updates multiple tracks in a single transaction.
	// This is optimized for syncing large batches of liked songs from Spotify.
	// It uses ON CONFLICT to update existing tracks or insert new ones.
	BulkUpsert(ctx context.Context, tracks []models.Track) error

	// FindByID retrieves a track by its internal database ID.
	// Returns ErrNotFound if no track exists with the given ID.
	FindByID(ctx context.Context, id uint) (*models.Track, error)

	// FindByUserID retrieves all tracks belonging to a specific user.
	// Returns an empty slice (not an error) if the user has no tracks.
	FindByUserID(ctx context.Context, userID uint) ([]models.Track, error)

	// FindBySpotifyIDs retrieves multiple tracks by their Spotify track IDs for a specific user.
	// Returns only the tracks that exist; missing tracks are not included.
	FindBySpotifyIDs(ctx context.Context, userID uint, spotifyIDs []string) ([]models.Track, error)

	// FindByArtist retrieves all tracks for a user by a specific artist name.
	FindByArtist(ctx context.Context, userID uint, artistName string) ([]models.Track, error)

	// UpdateAudioFeatures updates the audio features for a specific track.
	// Returns ErrNotFound if the track does not exist.
	UpdateAudioFeatures(ctx context.Context, trackID uint, features *models.AudioFeatures) error

	// Delete soft-deletes a track from the database by its ID.
	// Returns ErrNotFound if the track does not exist.
	Delete(ctx context.Context, id uint) error

	// CountByUserID returns the total number of tracks for a user.
	CountByUserID(ctx context.Context, userID uint) (int64, error)

	// GetUniqueArtists returns a list of unique artist names for a user's tracks.
	GetUniqueArtists(ctx context.Context, userID uint) ([]string, error)
}

// trackRepository is the concrete implementation of TrackRepository using GORM.
type trackRepository struct {
	db *gorm.DB
}

// NewTrackRepository creates a new TrackRepository with the given database connection.
// The returned interface hides the concrete implementation details.
func NewTrackRepository(db *gorm.DB) TrackRepository {
	return &trackRepository{db: db}
}

// Create inserts a new track into the database.
func (r *trackRepository) Create(ctx context.Context, track *models.Track) error {
	if track == nil {
		return ErrInvalidInput
	}

	if err := r.db.WithContext(ctx).Create(track).Error; err != nil {
		if isDuplicateKeyError(err) {
			return ErrDuplicateKey
		}
		return fmt.Errorf("creating track: %w", err)
	}
	return nil
}

// BulkUpsert inserts or updates multiple tracks in a single transaction.
func (r *trackRepository) BulkUpsert(ctx context.Context, tracks []models.Track) error {
	if len(tracks) == 0 {
		return nil // Nothing to do
	}

	// Use GORM's Clauses to handle ON CONFLICT (upsert)
	// This will update all fields if a conflict occurs on user_id + spotify_track_id
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "spotify_track_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"name",
			"artist_name",
			"album_name",
			"genres",
			"audio_features",
			"is_liked",
			"added_at",
			"updated_at",
		}),
	}).Create(&tracks)

	if result.Error != nil {
		return fmt.Errorf("bulk upserting tracks: %w", result.Error)
	}

	return nil
}

// FindByID retrieves a track by its internal database ID.
func (r *trackRepository) FindByID(ctx context.Context, id uint) (*models.Track, error) {
	var track models.Track
	if err := r.db.WithContext(ctx).First(&track, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("finding track by ID: %w", err)
	}
	return &track, nil
}

// FindByUserID retrieves all tracks belonging to a specific user.
func (r *trackRepository) FindByUserID(ctx context.Context, userID uint) ([]models.Track, error) {
	var tracks []models.Track
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("added_at DESC").
		Find(&tracks).Error; err != nil {
		return nil, fmt.Errorf("finding tracks by user ID: %w", err)
	}
	return tracks, nil
}

// FindBySpotifyIDs retrieves multiple tracks by their Spotify track IDs for a specific user.
func (r *trackRepository) FindBySpotifyIDs(ctx context.Context, userID uint, spotifyIDs []string) ([]models.Track, error) {
	if len(spotifyIDs) == 0 {
		return []models.Track{}, nil
	}

	var tracks []models.Track
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND spotify_track_id IN ?", userID, spotifyIDs).
		Find(&tracks).Error; err != nil {
		return nil, fmt.Errorf("finding tracks by Spotify IDs: %w", err)
	}
	return tracks, nil
}

// FindByArtist retrieves all tracks for a user by a specific artist name.
func (r *trackRepository) FindByArtist(ctx context.Context, userID uint, artistName string) ([]models.Track, error) {
	var tracks []models.Track
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND artist_name = ?", userID, artistName).
		Order("added_at DESC").
		Find(&tracks).Error; err != nil {
		return nil, fmt.Errorf("finding tracks by artist: %w", err)
	}
	return tracks, nil
}

// UpdateAudioFeatures updates the audio features for a specific track.
func (r *trackRepository) UpdateAudioFeatures(ctx context.Context, trackID uint, features *models.AudioFeatures) error {
	result := r.db.WithContext(ctx).
		Model(&models.Track{}).
		Where("id = ?", trackID).
		Update("audio_features", features)

	if result.Error != nil {
		return fmt.Errorf("updating track audio features: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// Delete soft-deletes a track from the database by its ID.
func (r *trackRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&models.Track{}, id)
	if result.Error != nil {
		return fmt.Errorf("deleting track: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// CountByUserID returns the total number of tracks for a user.
func (r *trackRepository) CountByUserID(ctx context.Context, userID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.Track{}).
		Where("user_id = ?", userID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("counting tracks by user ID: %w", err)
	}
	return count, nil
}

// GetUniqueArtists returns a list of unique artist names for a user's tracks.
func (r *trackRepository) GetUniqueArtists(ctx context.Context, userID uint) ([]string, error) {
	var artists []string
	if err := r.db.WithContext(ctx).
		Model(&models.Track{}).
		Where("user_id = ?", userID).
		Distinct("artist_name").
		Order("artist_name ASC").
		Pluck("artist_name", &artists).Error; err != nil {
		return nil, fmt.Errorf("getting unique artists: %w", err)
	}
	return artists, nil
}
