package web_test

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/db"
	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/web"
)

func loadEnvFromRoot() {
	cwd, _ := os.Getwd()
	envPath := filepath.Join(cwd, "../../.env")
	file, err := os.Open(envPath)
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
	}
}

func TestSEOIntegrationLiveDB(t *testing.T) {
	loadEnvFromRoot()
	dbURL := strings.Trim(strings.TrimSpace(os.Getenv("DATABASE_URL")), "\"'")
	if dbURL == "" {
		t.Skip("Skipping live DB integration test: DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		t.Skipf("Skipping live DB integration test: could not connect to database: %v", err)
	}
	defer pool.Close()

	queries := db.New(pool)
	server := web.NewServer(queries, "test-secret")
	ts := httptest.NewServer(server.Routes())
	defer ts.Close()

	// 1. Check /robots.txt
	resp, err := http.Get(ts.URL + "/robots.txt")
	if err != nil {
		t.Fatalf("failed to GET /robots.txt: %v", err)
	}
	robotsBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for /robots.txt, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(robotsBody), "Sitemap:") {
		t.Errorf("expected Sitemap directive in /robots.txt")
	}

	// 2. Check /sitemap.xml
	resp, err = http.Get(ts.URL + "/sitemap.xml")
	if err != nil {
		t.Fatalf("failed to GET /sitemap.xml: %v", err)
	}
	sitemapBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for /sitemap.xml, got %d", resp.StatusCode)
	}
	sitemapText := string(sitemapBody)
	if !strings.Contains(sitemapText, "<urlset") {
		t.Errorf("expected <urlset> in /sitemap.xml")
	}

	// 3. Extract event slug and test /events/{slug}
	re := regexp.MustCompile(`/events/([a-zA-Z0-9_-]+)`)
	match := re.FindStringSubmatch(sitemapText)
	if len(match) > 1 {
		eventSlug := match[1]
		resp, err = http.Get(ts.URL + "/events/" + eventSlug)
		if err != nil {
			t.Fatalf("failed to GET /events/%s: %v", eventSlug, err)
		}
		detailBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 for /events/%s, got %d", eventSlug, resp.StatusCode)
		}
		detailHtml := string(detailBody)
		if !strings.Contains(detailHtml, "SportsEvent") {
			t.Errorf("expected SportsEvent Schema in event detail HTML")
		}
		if !strings.Contains(detailHtml, `<meta name="robots" content="index, follow">`) {
			t.Errorf("expected index, follow in event detail HTML")
		}
		if !strings.Contains(detailHtml, `<link rel="canonical" href="`) {
			t.Errorf("expected canonical link in event detail HTML")
		}
	}

	// 4. Verify HTMX partials update countdown widget per series
	fetchHTMX := func(url string) string {
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("HX-Request", "true")
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("failed HTMX request to %s: %v", url, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200 for %s, got %d", url, resp.StatusCode)
		}
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}

	// 4a. ALL partial
	allHTML := fetchHTMX(ts.URL + "/")
	if !strings.Contains(allHTML, `id="calendar-section"`) {
		t.Errorf("expected calendar-section in HTMX partial")
	}
	if !strings.Contains(allHTML, `id="telemetry-hud"`) {
		t.Errorf("expected telemetry-hud inside swapped calendar-section")
	}

	// 4b. F1 partial
	f1HTML := fetchHTMX(ts.URL + "/?series=f1")
	if !strings.Contains(f1HTML, `id="telemetry-hud"`) {
		t.Errorf("expected telemetry-hud inside F1 calendar-section")
	}
	if !strings.Contains(f1HTML, "bg-[#e10600]") {
		t.Errorf("expected active F1 tab style in partial")
	}

	// 4c. MotoGP partial
	motogpHTML := fetchHTMX(ts.URL + "/?series=motogp")
	if !strings.Contains(motogpHTML, `id="telemetry-hud"`) {
		t.Errorf("expected telemetry-hud inside MotoGP calendar-section")
	}
	if !strings.Contains(motogpHTML, "bg-[#0090d0]") {
		t.Errorf("expected active MotoGP tab style in partial")
	}
}
