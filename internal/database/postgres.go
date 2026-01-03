package database

import (
	"fmt"
	"time"

	"github.com/prestonfoshee/bopbridge/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PostgresDB struct {
	db *gorm.DB
}

// GetDB returns the underlying GORM database instance
func (p *PostgresDB) GetDB() *gorm.DB {
	return p.db
}

// Close closes the database connection gracefully
// Call this during application shutdown to properly release resources
func (p *PostgresDB) Close() error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}
	return sqlDB.Close()
}

// NewPostgresDB creates a new PostgreSQL database connection with connection pooling
func NewPostgresDB(cfg config.DatabaseConfig) (*PostgresDB, error) {
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

	return &PostgresDB{db: db}, nil
}