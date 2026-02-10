package services

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/prestonfoshee/bopbridge/internal/models"
	"github.com/prestonfoshee/bopbridge/internal/repository"
	"github.com/rs/zerolog/log"
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
// Uses a pipelined approach: fetching and DB inserts happen concurrently for maximum speed.
func (s *trackService) SyncLikedSongs(ctx context.Context, userID uint) (int, error) {
	startTime := time.Now()
	log.Info().Uint("user_id", userID).Msg("Starting pipelined sync of liked songs from Spotify")

	// Create buffered channel for streaming batches from Spotify to DB
	// Buffer of 20 batches allows fetching to stay ahead of DB inserts
	tracksChan := make(chan []models.Track, 20)

	// Track counts atomically
	var totalInserted int64
	var insertErr error
	insertDone := make(chan struct{})

	// Start DB writer goroutine (consumer) - inserts batches as they arrive
	go func() {
		defer close(insertDone)
		batchNum := 0

		for batch := range tracksChan {
			batchNum++
			log.Debug().
				Uint("user_id", userID).
				Int("batch", batchNum).
				Int("batch_size", len(batch)).
				Msg("Inserting batch to database")

			if err := s.trackRepo.BulkUpsert(ctx, batch); err != nil {
				log.Error().
					Err(err).
					Uint("user_id", userID).
					Int("batch", batchNum).
					Msg("Failed to insert batch to database")
				insertErr = fmt.Errorf("inserting batch %d: %w", batchNum, err)
				// Continue draining channel to prevent deadlock
				for range tracksChan {
				}
				return
			}

			atomic.AddInt64(&totalInserted, int64(len(batch)))
			log.Debug().
				Uint("user_id", userID).
				Int("batch", batchNum).
				Int64("total_inserted", atomic.LoadInt64(&totalInserted)).
				Msg("Batch inserted successfully")
		}
	}()

	// Start streaming fetch from Spotify (producer)
	// This will send batches to tracksChan and close it when done
	totalFetched, fetchErr := s.spotifyService.FetchLikedSongsStream(ctx, userID, tracksChan)

	// Wait for DB writer to finish
	<-insertDone

	// Check for errors
	if fetchErr != nil {
		log.Error().Err(fetchErr).Uint("user_id", userID).Msg("Fetch from Spotify failed")
		return int(atomic.LoadInt64(&totalInserted)), fmt.Errorf("fetching liked songs: %w", fetchErr)
	}
	if insertErr != nil {
		log.Error().Err(insertErr).Uint("user_id", userID).Msg("Database insert failed")
		return int(atomic.LoadInt64(&totalInserted)), fmt.Errorf("storing tracks: %w", insertErr)
	}

	duration := time.Since(startTime)
	finalCount := int(atomic.LoadInt64(&totalInserted))

	log.Info().
		Uint("user_id", userID).
		Int("fetched_count", totalFetched).
		Int("inserted_count", finalCount).
		Dur("total_duration", duration).
		Float64("tracks_per_second", float64(finalCount)/duration.Seconds()).
		Msg("Pipelined sync completed successfully")

	return finalCount, nil
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
