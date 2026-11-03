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

	_ "github.com/go-sql-driver/mysql"
)

var DEFAULT_TIMEOUT = 2 * time.Second

const DEFAULT_PORT = "8000"

func main() {
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed loading server config: %v", err)
	}
	db, err := connectDB(config)
	if err != nil {
		log.Fatalf("DB startup failed: %v", err)
	}
	defer db.Close()

	router := appRouter(db)

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
