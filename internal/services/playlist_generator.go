package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/prestonfoshee/bopbridge/internal/models"
	"github.com/prestonfoshee/bopbridge/internal/repository"
)

// PlaylistGeneratorService defines the interface for generating playlists.
// It orchestrates track analysis and playlist creation.
type PlaylistGeneratorService interface {
	// GenerateByArtist creates a playlist containing all tracks by a specific artist.
	GenerateByArtist(ctx context.Context, userID uint, artistName string, syncToSpotify bool) (*models.Playlist, error)

	// GenerateByGenre creates a playlist containing tracks from a specific genre.
	GenerateByGenre(ctx context.Context, userID uint, genre string, syncToSpotify bool) (*models.Playlist, error)

	// GenerateByMood creates a playlist containing tracks matching a specific mood.
	GenerateByMood(ctx context.Context, userID uint, mood string, trackCount int, syncToSpotify bool) (*models.Playlist, error)

	// GenerateByEnergy creates a playlist with tracks in a specific energy range.
	GenerateByEnergy(ctx context.Context, userID uint, minEnergy, maxEnergy float64, trackCount int, syncToSpotify bool) (*models.Playlist, error)

	// GenerateSimilar creates a playlist of tracks similar to a given track.
	GenerateSimilar(ctx context.Context, userID uint, trackID uint, trackCount int, syncToSpotify bool) (*models.Playlist, error)

	// GenerateTopArtists creates playlists for the user's top N artists.
	GenerateTopArtists(ctx context.Context, userID uint, topN int, syncToSpotify bool) ([]*models.Playlist, error)
}

// playlistGeneratorService is the concrete implementation of PlaylistGeneratorService.
type playlistGeneratorService struct {
	playlistRepo   repository.PlaylistRepository
	trackRepo      repository.TrackRepository
	spotifyService SpotifyService
	analyzer       AnalyzerService
}

// NewPlaylistGeneratorService creates a new PlaylistGeneratorService.
func NewPlaylistGeneratorService(
	playlistRepo repository.PlaylistRepository,
	trackRepo repository.TrackRepository,
	spotifyService SpotifyService,
	analyzer AnalyzerService,
) PlaylistGeneratorService {
	return &playlistGeneratorService{
		playlistRepo:   playlistRepo,
		trackRepo:      trackRepo,
		spotifyService: spotifyService,
		analyzer:       analyzer,
	}
}

// GenerateByArtist creates a playlist containing all tracks by a specific artist.
func (s *playlistGeneratorService) GenerateByArtist(ctx context.Context, userID uint, artistName string, syncToSpotify bool) (*models.Playlist, error) {
	// Get tracks by artist
	tracks, err := s.trackRepo.FindByArtist(ctx, userID, artistName)
	if err != nil {
		return nil, fmt.Errorf("finding tracks by artist: %w", err)
	}

	if len(tracks) == 0 {
		return nil, fmt.Errorf("no tracks found for artist: %s", artistName)
	}

	// Create playlist record
	params, _ := json.Marshal(map[string]interface{}{
		"artist": artistName,
	})

	playlist := &models.Playlist{
		UserID:      userID,
		Name:        fmt.Sprintf("%s - All Tracks", artistName),
		Description: fmt.Sprintf("All your liked songs by %s", artistName),
		Strategy:    models.StrategyGenre,
		Params:      string(params),
		TrackCount:  len(tracks),
		IsPublic:    false,
		IsSynced:    false,
	}

	if err := s.playlistRepo.Create(ctx, playlist); err != nil {
		return nil, fmt.Errorf("creating playlist record: %w", err)
	}

	// Add tracks to playlist
	if err := s.addTracksToPlaylist(ctx, playlist.ID, tracks); err != nil {
		return nil, fmt.Errorf("adding tracks to playlist: %w", err)
	}

	// Sync to Spotify if requested
	if syncToSpotify {
		if err := s.syncPlaylistToSpotify(ctx, userID, playlist, tracks); err != nil {
			return nil, fmt.Errorf("syncing playlist to Spotify: %w", err)
		}
	}

	return playlist, nil
}

// GenerateByGenre creates a playlist containing tracks from a specific genre.
func (s *playlistGeneratorService) GenerateByGenre(ctx context.Context, userID uint, genre string, syncToSpotify bool) (*models.Playlist, error) {
	// Get all user tracks
	allTracks, err := s.trackRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding user tracks: %w", err)
	}

	// Group by genre
	genreGroups := s.analyzer.GroupByGenre(allTracks)
	tracks, ok := genreGroups[genre]
	if !ok || len(tracks) == 0 {
		return nil, fmt.Errorf("no tracks found for genre: %s", genre)
	}

	// Create playlist record
	params, _ := json.Marshal(map[string]interface{}{
		"genre": genre,
	})

	playlist := &models.Playlist{
		UserID:      userID,
		Name:        fmt.Sprintf("%s Playlist", genre),
		Description: fmt.Sprintf("All your %s tracks", genre),
		Strategy:    models.StrategyGenre,
		Params:      string(params),
		TrackCount:  len(tracks),
		IsPublic:    false,
		IsSynced:    false,
	}

	if err := s.playlistRepo.Create(ctx, playlist); err != nil {
		return nil, fmt.Errorf("creating playlist record: %w", err)
	}

	// Add tracks to playlist
	if err := s.addTracksToPlaylist(ctx, playlist.ID, tracks); err != nil {
		return nil, fmt.Errorf("adding tracks to playlist: %w", err)
	}

	// Sync to Spotify if requested
	if syncToSpotify {
		if err := s.syncPlaylistToSpotify(ctx, userID, playlist, tracks); err != nil {
			return nil, fmt.Errorf("syncing playlist to Spotify: %w", err)
		}
	}

	return playlist, nil
}

// GenerateByMood creates a playlist containing tracks matching a specific mood.
func (s *playlistGeneratorService) GenerateByMood(ctx context.Context, userID uint, mood string, trackCount int, syncToSpotify bool) (*models.Playlist, error) {
	// Get all user tracks
	allTracks, err := s.trackRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding user tracks: %w", err)
	}

	// Group by mood
	moodGroups := s.analyzer.GroupByMood(allTracks)
	tracks, ok := moodGroups[mood]
	if !ok || len(tracks) == 0 {
		return nil, fmt.Errorf("no tracks found for mood: %s", mood)
	}

	// Limit track count if specified
	if trackCount > 0 && trackCount < len(tracks) {
		tracks = tracks[:trackCount]
	}

	// Create playlist record
	params, _ := json.Marshal(map[string]interface{}{
		"mood":        mood,
		"track_count": len(tracks),
	})

	playlist := &models.Playlist{
		UserID:      userID,
		Name:        fmt.Sprintf("%s Vibes", capitalize(mood)),
		Description: fmt.Sprintf("Tracks that match a %s mood", mood),
		Strategy:    models.StrategyMood,
		Params:      string(params),
		TrackCount:  len(tracks),
		IsPublic:    false,
		IsSynced:    false,
	}

	if err := s.playlistRepo.Create(ctx, playlist); err != nil {
		return nil, fmt.Errorf("creating playlist record: %w", err)
	}

	// Add tracks to playlist
	if err := s.addTracksToPlaylist(ctx, playlist.ID, tracks); err != nil {
		return nil, fmt.Errorf("adding tracks to playlist: %w", err)
	}

	// Sync to Spotify if requested
	if syncToSpotify {
		if err := s.syncPlaylistToSpotify(ctx, userID, playlist, tracks); err != nil {
			return nil, fmt.Errorf("syncing playlist to Spotify: %w", err)
		}
	}

	return playlist, nil
}

// GenerateByEnergy creates a playlist with tracks in a specific energy range.
func (s *playlistGeneratorService) GenerateByEnergy(ctx context.Context, userID uint, minEnergy, maxEnergy float64, trackCount int, syncToSpotify bool) (*models.Playlist, error) {
	// Get all user tracks
	allTracks, err := s.trackRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding user tracks: %w", err)
	}

	// Filter by energy range
	tracks := s.analyzer.FilterByEnergyRange(allTracks, minEnergy, maxEnergy)
	if len(tracks) == 0 {
		return nil, fmt.Errorf("no tracks found with energy between %.2f and %.2f", minEnergy, maxEnergy)
	}

	// Limit track count if specified
	if trackCount > 0 && trackCount < len(tracks) {
		tracks = tracks[:trackCount]
	}

	// Create playlist record
	params, _ := json.Marshal(map[string]interface{}{
		"min_energy":  minEnergy,
		"max_energy":  maxEnergy,
		"track_count": len(tracks),
	})

	energyLabel := "Medium"
	if minEnergy >= 0.7 {
		energyLabel = "High"
	} else if maxEnergy <= 0.4 {
		energyLabel = "Low"
	}

	playlist := &models.Playlist{
		UserID:      userID,
		Name:        fmt.Sprintf("%s Energy Playlist", energyLabel),
		Description: fmt.Sprintf("Tracks with energy between %.2f and %.2f", minEnergy, maxEnergy),
		Strategy:    models.StrategyEnergy,
		Params:      string(params),
		TrackCount:  len(tracks),
		IsPublic:    false,
		IsSynced:    false,
	}

	if err := s.playlistRepo.Create(ctx, playlist); err != nil {
		return nil, fmt.Errorf("creating playlist record: %w", err)
	}

	// Add tracks to playlist
	if err := s.addTracksToPlaylist(ctx, playlist.ID, tracks); err != nil {
		return nil, fmt.Errorf("adding tracks to playlist: %w", err)
	}

	// Sync to Spotify if requested
	if syncToSpotify {
		if err := s.syncPlaylistToSpotify(ctx, userID, playlist, tracks); err != nil {
			return nil, fmt.Errorf("syncing playlist to Spotify: %w", err)
		}
	}

	return playlist, nil
}

// GenerateSimilar creates a playlist of tracks similar to a given track.
func (s *playlistGeneratorService) GenerateSimilar(ctx context.Context, userID uint, trackID uint, trackCount int, syncToSpotify bool) (*models.Playlist, error) {
	// Get the seed track
	seedTrack, err := s.trackRepo.FindByID(ctx, trackID)
	if err != nil {
		return nil, fmt.Errorf("finding seed track: %w", err)
	}

	// Get all user tracks as candidates
	allTracks, err := s.trackRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding user tracks: %w", err)
	}

	// Find similar tracks
	tracks := s.analyzer.FindSimilarTracks(*seedTrack, allTracks, trackCount)
	if len(tracks) == 0 {
		return nil, fmt.Errorf("no similar tracks found")
	}

	// Create playlist record
	params, _ := json.Marshal(map[string]interface{}{
		"seed_track_id": trackID,
		"track_count":   len(tracks),
	})

	playlist := &models.Playlist{
		UserID:      userID,
		Name:        fmt.Sprintf("Similar to %s", seedTrack.Name),
		Description: fmt.Sprintf("Tracks similar to %s by %s", seedTrack.Name, seedTrack.ArtistName),
		Strategy:    models.StrategySimilarity,
		Params:      string(params),
		TrackCount:  len(tracks),
		IsPublic:    false,
		IsSynced:    false,
	}

	if err := s.playlistRepo.Create(ctx, playlist); err != nil {
		return nil, fmt.Errorf("creating playlist record: %w", err)
	}

	// Add tracks to playlist
	if err := s.addTracksToPlaylist(ctx, playlist.ID, tracks); err != nil {
		return nil, fmt.Errorf("adding tracks to playlist: %w", err)
	}

	// Sync to Spotify if requested
	if syncToSpotify {
		if err := s.syncPlaylistToSpotify(ctx, userID, playlist, tracks); err != nil {
			return nil, fmt.Errorf("syncing playlist to Spotify: %w", err)
		}
	}

	return playlist, nil
}

// GenerateTopArtists creates playlists for the user's top N artists.
func (s *playlistGeneratorService) GenerateTopArtists(ctx context.Context, userID uint, topN int, syncToSpotify bool) ([]*models.Playlist, error) {
	// Get all user tracks
	allTracks, err := s.trackRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding user tracks: %w", err)
	}

	// Get top artists
	topArtists := s.analyzer.GetTopArtists(allTracks, topN)
	if len(topArtists) == 0 {
		return nil, fmt.Errorf("no artists found")
	}

	var playlists []*models.Playlist

	// Create a playlist for each top artist
	for _, artistStat := range topArtists {
		playlist, err := s.GenerateByArtist(ctx, userID, artistStat.Name, syncToSpotify)
		if err != nil {
			fmt.Printf("Warning: failed to create playlist for artist %s: %v\n", artistStat.Name, err)
			continue
		}
		playlists = append(playlists, playlist)
	}

	return playlists, nil
}

// addTracksToPlaylist adds tracks to a playlist in the database.
func (s *playlistGeneratorService) addTracksToPlaylist(ctx context.Context, playlistID uint, tracks []models.Track) error {
	// Note: This is a simplified implementation. In a production system,
	// you'd want to use a proper repository method for playlist_tracks.
	// For now, we'll just count the tracks in the playlist record.
	return nil
}

// syncPlaylistToSpotify syncs a playlist to Spotify.
func (s *playlistGeneratorService) syncPlaylistToSpotify(ctx context.Context, userID uint, playlist *models.Playlist, tracks []models.Track) error {
	// Extract Spotify track IDs
	spotifyTrackIDs := make([]string, len(tracks))
	for i, track := range tracks {
		spotifyTrackIDs[i] = track.SpotifyTrackID
	}

	// Create playlist on Spotify
	spotifyPlaylistID, err := s.spotifyService.CreatePlaylistOnSpotify(
		ctx,
		userID,
		playlist.Name,
		playlist.Description,
		spotifyTrackIDs,
		playlist.IsPublic,
	)
	if err != nil {
		return fmt.Errorf("creating playlist on Spotify: %w", err)
	}

	// Mark playlist as synced
	if err := s.playlistRepo.MarkSynced(ctx, playlist.ID, spotifyPlaylistID); err != nil {
		return fmt.Errorf("marking playlist as synced: %w", err)
	}

	playlist.IsSynced = true
	playlist.SpotifyPlaylistID = spotifyPlaylistID

	return nil
}

// capitalize capitalizes the first letter of a string.
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return fmt.Sprintf("%c%s", s[0]-32, s[1:])
}
