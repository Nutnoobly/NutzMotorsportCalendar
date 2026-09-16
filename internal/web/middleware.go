package web

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// loggingMiddleware logs HTTP method, path, remote address, and duration.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s in %v", r.Method, r.URL.Path, time.Since(start))
	})
}

// adminAuthMiddleware guards the /admin/refresh endpoint using Bearer token authentication.
func (s *Server) adminAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.adminSecret == "" {
			http.Error(w, "Admin secret not configured", http.StatusInternalServerError)
			return
		}

		authHeader := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, prefix)
		if token != s.adminSecret {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}
