package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prestonfoshee/bopbridge/internal/config"
	"github.com/prestonfoshee/bopbridge/internal/models"
	"github.com/prestonfoshee/bopbridge/internal/repository"
	"github.com/rs/zerolog/log"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
)

// SpotifyService defines the interface for interacting with the Spotify API.
// It handles OAuth authentication, token management, and wraps Spotify API calls.
type SpotifyService interface {
	// GetAuthURL generates the Spotify OAuth authorization URL.
	GetAuthURL(state string) string

	// ExchangeCode exchanges an OAuth authorization code for access and refresh tokens.
	// It creates or updates the user in the database with the tokens.
	ExchangeCode(ctx context.Context, code string) (*models.User, error)

	// FetchAllLikedSongs retrieves all liked songs for a user from Spotify.
	// It automatically handles pagination (Spotify returns max 50 tracks per request).
	FetchAllLikedSongs(ctx context.Context, userID uint) ([]models.Track, error)

	// FetchLikedSongsStream fetches liked songs and streams batches through a channel.
	// This allows pipelining with database inserts for maximum throughput.
	// The caller must consume from tracksChan until it's closed.
	// Returns the total number of tracks and any error that occurred.
	FetchLikedSongsStream(ctx context.Context, userID uint, tracksChan chan<- []models.Track) (int, error)

	// FetchAudioFeatures retrieves audio features for multiple tracks from Spotify.
	// It automatically handles batching (Spotify accepts max 100 tracks per request).
	FetchAudioFeatures(ctx context.Context, userID uint, trackIDs []string) (map[string]*models.AudioFeatures, error)

	// CreatePlaylistOnSpotify creates a new playlist on Spotify and adds tracks to it.
	// Returns the Spotify playlist ID.
	CreatePlaylistOnSpotify(ctx context.Context, userID uint, name, description string, trackIDs []string, isPublic bool) (string, error)

	// RefreshUserToken refreshes the OAuth access token for a user.
	RefreshUserToken(ctx context.Context, userID uint) error

	// GetClient returns an authenticated Spotify client for a user.
	// It automatically refreshes the token if needed.
	GetClient(ctx context.Context, userID uint) (*spotify.Client, error)
}

// spotifyService is the concrete implementation of SpotifyService.
type spotifyService struct {
	config   *config.SpotifyConfig
	auth     *spotifyauth.Authenticator
	userRepo repository.UserRepository
}

// NewSpotifyService creates a new SpotifyService with the given configuration and repository.
func NewSpotifyService(cfg *config.SpotifyConfig, userRepo repository.UserRepository) SpotifyService {
	auth := spotifyauth.New(
		spotifyauth.WithRedirectURL(cfg.RedirectURI),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadPrivate,
			spotifyauth.ScopeUserReadEmail,
			spotifyauth.ScopeUserLibraryRead,
			spotifyauth.ScopePlaylistModifyPublic,
			spotifyauth.ScopePlaylistModifyPrivate,
		),
		spotifyauth.WithClientID(cfg.ClientID),
		spotifyauth.WithClientSecret(cfg.ClientSecret),
	)

	return &spotifyService{
		config:   cfg,
		auth:     auth,
		userRepo: userRepo,
	}
}

// GetAuthURL generates the Spotify OAuth authorization URL.
func (s *spotifyService) GetAuthURL(state string) string {
	return s.auth.AuthURL(state)
}

// ExchangeCode exchanges an OAuth authorization code for access and refresh tokens.
func (s *spotifyService) ExchangeCode(ctx context.Context, code string) (*models.User, error) {
	token, err := s.auth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchanging code for token: %w", err)
	}

	// Create authenticated Spotify client
	client := spotify.New(s.auth.Client(ctx, token))

	// Fetch user profile from Spotify
	spotifyUser, err := client.CurrentUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetching Spotify user profile: %w", err)
	}

	// Check if user already exists
	existingUser, err := s.userRepo.FindBySpotifyID(ctx, spotifyUser.ID)
	if err != nil && err != repository.ErrNotFound {
		return nil, fmt.Errorf("checking for existing user: %w", err)
	}

	// Calculate token expiry
	expiry := time.Now().Add(time.Hour) // Spotify tokens typically last 1 hour
	if token.Expiry.After(time.Now()) {
		expiry = token.Expiry
	}

	if existingUser != nil {
		// Update existing user's tokens
		existingUser.AccessToken = token.AccessToken
		existingUser.RefreshToken = token.RefreshToken
		existingUser.TokenExpiry = expiry
		existingUser.DisplayName = spotifyUser.DisplayName
		if len(spotifyUser.Email) > 0 {
			existingUser.Email = spotifyUser.Email
		}

		if err := s.userRepo.Update(ctx, existingUser); err != nil {
			return nil, fmt.Errorf("updating existing user: %w", err)
		}
		return existingUser, nil
	}

	// Create new user
	user := &models.User{
		SpotifyID:    spotifyUser.ID,
		Email:        spotifyUser.Email,
		DisplayName:  spotifyUser.DisplayName,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenExpiry:  expiry,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("creating new user: %w", err)
	}

	return user, nil
}

// FetchAllLikedSongs retrieves all liked songs for a user from Spotify.
// Uses concurrent goroutines to fetch batches in parallel for maximum speed.
func (s *spotifyService) FetchAllLikedSongs(ctx context.Context, userID uint) ([]models.Track, error) {
	startTime := time.Now()
	log.Debug().Uint("user_id", userID).Msg("Getting Spotify client for user")

	client, err := s.GetClient(ctx, userID)
	if err != nil {
		log.Error().Err(err).Uint("user_id", userID).Msg("Failed to get Spotify client")
		return nil, fmt.Errorf("getting Spotify client: %w", err)
	}

	log.Debug().Uint("user_id", userID).Msg("Spotify client obtained, fetching total track count")

	// Step 1: Get total track count with initial API call
	firstBatch, err := client.CurrentUsersTracks(ctx, spotify.Limit(1), spotify.Offset(0))
	if err != nil {
		log.Error().Err(err).Uint("user_id", userID).Msg("Failed to get track count")
		return nil, fmt.Errorf("getting track count: %w", err)
	}

	totalTracks := int(firstBatch.Total)
	if totalTracks == 0 {
		log.Warn().Uint("user_id", userID).Msg("No liked songs found in Spotify library")
		return []models.Track{}, nil
	}

	const limit = 50              // Spotify's maximum per request
	const maxConcurrent = 5       // Maximum concurrent API requests (reduced from 10 to avoid rate limits)

	numBatches := (totalTracks + limit - 1) / limit // Ceiling division

	log.Info().
		Uint("user_id", userID).
		Int("total_tracks", totalTracks).
		Int("num_batches", numBatches).
		Int("max_concurrent", maxConcurrent).
		Msg("Starting parallel fetch of liked songs")

	// Step 2: Create channel for collecting track batches
	// Buffered channel to avoid blocking workers
	tracksChan := make(chan []models.Track, numBatches)

	// Step 3: Spawn concurrent fetch workers using errgroup
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrent)

	for i := 0; i < numBatches; i++ {
		offset := i * limit
		batchNum := i + 1

		g.Go(func() error {
			log.Debug().
				Uint("user_id", userID).
				Int("batch", batchNum).
				Int("offset", offset).
				Msg("Fetching batch concurrently")

			savedTracks, err := client.CurrentUsersTracks(gCtx, spotify.Limit(limit), spotify.Offset(offset))
			if err != nil {
				log.Error().
					Err(err).
					Uint("user_id", userID).
					Int("batch", batchNum).
					Int("offset", offset).
					Msg("Failed to fetch batch")
				return fmt.Errorf("fetching batch at offset %d: %w", offset, err)
			}

			// Convert Spotify tracks to our model
			tracks := convertSavedTracksToModels(savedTracks.Tracks, userID)

			log.Debug().
				Uint("user_id", userID).
				Int("batch", batchNum).
				Int("tracks_fetched", len(tracks)).
				Msg("Batch fetched successfully")

			tracksChan <- tracks
			return nil
		})
	}

	// Step 4: Collect results in a separate goroutine
	var allTracks []models.Track
	var collectErr error
	var mu sync.Mutex
	collectDone := make(chan struct{})

	go func() {
		defer close(collectDone)
		for batch := range tracksChan {
			mu.Lock()
			allTracks = append(allTracks, batch...)
			mu.Unlock()
		}
	}()

	// Step 5: Wait for all fetchers to complete, then close channel
	fetchErr := g.Wait()
	close(tracksChan)

	// Wait for collector to finish
	<-collectDone

	if fetchErr != nil {
		log.Error().Err(fetchErr).Uint("user_id", userID).Msg("Parallel fetch failed")
		return nil, fetchErr
	}
	if collectErr != nil {
		log.Error().Err(collectErr).Uint("user_id", userID).Msg("Track collection failed")
		return nil, collectErr
	}

	duration := time.Since(startTime)
	log.Info().
		Uint("user_id", userID).
		Int("total_tracks", len(allTracks)).
		Int("num_batches", numBatches).
		Dur("duration", duration).
		Float64("tracks_per_second", float64(len(allTracks))/duration.Seconds()).
		Msg("Parallel fetch completed successfully")

	return allTracks, nil
}

// FetchLikedSongsStream fetches liked songs and streams batches through a channel.
// This allows pipelining with database inserts for maximum throughput.
// The channel will be closed when fetching is complete.
func (s *spotifyService) FetchLikedSongsStream(ctx context.Context, userID uint, tracksChan chan<- []models.Track) (int, error) {
	startTime := time.Now()
	log.Debug().Uint("user_id", userID).Msg("Getting Spotify client for streaming fetch")

	client, err := s.GetClient(ctx, userID)
	if err != nil {
		log.Error().Err(err).Uint("user_id", userID).Msg("Failed to get Spotify client")
		close(tracksChan)
		return 0, fmt.Errorf("getting Spotify client: %w", err)
	}

	// Get total track count with initial API call
	firstBatch, err := client.CurrentUsersTracks(ctx, spotify.Limit(1), spotify.Offset(0))
	if err != nil {
		log.Error().Err(err).Uint("user_id", userID).Msg("Failed to get track count")
		close(tracksChan)
		return 0, fmt.Errorf("getting track count: %w", err)
	}

	totalTracks := int(firstBatch.Total)
	if totalTracks == 0 {
		log.Warn().Uint("user_id", userID).Msg("No liked songs found in Spotify library")
		close(tracksChan)
		return 0, nil
	}

	const limit = 50        // Spotify's maximum per request
	const maxConcurrent = 5 // Maximum concurrent API requests (reduced from 10 to avoid rate limits)

	numBatches := (totalTracks + limit - 1) / limit

	log.Info().
		Uint("user_id", userID).
		Int("total_tracks", totalTracks).
		Int("num_batches", numBatches).
		Int("max_concurrent", maxConcurrent).
		Msg("Starting parallel streaming fetch of liked songs")

	// Track total fetched for return value
	var totalFetched int
	var fetchMu sync.Mutex

	// Spawn concurrent fetch workers
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrent)

	for i := 0; i < numBatches; i++ {
		offset := i * limit
		batchNum := i + 1

		g.Go(func() error {
			log.Debug().
				Uint("user_id", userID).
				Int("batch", batchNum).
				Int("offset", offset).
				Msg("Fetching batch concurrently (streaming)")

			savedTracks, err := client.CurrentUsersTracks(gCtx, spotify.Limit(limit), spotify.Offset(offset))
			if err != nil {
				log.Error().
					Err(err).
					Uint("user_id", userID).
					Int("batch", batchNum).
					Int("offset", offset).
					Msg("Failed to fetch batch")
				return fmt.Errorf("fetching batch at offset %d: %w", offset, err)
			}

			tracks := convertSavedTracksToModels(savedTracks.Tracks, userID)

			fetchMu.Lock()
			totalFetched += len(tracks)
			fetchMu.Unlock()

			log.Debug().
				Uint("user_id", userID).
				Int("batch", batchNum).
				Int("tracks_fetched", len(tracks)).
				Msg("Streaming batch to channel")

			// Send to channel (will block if consumer is slow, providing backpressure)
			select {
			case tracksChan <- tracks:
			case <-gCtx.Done():
				return gCtx.Err()
			}

			return nil
		})
	}

	// Wait for all fetchers to complete, then close channel
	fetchErr := g.Wait()
	close(tracksChan)

	duration := time.Since(startTime)
	if fetchErr != nil {
		log.Error().Err(fetchErr).Uint("user_id", userID).Dur("duration", duration).Msg("Streaming fetch failed")
		return totalFetched, fetchErr
	}

	log.Info().
		Uint("user_id", userID).
		Int("total_tracks", totalFetched).
		Int("num_batches", numBatches).
		Dur("duration", duration).
		Float64("tracks_per_second", float64(totalFetched)/duration.Seconds()).
		Msg("Streaming fetch completed successfully")

	return totalFetched, nil
}

// convertSavedTracksToModels converts Spotify saved tracks to our Track model.
func convertSavedTracksToModels(savedTracks []spotify.SavedTrack, userID uint) []models.Track {
	tracks := make([]models.Track, 0, len(savedTracks))

	for _, item := range savedTracks {
		// Parse the AddedAt timestamp
		addedAt := time.Now()
		if item.AddedAt != "" {
			if parsed, err := time.Parse(time.RFC3339, item.AddedAt); err == nil {
				addedAt = parsed
			}
		}

		track := models.Track{
			UserID:         userID,
			SpotifyTrackID: item.ID.String(),
			Name:           item.Name,
			ArtistName:     getFirstArtistName(item.Artists),
			AlbumName:      item.Album.Name,
			IsLiked:        true,
			AddedAt:        addedAt,
		}
		tracks = append(tracks, track)
	}

	return tracks
}

// @todo: the audio features endpoint has been deprecated. we need to find an alternative solution.
// FetchAudioFeatures retrieves audio features for multiple tracks from Spotify.
func (s *spotifyService) FetchAudioFeatures(ctx context.Context, userID uint, trackIDs []string) (map[string]*models.AudioFeatures, error) {
	if len(trackIDs) == 0 {
		return make(map[string]*models.AudioFeatures), nil
	}

	client, err := s.GetClient(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting Spotify client: %w", err)
	}

	featuresMap := make(map[string]*models.AudioFeatures)
	batchSize := 100 // Spotify's maximum for audio features

	for i := 0; i < len(trackIDs); i += batchSize {
		end := i + batchSize
		if end > len(trackIDs) {
			end = len(trackIDs)
		}
		batch := trackIDs[i:end]

		// Convert string IDs to spotify.ID
		spotifyIDs := make([]spotify.ID, len(batch))
		for j, id := range batch {
			spotifyIDs[j] = spotify.ID(id)
		}

		features, err := client.GetAudioFeatures(ctx, spotifyIDs...)
		if err != nil {
			return nil, fmt.Errorf("fetching audio features (batch starting at %d): %w", i, err)
		}

		// Convert Spotify audio features to our model
		for j, feature := range features {
			if feature == nil {
				continue // Some tracks may not have audio features
			}
			featuresMap[batch[j]] = &models.AudioFeatures{
				Acousticness:     float64(feature.Acousticness),
				Danceability:     float64(feature.Danceability),
				Energy:           float64(feature.Energy),
				Instrumentalness: float64(feature.Instrumentalness),
				Liveness:         float64(feature.Liveness),
				Loudness:         float64(feature.Loudness),
				Speechiness:      float64(feature.Speechiness),
				Valence:          float64(feature.Valence),
				Tempo:            float64(feature.Tempo),
				Key:              int(feature.Key),
				Mode:             int(feature.Mode),
				TimeSignature:    int(feature.TimeSignature),
				DurationMs:       int(feature.Duration),
			}
		}
	}

	return featuresMap, nil
}

// CreatePlaylistOnSpotify creates a new playlist on Spotify and adds tracks to it.
func (s *spotifyService) CreatePlaylistOnSpotify(ctx context.Context, userID uint, name, description string, trackIDs []string, isPublic bool) (string, error) {
	client, err := s.GetClient(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("getting Spotify client: %w", err)
	}

	// Get current user to create playlist for them
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("finding user: %w", err)
	}

	// Create the playlist
	playlist, err := client.CreatePlaylistForUser(ctx, user.SpotifyID, name, description, isPublic, false)
	if err != nil {
		return "", fmt.Errorf("creating Spotify playlist: %w", err)
	}

	// Add tracks to the playlist (Spotify accepts max 100 tracks per request)
	if len(trackIDs) > 0 {
		batchSize := 100
		for i := 0; i < len(trackIDs); i += batchSize {
			end := i + batchSize
			if end > len(trackIDs) {
				end = len(trackIDs)
			}
			batch := trackIDs[i:end]

			// Convert string IDs to spotify.ID
			spotifyIDs := make([]spotify.ID, len(batch))
			for j, id := range batch {
				spotifyIDs[j] = spotify.ID(id)
			}

			_, err := client.AddTracksToPlaylist(ctx, playlist.ID, spotifyIDs...)
			if err != nil {
				return "", fmt.Errorf("adding tracks to playlist (batch starting at %d): %w", i, err)
			}
		}
	}

	return playlist.ID.String(), nil
}

// RefreshUserToken refreshes the OAuth access token for a user.
func (s *spotifyService) RefreshUserToken(ctx context.Context, userID uint) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("finding user: %w", err)
	}

	// Create token source for refreshing
	token := &oauth2.Token{
		AccessToken:  user.AccessToken,
		RefreshToken: user.RefreshToken,
		Expiry:       user.TokenExpiry,
	}

	tokenSource := s.auth.Client(ctx, token).Transport.(*oauth2.Transport).Source
	newToken, err := tokenSource.Token()
	if err != nil {
		return fmt.Errorf("refreshing token: %w", err)
	}

	// Update user's tokens in database
	expiry := time.Now().Add(time.Hour)
	if newToken.Expiry.After(time.Now()) {
		expiry = newToken.Expiry
	}

	if err := s.userRepo.UpdateTokens(ctx, userID, newToken.AccessToken, newToken.RefreshToken, expiry); err != nil {
		return fmt.Errorf("updating user tokens: %w", err)
	}

	return nil
}

// GetClient returns an authenticated Spotify client for a user.
func (s *spotifyService) GetClient(ctx context.Context, userID uint) (*spotify.Client, error) {
	log.Debug().Uint("user_id", userID).Msg("Looking up user for Spotify client")
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		log.Error().Err(err).Uint("user_id", userID).Msg("Failed to find user")
		return nil, fmt.Errorf("finding user: %w", err)
	}

	log.Debug().
		Uint("user_id", userID).
		Str("spotify_id", user.SpotifyID).
		Time("token_expiry", user.TokenExpiry).
		Dur("time_until_expiry", time.Until(user.TokenExpiry)).
		Msg("Found user, checking token expiry")

	// Check if token needs refresh (within 5 minutes of expiry)
	if time.Until(user.TokenExpiry) < 5*time.Minute {
		log.Info().Uint("user_id", userID).Msg("Token is expiring soon, refreshing...")
		if err := s.RefreshUserToken(ctx, userID); err != nil {
			log.Error().Err(err).Uint("user_id", userID).Msg("Failed to refresh token")
			return nil, fmt.Errorf("refreshing token: %w", err)
		}
		log.Info().Uint("user_id", userID).Msg("Token refreshed successfully")
		// Reload user with fresh token
		user, err = s.userRepo.FindByID(ctx, userID)
		if err != nil {
			log.Error().Err(err).Uint("user_id", userID).Msg("Failed to reload user after token refresh")
			return nil, fmt.Errorf("reloading user after token refresh: %w", err)
		}
	}

	token := &oauth2.Token{
		AccessToken:  user.AccessToken,
		RefreshToken: user.RefreshToken,
		Expiry:       user.TokenExpiry,
	}

	httpClient := s.auth.Client(ctx, token)
	client := spotify.New(httpClient)

	log.Debug().Uint("user_id", userID).Msg("Spotify client created successfully")
	return client, nil
}

// getFirstArtistName is a helper function to extract the first artist name from a list of artists.
func getFirstArtistName(artists []spotify.SimpleArtist) string {
	if len(artists) == 0 {
		return "Unknown Artist"
	}
	return artists[0].Name
}
