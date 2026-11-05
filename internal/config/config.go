package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBUser     string
	DBPassword string
	DBName     string
	HTTPPort   string
	DBHost     string
}

const DEFAULT_PORT = "8000"

func LoadConfig() (Config, error) {
	// Optional local development configuration.
	_ = godotenv.Load()

	config := Config{
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),
		DBHost:     os.Getenv("DB_HOST"),
		HTTPPort:   os.Getenv("HTTP_PORT"),
	}

	if config.DBUser == "" {
		return Config{}, errors.New("DB_USER is required")
	}

	if config.DBName == "" {
		return Config{}, errors.New("DB_NAME is required")
	}

	if config.DBHost == "" {
		config.DBHost = "127.0.0.1:3306"
	}

	if config.HTTPPort == "" {
		config.HTTPPort = DEFAULT_PORT
	}

	return config, nil
}
