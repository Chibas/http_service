package util

import (
	"database/sql"
	"os"
	"testing"

	"github.com/Chibas/http_service/internal/config"
	"github.com/Chibas/http_service/internal/database"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

// TestConnectDB selects a test database using the caller's environment variable.
func TestConnectDB(t *testing.T, databaseNameEnv string) *sql.DB {
	t.Helper()
	_ = godotenv.Load("../../.env")

	cfg := config.Config{
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv(databaseNameEnv),
		DBHost:     os.Getenv("DB_HOST"),
		HTTPPort:   os.Getenv("HTTP_PORT"),
	}
	if cfg.DBHost == "" {
		cfg.DBHost = "127.0.0.1:3306"
	}
	if cfg.DBUser == "" {
		t.Fatal("DB_USER is required")
	}
	if cfg.DBName == "" {
		t.Fatalf("%s is required", databaseNameEnv)
	}
	db, err := database.ConnectDB(cfg)
	if err != nil {
		t.Fatalf("Failed to open the test database: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("Failed to close test database: %v", err)
		}
	})
	return db
}
