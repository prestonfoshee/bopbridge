package database

import (
	"fmt"
	"time"

	"github.com/prestonfoshee/bopbridge/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// NewConnection creates and returns a new PostgreSQL database connection using GORM.
// It accepts a DatabaseConfig and returns a concrete *gorm.DB instance that can be
// passed directly to repositories.
//
// Connection pooling is automatically handled by GORM/pgx with the following defaults:
//   - MaxIdleConns: 10 (max idle connections in the pool)
//   - MaxOpenConns: 100 (max open connections to the database)
//   - ConnMaxLifetime: 1 hour (max time a connection can be reused)
//
// This function follows the Go pattern "Accept interfaces, return structs" by
// returning the concrete *gorm.DB type, which repositories need for specific functionality.
func NewConnection(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying *sql.DB for connection pool configuration
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Configure connection pool settings
	sqlDB.SetMaxIdleConns(10)           // Max idle connections in the pool
	sqlDB.SetMaxOpenConns(100)          // Max open connections to the database
	sqlDB.SetConnMaxLifetime(time.Hour) // Max time a connection can be reused

	return db, nil
}