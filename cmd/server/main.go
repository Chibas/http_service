package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/http_service/internal/config"
	"example.com/http_service/internal/database"
	"example.com/http_service/internal/middleware"
	"example.com/http_service/internal/task"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	config, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed loading server config: %v", err)
	}
	db, err := database.ConnectDB(config)
	if err != nil {
		log.Fatalf("DB startup failed: %v", err)
	}
	defer db.Close()

	router := middleware.LoggingMiddleware(
		middleware.RecoveryMiddleware(
			task.AppRouter(db),
		),
	)

	server := &http.Server{
		Addr:    ":" + config.HTTPPort,
		Handler: router,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errCh := make(chan error, 1)

	go func() {
		fmt.Printf("Server running on port %s\n", config.HTTPPort)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server failed: %v", err)
		}
		return
	}

	sdwnCtx, sdwnCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer sdwnCancel()
	err = server.Shutdown(sdwnCtx)

	if err != nil {
		log.Printf("Shutdown failed: %v", err)
	}
	log.Println("HTTP server stopped gracefully")
}
