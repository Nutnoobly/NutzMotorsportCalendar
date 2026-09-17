package web

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/db"
	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/fetcher"
	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/views"
	"github.com/jackc/pgx/v5/pgtype"
)

// handleHome renders the homepage with race events from Supabase.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	seriesFilter := r.URL.Query().Get("series") // "f1", "motogp", or ""

	// Filter from 1 month ago onwards
	oneMonthAgo := time.Now().AddDate(0, -1, 0)
	startsAt := pgtype.Timestamptz{Time: oneMonthAgo, Valid: true}

	var events []db.ListUpcomingEventsRow
	var err error

	if seriesFilter == "f1" || seriesFilter == "motogp" {
		seriesEvents, qErr := s.queries.ListUpcomingEventsBySeries(r.Context(), db.ListUpcomingEventsBySeriesParams{
			SerieID:       seriesFilter,
			EventStartsAt: startsAt,
		})
		err = qErr
		// Map to common ListUpcomingEventsRow slice
		for _, e := range seriesEvents {
			events = append(events, db.ListUpcomingEventsRow(e))
		}
	} else {
		events, err = s.queries.ListUpcomingEvents(r.Context(), startsAt)
	}

	if err != nil {
		http.Error(w, "Failed to load events", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Header.Get("HX-Request") == "true" {
		if err := views.EventsSection(seriesFilter, events).Render(r.Context(), w); err != nil {
			http.Error(w, "Failed to render partial", http.StatusInternalServerError)
		}
		return
	}

	if err := views.Home(seriesFilter, events).Render(r.Context(), w); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}

// handleEventDetail renders the event detail page by slug.
func (s *Server) handleEventDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	event, err := s.queries.GetEventBySlug(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	sessions, _ := s.queries.ListSessionsByEventID(r.Context(), event.EventID)
	results, _ := s.queries.ListResultsByEventID(r.Context(), event.EventID)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := views.EventDetail(event, sessions, results).Render(r.Context(), w); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}

// handleICS provides .ics calendar exports for a specific series.
func (s *Server) handleICS(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.ics\"", slug))
	fmt.Fprintf(w, "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//NutzMotorsportCalendar//EN\nX-WR-CALNAME:%s Calendar\nEND:VCALENDAR\n", slug)
}

// handleRefresh triggers upstream API sync (protected by secret auth).
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("sync") == "true" {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Minute)
		defer cancel()

		if err := fetcher.SyncAll(ctx, s.queries); err != nil {
			log.Printf("Refresh error: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"status":"error","message":%q}`+"\n", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok","message":"Refresh completed successfully"}`)
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		if err := fetcher.SyncAll(ctx, s.queries); err != nil {
			log.Printf("Refresh error: %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintln(w, `{"status":"ok","message":"Refresh started in background"}`)
}
