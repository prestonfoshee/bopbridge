package services

import (
	"context"
	"fmt"
	"time"

	"github.com/prestonfoshee/bopbridge/internal/config"
	"github.com/prestonfoshee/bopbridge/internal/models"
	"github.com/prestonfoshee/bopbridge/internal/repository"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
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
func (s *spotifyService) FetchAllLikedSongs(ctx context.Context, userID uint) ([]models.Track, error) {
	client, err := s.GetClient(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting Spotify client: %w", err)
	}

	var allTracks []models.Track
	offset := 0
	limit := 50 // Spotify's maximum

	for {
		savedTracks, err := client.CurrentUsersTracks(ctx, spotify.Limit(limit), spotify.Offset(offset))
		if err != nil {
			return nil, fmt.Errorf("fetching liked songs (offset %d): %w", offset, err)
		}

		// Convert Spotify tracks to our model
		for _, item := range savedTracks.Tracks {
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
			allTracks = append(allTracks, track)
		}

		// Check if we've fetched all tracks
		if len(savedTracks.Tracks) < limit {
			break
		}
		offset += limit
	}

	return allTracks, nil
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
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding user: %w", err)
	}

	// Check if token needs refresh (within 5 minutes of expiry)
	if time.Until(user.TokenExpiry) < 5*time.Minute {
		if err := s.RefreshUserToken(ctx, userID); err != nil {
			return nil, fmt.Errorf("refreshing token: %w", err)
		}
		// Reload user with fresh token
		user, err = s.userRepo.FindByID(ctx, userID)
		if err != nil {
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

	return client, nil
}

// getFirstArtistName is a helper function to extract the first artist name from a list of artists.
func getFirstArtistName(artists []spotify.SimpleArtist) string {
	if len(artists) == 0 {
		return "Unknown Artist"
	}
	return artists[0].Name
}
