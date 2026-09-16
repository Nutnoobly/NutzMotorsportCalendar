package web

import (
	"net/http"

	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/db"
)

// Server holds dependencies required by HTTP handlers.
type Server struct {
	queries     *db.Queries
	adminSecret string
}

// NewServer initializes a new Server with required dependencies.
func NewServer(queries *db.Queries, adminSecret string) *Server {
	return &Server{
		queries:     queries,
		adminSecret: adminSecret,
	}
}

// Routes registers all application endpoints and middleware on a ServeMux.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Static assets (CSS, JS, images)
	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fileServer))

	// Application routes (Go 1.22+ method + pattern syntax)
	mux.HandleFunc("GET /", s.handleHome)
	mux.HandleFunc("GET /events/{slug}", s.handleEventDetail)
	mux.HandleFunc("GET /series/{slug}/calendar.ics", s.handleICS)
	mux.HandleFunc("POST /admin/refresh", s.adminAuthMiddleware(s.handleRefresh))

	// Wrap entire router with global logging middleware
	return loggingMiddleware(mux)
}
