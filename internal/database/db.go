package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"example.com/http_service/internal/config"
)

func ConnectDB(cfg config.Config) (*sql.DB, error) {
	config := fmt.Sprintf(
		"%s:%s@tcp(%s)/%s",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBName,
	)
	db, err := sql.Open("mysql", config)
	if err != nil {
		return nil, fmt.Errorf("DB failed to open: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = db.PingContext(ctx)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("Failed to ping a db: %w", err)
	}
	return db, nil
}
