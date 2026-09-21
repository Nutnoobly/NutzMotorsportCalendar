package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/db"
	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/views"
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

	tables := []string{"series", "circuits", "teams", "drivers", "events", "sessions", "results", "sync_run"}
	fmt.Println("\nTable Row Counts:")
	for _, t := range tables {
		var cnt int
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM "+t).Scan(&cnt)
		fmt.Printf(" - %-10s: %d\n", t, cnt)
	}

	rows, err := pool.Query(ctx, "SELECT sync_id, serie_id, ok, message, started_at, finished_at FROM sync_run ORDER BY sync_id DESC LIMIT 5")
	if err == nil {
		fmt.Println("\nRecent Sync Runs:")
		for rows.Next() {
			var id int
			var sid, msg string
			var ok bool
			var st, ft any
			_ = rows.Scan(&id, &sid, &ok, &msg, &st, &ft)
			fmt.Printf(" - Run #%d [%s] ok=%v msg=%s\n", id, sid, ok, msg)
		}
		rows.Close()
	}

	eventRows, err := pool.Query(ctx, "SELECT event_id, serie_id, event_season, event_round, event_name, event_status FROM events ORDER BY event_starts_at ASC LIMIT 6")
	if err == nil {
		fmt.Println("\nSample Events:")
		for eventRows.Next() {
			var id, round, season int
			var sid, name, status string
			_ = eventRows.Scan(&id, &sid, &season, &round, &name, &status)
			fmt.Printf(" - #%d [%s] %s (S%d R%d, status: %s)\n", id, sid, name, season, round, status)
		}
		eventRows.Close()
	}

	resultRows, err := pool.Query(ctx, `
		SELECT r.event_id, r.session_type, r.result_position, d.driver_first_name, d.driver_last_name, t.team_name, r.result_time_or_gap, r.result_points
		FROM results r
		JOIN drivers d ON r.driver_id = d.driver_id
		JOIN teams t ON r.team_id = t.team_id
		ORDER BY r.event_id, r.session_type, r.result_position
		LIMIT 6
	`)
	if err == nil {
		fmt.Println("\nSample Results (Podium Top-3):")
		for resultRows.Next() {
			var eid, pos int
			var stype, fname, lname, tname string
			var timeGap any
			var pts float64
			_ = resultRows.Scan(&eid, &stype, &pos, &fname, &lname, &tname, &timeGap, &pts)
			fmt.Printf(" - Event %d [%s] P%d: %s %s (%s) - Gap/Time: %v, Points: %.1f\n", eid, stype, pos, fname, lname, tname, timeGap, pts)
		}
		resultRows.Close()
	}

	fmt.Println("\nF1 Official Race Summaries Verification:")
	f1Events, err := pool.Query(ctx, "SELECT event_id, event_round, event_name, event_slug FROM events WHERE serie_id = 'f1' AND event_status = 'completed' ORDER BY event_round ASC")
	if err == nil {
		for f1Events.Next() {
			var eid, round int
			var ename, eslug string
			_ = f1Events.Scan(&eid, &round, &ename, &eslug)
			results, _ := queries.ListResultsByEventID(ctx, int32(eid))
			races := views.FilterResultsBySession(results, "race")
			sprints := views.FilterResultsBySession(results, "sprint")
			summary := views.GenerateRaceSummary(results)
			fmt.Printf(" [R%02d] %s (%s):\n    Results: %d (Race: %d, Sprint: %d) | Summary: %s\n", round, ename, eslug, len(results), len(races), len(sprints), summary)
		}
		f1Events.Close()
	}
}
