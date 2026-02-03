package services

import (
	"context"
	"fmt"

	"github.com/prestonfoshee/bopbridge/internal/models"
	"github.com/prestonfoshee/bopbridge/internal/repository"
)

// TrackService defines the interface for managing track data.
// It coordinates between the Spotify API and local database storage.
type TrackService interface {
	// SyncLikedSongs fetches all liked songs from Spotify and stores them in the database.
	// This may take some time for users with large libraries.
	SyncLikedSongs(ctx context.Context, userID uint) (int, error)

	// GetUserTracks retrieves all tracks for a user from the database.
	GetUserTracks(ctx context.Context, userID uint) ([]models.Track, error)

	// GetTracksByArtist retrieves all tracks for a specific artist.
	GetTracksByArtist(ctx context.Context, userID uint, artistName string) ([]models.Track, error)

	// GetUniqueArtists returns a list of all unique artist names in a user's library.
	GetUniqueArtists(ctx context.Context, userID uint) ([]string, error)

	// GetTrackCount returns the total number of tracks for a user.
	GetTrackCount(ctx context.Context, userID uint) (int64, error)

	// SyncAudioFeatures fetches and stores audio features for tracks that don't have them yet.
	SyncAudioFeatures(ctx context.Context, userID uint) (int, error)
}

// trackService is the concrete implementation of TrackService.
type trackService struct {
	trackRepo      repository.TrackRepository
	spotifyService SpotifyService
}

// NewTrackService creates a new TrackService with the given repositories and services.
func NewTrackService(trackRepo repository.TrackRepository, spotifyService SpotifyService) TrackService {
	return &trackService{
		trackRepo:      trackRepo,
		spotifyService: spotifyService,
	}
}

// SyncLikedSongs fetches all liked songs from Spotify and stores them in the database.
func (s *trackService) SyncLikedSongs(ctx context.Context, userID uint) (int, error) {
	// Fetch all liked songs from Spotify
	tracks, err := s.spotifyService.FetchAllLikedSongs(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("fetching liked songs from Spotify: %w", err)
	}

	if len(tracks) == 0 {
		return 0, nil
	}

	// Fetch audio features for all tracks
	trackIDs := make([]string, len(tracks))
	for i, track := range tracks {
		trackIDs[i] = track.SpotifyTrackID
	}

	features, err := s.spotifyService.FetchAudioFeatures(ctx, userID, trackIDs)
	if err != nil {
		// Don't fail the entire sync if audio features fail
		// We can fetch them later with SyncAudioFeatures
		fmt.Printf("Warning: failed to fetch audio features: %v\n", err)
	} else {
		// Attach audio features to tracks
		for i := range tracks {
			if feature, ok := features[tracks[i].SpotifyTrackID]; ok {
				tracks[i].AudioFeatures = feature
			}
		}
	}

	// Bulk upsert all tracks into the database
	if err := s.trackRepo.BulkUpsert(ctx, tracks); err != nil {
		return 0, fmt.Errorf("storing tracks in database: %w", err)
	}

	return len(tracks), nil
}

// GetUserTracks retrieves all tracks for a user from the database.
func (s *trackService) GetUserTracks(ctx context.Context, userID uint) ([]models.Track, error) {
	tracks, err := s.trackRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting user tracks: %w", err)
	}
	return tracks, nil
}

// GetTracksByArtist retrieves all tracks for a specific artist.
func (s *trackService) GetTracksByArtist(ctx context.Context, userID uint, artistName string) ([]models.Track, error) {
	tracks, err := s.trackRepo.FindByArtist(ctx, userID, artistName)
	if err != nil {
		return nil, fmt.Errorf("getting tracks by artist: %w", err)
	}
	return tracks, nil
}

// GetUniqueArtists returns a list of all unique artist names in a user's library.
func (s *trackService) GetUniqueArtists(ctx context.Context, userID uint) ([]string, error) {
	artists, err := s.trackRepo.GetUniqueArtists(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting unique artists: %w", err)
	}
	return artists, nil
}

// GetTrackCount returns the total number of tracks for a user.
func (s *trackService) GetTrackCount(ctx context.Context, userID uint) (int64, error) {
	count, err := s.trackRepo.CountByUserID(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("getting track count: %w", err)
	}
	return count, nil
}

// SyncAudioFeatures fetches and stores audio features for tracks that don't have them yet.
func (s *trackService) SyncAudioFeatures(ctx context.Context, userID uint) (int, error) {
	// Get all user tracks
	tracks, err := s.trackRepo.FindByUserID(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("getting user tracks: %w", err)
	}

	// Find tracks without audio features
	var tracksWithoutFeatures []models.Track
	for _, track := range tracks {
		if track.AudioFeatures == nil {
			tracksWithoutFeatures = append(tracksWithoutFeatures, track)
		}
	}

	if len(tracksWithoutFeatures) == 0 {
		return 0, nil
	}

	// Extract track IDs
	trackIDs := make([]string, len(tracksWithoutFeatures))
	for i, track := range tracksWithoutFeatures {
		trackIDs[i] = track.SpotifyTrackID
	}

	// Fetch audio features from Spotify
	features, err := s.spotifyService.FetchAudioFeatures(ctx, userID, trackIDs)
	if err != nil {
		return 0, fmt.Errorf("fetching audio features from Spotify: %w", err)
	}

	// Update tracks with audio features
	updatedCount := 0
	for _, track := range tracksWithoutFeatures {
		if feature, ok := features[track.SpotifyTrackID]; ok {
			if err := s.trackRepo.UpdateAudioFeatures(ctx, track.ID, feature); err != nil {
				fmt.Printf("Warning: failed to update audio features for track %d: %v\n", track.ID, err)
				continue
			}
			updatedCount++
		}
	}

	return updatedCount, nil
}
