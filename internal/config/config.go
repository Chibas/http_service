package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBUser     string
	DBPassword string
	DBName     string
	HTTPPort   string
}

const DEFAULT_PORT = "8000"

func LoadConfig() (Config, error) {
	var config = Config{
		HTTPPort: DEFAULT_PORT,
	}
	if err := godotenv.Load(".env"); err != nil {
		return Config{}, fmt.Errorf("Error loading .env: %w", err)
	}
	config.DBUser = os.Getenv("DB_USER")
	if config.DBUser == "" {
		return Config{}, errors.New("DB USER MISSING IN .env")
	}
	config.DBName = os.Getenv("DB_NAME")
	if config.DBName == "" {
		return Config{}, errors.New("DB NAME MISSING IN .env")
	}
	config.DBPassword = os.Getenv("DB_PASSWORD")
	if port := os.Getenv("HTTP_PORT"); port != "" {
		config.HTTPPort = port
	}

	return config, nil
}
