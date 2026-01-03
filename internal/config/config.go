package config

import (
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Spotify SpotifyConfig `mapstructure:"spotify"`
	JWT JWTConfig `mapstructure:"jwt"`
}

type ServerConfig struct {
	Port string `mapstructure:"port"`
	Env string `mapstructure:"env"`
}

type DatabaseConfig struct {
	Host string `mapstructure:"host"`
	Port string `mapstructure:"port"`
	User string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName string `mapstructure:"dbname"`
}

type SpotifyConfig struct {
	ClientID string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
}

type JWTConfig struct {
	Secret string `mapstructure:"secret"`
	ExpirationTime int `mapstructure:"expiration_time"`
}

func Load() (*Config, error) {
	// Load .env file for local development
	godotenv.Load()
	
	// Tell Viper to read from environment variables
	viper.AutomaticEnv()
	
	// Set defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.env", "development")
	
	// Map environment variables to config structure
	viper.SetEnvPrefix("") // No prefix, use exact names
	
	// Bind specific env vars
	viper.BindEnv("database.host", "DB_HOST")
	viper.BindEnv("database.port", "DB_PORT")
	viper.BindEnv("database.user", "DB_USER")
	viper.BindEnv("database.password", "DB_PASSWORD")
	viper.BindEnv("database.dbname", "DB_NAME")
	// ... bind others similarly
	
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}
	
	return &config, nil
}
