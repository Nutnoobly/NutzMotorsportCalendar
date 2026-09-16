package main

import (
	"bufio"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Nutnoobly/NutzMotosportCalendar/internal/db"
	"github.com/Nutnoobly/NutzMotosportCalendar/internal/web"
)

// loadDotEnv loads simple key=val environment variables from .env if present.
func loadDotEnv(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			if os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
		if err := scanner.Err(); err != nil {
			log.Fatalf("Error reading .env file: %v", err)
		}
	}
}

func main() {
	loadDotEnv(".env")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	adminSecret := os.Getenv("ADMIN_SECRET")

	// 1. Root context cancelled on SIGINT (Ctrl+C) or SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 2. Connect to Supabase via Session Pooler
	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to initialize database pool: %v", err)
	}
	defer pool.Close()

	// 3. Initialize queries and web server
	queries := db.New(pool)
	webServer := web.NewServer(queries, adminSecret)

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      webServer.Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 4. Start server in a background goroutine
	go func() {
		log.Printf("Server listening on http://localhost:%s", port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// 5. Wait for shutdown signal (Ctrl+C)
	<-ctx.Done()
	log.Println("Shutting down server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Forced shutdown error: %v", err)
	}
	log.Println("Server exited cleanly.")
}
