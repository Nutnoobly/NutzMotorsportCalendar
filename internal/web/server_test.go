package web_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/db"
	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/web"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestStaticThemeFile(t *testing.T) {
	// Locate repository root
	cwd, _ := os.Getwd()
	repoRoot := filepath.Join(cwd, "../..")
	if err := os.Chdir(repoRoot); err != nil {
		t.Skipf("skipping test; could not change directory to %s: %v", repoRoot, err)
	}
	defer os.Chdir(cwd)

	server := web.NewServer(nil, "secret")
	handler := server.Routes()

	req := httptest.NewRequest("GET", "/static/theme.js", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "window.toggleTheme") {
		t.Errorf("expected theme.js to contain window.toggleTheme")
	}
	if !strings.Contains(body, "localStorage.getItem('theme')") {
		t.Errorf("expected theme.js to check localStorage")
	}
}

func TestStaticOGBanner(t *testing.T) {
	cwd, _ := os.Getwd()
	repoRoot := filepath.Join(cwd, "../..")
	if err := os.Chdir(repoRoot); err != nil {
		t.Skipf("skipping test; could not change directory to %s: %v", repoRoot, err)
	}
	defer os.Chdir(cwd)

	server := web.NewServer(nil, "secret")
	handler := server.Routes()

	req := httptest.NewRequest("GET", "/static/og-banner.png", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "image/png") {
		t.Errorf("expected image/png content type, got %s", contentType)
	}
	if rr.Body.Len() == 0 {
		t.Errorf("expected non-empty image content")
	}
}


func TestRobotsTxt(t *testing.T) {
	server := web.NewServer(nil, "secret")
	handler := server.Routes()

	req := httptest.NewRequest("GET", "/robots.txt", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("expected text/plain content type, got %s", contentType)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "User-agent: *") {
		t.Errorf("expected User-agent: * in robots.txt")
	}
	if !strings.Contains(body, "Allow: /") {
		t.Errorf("expected Allow: / in robots.txt")
	}
	if !strings.Contains(body, "Disallow: /admin/") {
		t.Errorf("expected Disallow: /admin/ in robots.txt")
	}
	if !strings.Contains(body, "Sitemap: https://nutzmotorsportcalendar.onrender.com/sitemap.xml") {
		t.Errorf("expected default sitemap url in robots.txt, got:\n%s", body)
	}
}

func TestRobotsTxtWithCustomBaseURL(t *testing.T) {
	orig := os.Getenv("BASE_URL")
	defer os.Setenv("BASE_URL", orig)
	os.Setenv("BASE_URL", "https://motorsport.example.com")

	server := web.NewServer(nil, "secret")
	handler := server.Routes()

	req := httptest.NewRequest("GET", "/robots.txt", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "Sitemap: https://motorsport.example.com/sitemap.xml") {
		t.Errorf("expected custom sitemap url in robots.txt, got:\n%s", body)
	}
}

func TestSitemapXml(t *testing.T) {
	server := web.NewServer(nil, "secret")
	handler := server.Routes()

	req := httptest.NewRequest("GET", "/sitemap.xml", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/xml") {
		t.Errorf("expected application/xml content type, got %s", contentType)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Errorf("expected XML declaration in sitemap")
	}
	if !strings.Contains(body, `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`) {
		t.Errorf("expected urlset root element in sitemap")
	}
	if !strings.Contains(body, "<loc>https://nutzmotorsportcalendar.onrender.com/</loc>") {
		t.Errorf("expected home URL in sitemap")
	}
	if !strings.Contains(body, "<loc>https://nutzmotorsportcalendar.onrender.com/?series=f1</loc>") {
		t.Errorf("expected F1 URL in sitemap")
	}
	if !strings.Contains(body, "<loc>https://nutzmotorsportcalendar.onrender.com/?series=motogp</loc>") {
		t.Errorf("expected MotoGP URL in sitemap")
	}
}

func TestShouldTriggerEventRefresh(t *testing.T) {
	now := time.Now()

	t.Run("future event far away returns false", func(t *testing.T) {
		event := db.GetEventBySlugRow{
			EventSlug:     "future-gp",
			EventStatus:   "scheduled",
			EventStartsAt: pgtype.Timestamptz{Time: now.Add(24 * time.Hour), Valid: true},
		}
		if web.ShouldTriggerEventRefresh(event, nil) {
			t.Errorf("expected false for distant future event")
		}
	})

	t.Run("completed event with results returns false", func(t *testing.T) {
		event := db.GetEventBySlugRow{
			EventSlug:     "completed-gp",
			EventStatus:   "completed",
			EventStartsAt: pgtype.Timestamptz{Time: now.Add(-2 * time.Hour), Valid: true},
		}
		results := []db.ListResultsByEventIDRow{
			{SessionType: "race", ResultPosition: 1},
		}
		if web.ShouldTriggerEventRefresh(event, results) {
			t.Errorf("expected false for completed event with results")
		}
	})

	t.Run("active event triggers refresh and debounces subsequent calls", func(t *testing.T) {
		event := db.GetEventBySlugRow{
			EventSlug:     "live-gp-" + t.Name(),
			EventStatus:   "scheduled",
			EventStartsAt: pgtype.Timestamptz{Time: now.Add(-1 * time.Hour), Valid: true},
		}
		// First call should trigger
		if !web.ShouldTriggerEventRefresh(event, nil) {
			t.Errorf("expected true for event that started 1 hour ago without results")
		}
		// Immediate second call should debounce (return false)
		if web.ShouldTriggerEventRefresh(event, nil) {
			t.Errorf("expected false due to debounce within 5 minutes")
		}
	})
}



