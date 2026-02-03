package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/prestonfoshee/bopbridge/internal/middleware"
	"github.com/prestonfoshee/bopbridge/internal/repository"
	"github.com/prestonfoshee/bopbridge/internal/services"
	"github.com/prestonfoshee/bopbridge/internal/utils"
)

// PlaylistHandler handles playlist-related HTTP requests.
type PlaylistHandler struct {
	playlistRepo      repository.PlaylistRepository
	playlistGenerator services.PlaylistGeneratorService
}

// NewPlaylistHandler creates a new PlaylistHandler.
func NewPlaylistHandler(
	playlistRepo repository.PlaylistRepository,
	playlistGenerator services.PlaylistGeneratorService,
) *PlaylistHandler {
	return &PlaylistHandler{
		playlistRepo:      playlistRepo,
		playlistGenerator: playlistGenerator,
	}
}

// GeneratePlaylistRequest represents the request body for playlist generation.
type GeneratePlaylistRequest struct {
	Strategy      string  `json:"strategy" binding:"required"`       // artist, genre, mood, energy, similar
	ArtistName    string  `json:"artist_name,omitempty"`            // For artist strategy
	Genre         string  `json:"genre,omitempty"`                   // For genre strategy
	Mood          string  `json:"mood,omitempty"`                    // For mood strategy
	MinEnergy     float64 `json:"min_energy,omitempty"`              // For energy strategy
	MaxEnergy     float64 `json:"max_energy,omitempty"`              // For energy strategy
	SeedTrackID   uint    `json:"seed_track_id,omitempty"`           // For similar strategy
	TrackCount    int     `json:"track_count,omitempty"`             // Limit number of tracks
	SyncToSpotify bool    `json:"sync_to_spotify"`                   // Whether to sync to Spotify
}

// GeneratePlaylistHandler generates a playlist based on the specified strategy.
// POST /api/v1/playlists/generate
func (h *PlaylistHandler) GeneratePlaylistHandler(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User ID not found")
		return
	}

	var req GeneratePlaylistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequestResponse(c, "Invalid request body")
		return
	}

	var playlist interface{}
	var err error

	switch req.Strategy {
	case "artist":
		if req.ArtistName == "" {
			utils.BadRequestResponse(c, "artist_name is required for artist strategy")
			return
		}
		playlist, err = h.playlistGenerator.GenerateByArtist(c.Request.Context(), userID, req.ArtistName, req.SyncToSpotify)

	case "genre":
		if req.Genre == "" {
			utils.BadRequestResponse(c, "genre is required for genre strategy")
			return
		}
		playlist, err = h.playlistGenerator.GenerateByGenre(c.Request.Context(), userID, req.Genre, req.SyncToSpotify)

	case "mood":
		if req.Mood == "" {
			utils.BadRequestResponse(c, "mood is required for mood strategy")
			return
		}
		playlist, err = h.playlistGenerator.GenerateByMood(c.Request.Context(), userID, req.Mood, req.TrackCount, req.SyncToSpotify)

	case "energy":
		if req.MinEnergy == 0 && req.MaxEnergy == 0 {
			utils.BadRequestResponse(c, "min_energy and max_energy are required for energy strategy")
			return
		}
		playlist, err = h.playlistGenerator.GenerateByEnergy(c.Request.Context(), userID, req.MinEnergy, req.MaxEnergy, req.TrackCount, req.SyncToSpotify)

	case "similar":
		if req.SeedTrackID == 0 {
			utils.BadRequestResponse(c, "seed_track_id is required for similar strategy")
			return
		}
		playlist, err = h.playlistGenerator.GenerateSimilar(c.Request.Context(), userID, req.SeedTrackID, req.TrackCount, req.SyncToSpotify)

	default:
		utils.BadRequestResponse(c, "Invalid strategy. Must be one of: artist, genre, mood, energy, similar")
		return
	}

	if err != nil {
		utils.InternalErrorResponse(c, err)
		return
	}

	utils.SuccessMessageResponse(c, http.StatusCreated, "Playlist generated successfully", gin.H{
		"playlist": playlist,
	})
}

// GenerateTopArtistsPlaylistsHandler generates playlists for the user's top artists.
// POST /api/v1/playlists/generate-top-artists
func (h *PlaylistHandler) GenerateTopArtistsPlaylistsHandler(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User ID not found")
		return
	}

	// Parse query parameters
	topN := 5 // Default
	if topNStr := c.Query("top_n"); topNStr != "" {
		if n, err := strconv.Atoi(topNStr); err == nil && n > 0 {
			topN = n
		}
	}

	syncToSpotify := c.Query("sync_to_spotify") == "true"

	playlists, err := h.playlistGenerator.GenerateTopArtists(c.Request.Context(), userID, topN, syncToSpotify)
	if err != nil {
		utils.InternalErrorResponse(c, err)
		return
	}

	utils.SuccessMessageResponse(c, http.StatusCreated, "Playlists generated successfully", gin.H{
		"playlists": playlists,
		"count":     len(playlists),
	})
}

// GetPlaylistsHandler retrieves all playlists for the authenticated user.
// GET /api/v1/playlists
func (h *PlaylistHandler) GetPlaylistsHandler(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User ID not found")
		return
	}

	playlists, err := h.playlistRepo.FindByUserID(c.Request.Context(), userID)
	if err != nil {
		utils.InternalErrorResponse(c, err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, gin.H{
		"playlists": playlists,
		"count":     len(playlists),
	})
}

// GetPlaylistHandler retrieves a specific playlist by ID.
// GET /api/v1/playlists/:id
func (h *PlaylistHandler) GetPlaylistHandler(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User ID not found")
		return
	}

	playlistID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid playlist ID")
		return
	}

	playlist, err := h.playlistRepo.FindByID(c.Request.Context(), uint(playlistID))
	if err != nil {
		if err == repository.ErrNotFound {
			utils.NotFoundResponse(c, "Playlist")
			return
		}
		utils.InternalErrorResponse(c, err)
		return
	}

	// Verify ownership
	if playlist.UserID != userID {
		utils.ForbiddenResponse(c, "You don't have access to this playlist")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, gin.H{
		"playlist": playlist,
	})
}

// DeletePlaylistHandler deletes a playlist.
// DELETE /api/v1/playlists/:id
func (h *PlaylistHandler) DeletePlaylistHandler(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		utils.UnauthorizedResponse(c, "User ID not found")
		return
	}

	playlistID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequestResponse(c, "Invalid playlist ID")
		return
	}

	// Verify ownership before deleting
	playlist, err := h.playlistRepo.FindByID(c.Request.Context(), uint(playlistID))
	if err != nil {
		if err == repository.ErrNotFound {
			utils.NotFoundResponse(c, "Playlist")
			return
		}
		utils.InternalErrorResponse(c, err)
		return
	}

	if playlist.UserID != userID {
		utils.ForbiddenResponse(c, "You don't have access to this playlist")
		return
	}

	if err := h.playlistRepo.Delete(c.Request.Context(), uint(playlistID)); err != nil {
		utils.InternalErrorResponse(c, err)
		return
	}

	utils.SuccessMessageResponse(c, http.StatusOK, "Playlist deleted successfully", nil)
}
