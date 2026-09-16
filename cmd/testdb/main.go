package main

import (
	"bufio"
	"os"
	"strings"
	"log"
	"fmt"
	"context"

	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/db"
)

func loadDotEnv(filepath string) {
	file, err := os.Open(filepath)
	if err != nil {
		log.Fatal(err)
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
			key := strings.TrimSpace(parts[0])
			value := strings.Trim(parts[1], "\"'")
			if os.Getenv(key) == "" {
				os.Setenv(key, value)
			}
		}
	}
	if err := scanner.Err(); err != nil {
    	log.Fatal(err)
	}
}

func main() {
	loadDotEnv(".env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL not set")
	}

	ctx := context.Background()

	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer pool.Close()

	fmt.Println("Successfully connected to Supabase PostgresSQL!")

	queries := db.New(pool)
	seriesList, err := queries.ListSeries(ctx)
	if err != nil {
		log.Fatalf("Failed to list series: %v", err)
	}
	fmt.Printf("Found %d series:\n", len(seriesList))
	for _, series := range seriesList {
		fmt.Printf(" - [%s] %s (slug: %s)\n", series.SerieID, series.SerieName, series.SerieSlug)
	}
}
