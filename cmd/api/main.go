package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prestonfoshee/bopbridge/internal/config"
	"github.com/prestonfoshee/bopbridge/internal/database"
	"github.com/prestonfoshee/bopbridge/internal/handlers"
	"github.com/prestonfoshee/bopbridge/internal/middleware"
	"github.com/prestonfoshee/bopbridge/internal/repository"
	"github.com/prestonfoshee/bopbridge/internal/services"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
)

func main() {
	// Initialize logger
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log := zlog.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msg("Starting BopBridge API server...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	log.Info().Str("env", cfg.Server.Env).Str("port", cfg.Server.Port).Msg("Configuration loaded")

	// Connect to database
	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}

	log.Info().Msg("Connected to database")

	// Verify database is ready by pinging
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to get database instance")
	}
	
	if err := sqlDB.Ping(); err != nil {
		log.Fatal().Err(err).Msg("Failed to ping database")
	}

	log.Info().Msg("Database is ready")

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	playlistRepo := repository.NewPlaylistRepository(db)
	trackRepo := repository.NewTrackRepository(db)

	log.Info().Msg("Repositories initialized")

	// Initialize services
	spotifyService := services.NewSpotifyService(&cfg.Spotify, userRepo)
	trackService := services.NewTrackService(trackRepo, spotifyService)
	analyzerService := services.NewAnalyzerService()
	playlistGenerator := services.NewPlaylistGeneratorService(playlistRepo, trackRepo, spotifyService, analyzerService)

	log.Info().Msg("Services initialized")

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(spotifyService, &cfg.JWT)
	trackHandler := handlers.NewTrackHandler(trackService, analyzerService)
	playlistHandler := handlers.NewPlaylistHandler(playlistRepo, playlistGenerator)

	log.Info().Msg("Handlers initialized")

	// Setup Gin router
	if cfg.Server.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"time":   time.Now().UTC(),
			"toots": "https://toots.gg/prestonfoshee",
		})
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Auth routes (public)
		auth := v1.Group("/auth")
		{
			auth.GET("/login", authHandler.LoginHandler)
			auth.GET("/callback", authHandler.CallbackHandler)
			auth.POST("/logout", authHandler.LogoutHandler)
			auth.GET("/me", middleware.RequireAuth(cfg.JWT.Secret), authHandler.MeHandler)
		}

		// Track routes (protected)
		tracks := v1.Group("/tracks")
		tracks.Use(middleware.RequireAuth(cfg.JWT.Secret))
		{
			tracks.POST("/sync", trackHandler.SyncTracksHandler)
			tracks.POST("/sync-features", trackHandler.SyncAudioFeaturesHandler)
			tracks.GET("", trackHandler.GetTracksHandler)
			tracks.GET("/stats", trackHandler.GetTrackStatsHandler)
			tracks.GET("/artists", trackHandler.GetArtistsHandler)
			tracks.GET("/artist/:name", trackHandler.GetTracksByArtistHandler)
		}

		// Playlist routes (protected)
		playlists := v1.Group("/playlists")
		playlists.Use(middleware.RequireAuth(cfg.JWT.Secret))
		{
			playlists.POST("/generate", playlistHandler.GeneratePlaylistHandler)
			playlists.POST("/generate-top-artists", playlistHandler.GenerateTopArtistsPlaylistsHandler)
			playlists.GET("", playlistHandler.GetPlaylistsHandler)
			playlists.GET("/:id", playlistHandler.GetPlaylistHandler)
			playlists.DELETE("/:id", playlistHandler.DeletePlaylistHandler)
		}
	}

	log.Info().Msg("Routes configured")

	// Setup HTTP server
	srv := &http.Server{
		Addr:           ":" + cfg.Server.Port,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	// Start server in a goroutine
	go func() {
		log.Info().Str("port", cfg.Server.Port).Msg("Starting HTTP server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	// Graceful shutdown with 5 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exited")
}
