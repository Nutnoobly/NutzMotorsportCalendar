package views_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/db"
	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/views"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestLayoutThemeAndNavbar(t *testing.T) {
	var buf bytes.Buffer
	meta := views.DefaultHomeMeta("")
	component := views.Layout(meta)
	err := component.Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}

	html := buf.String()

	// SEO verification
	if !strings.Contains(html, `<meta name="description" content="`+views.DefaultSiteDesc+`">`) {
		t.Errorf("expected meta description in head")
	}
	if !strings.Contains(html, `<meta name="robots" content="index, follow">`) {
		t.Errorf("expected robots meta tag")
	}
	if !strings.Contains(html, `<link rel="canonical" href="https://nutzmotorsportcalendar.onrender.com/">`) {
		t.Errorf("expected canonical link")
	}
	if !strings.Contains(html, `<meta property="og:site_name" content="Nutz Motorsport Calendar">`) {
		t.Errorf("expected og:site_name meta tag")
	}
	if !strings.Contains(html, `<meta name="twitter:card" content="summary_large_image">`) {
		t.Errorf("expected twitter:card meta tag")
	}
	if !strings.Contains(html, `application/ld+json`) || !strings.Contains(html, `"@type":"WebSite"`) {
		t.Errorf("expected JSON-LD structured data for WebSite")
	}

	// 1. Verify dark mode is default on html tag
	if !strings.Contains(html, `<html lang="en" class="h-full dark">`) {
		t.Errorf("expected <html ... class=\"h-full dark\">, got:\n%s", html)
	}

	// 2. Verify FOUC prevention script is in head
	if !strings.Contains(html, "localStorage.getItem('theme') === 'light'") {
		t.Errorf("expected theme check in head script")
	}

	// 3. Verify static/theme.js is loaded
	if !strings.Contains(html, `src="/static/theme.js"`) {
		t.Errorf("expected script tag for /static/theme.js")
	}

	// 4. Verify theme toggle button is present
	if !strings.Contains(html, `id="theme-toggle"`) {
		t.Errorf("expected #theme-toggle button in navbar")
	}
	if !strings.Contains(html, `onclick="toggleTheme()"`) {
		t.Errorf("expected onclick=\"toggleTheme()\" on button")
	}

	// 5. Verify Calendar button in navbar is removed
	if strings.Contains(html, `>Calendar</a>`) {
		t.Errorf("expected calendar link in navbar to be removed")
	}

	// 6. Verify mobile responsiveness constraints on header
	if !strings.Contains(html, "overflow-x-hidden") {
		t.Errorf("expected overflow-x-hidden on body to prevent horizontal scroll")
	}
	if !strings.Contains(html, "h-14 sm:h-16") {
		t.Errorf("expected compact mobile height h-14 on header")
	}
	if !strings.Contains(html, "truncate") {
		t.Errorf("expected truncate on timezone selector to avoid header blowup on phone")
	}
}

func TestEventDetailMeta(t *testing.T) {
	event := db.GetEventBySlugRow{
		EventID:     1,
		SerieID:     "f1",
		EventSeason: 2026,
		EventRound:  3,
		EventSlug:   "australian-grand-prix-2026",
		EventName:   "Australian Grand Prix",
		EventStartsAt: pgtype.Timestamptz{
			Time:  time.Date(2026, 3, 15, 4, 0, 0, 0, time.UTC),
			Valid: true,
		},
		EventStatus: "scheduled",
		CircuitName: "Albert Park Circuit",
		CircuitCountry: pgtype.Text{
			String: "Australia",
			Valid:  true,
		},
		CircuitLocality: pgtype.Text{
			String: "Melbourne",
			Valid:  true,
		},
		EventOfficialUrl: pgtype.Text{
			String: "https://www.formula1.com",
			Valid:  true,
		},
	}

	meta := views.EventDetailMeta(event)

	if !strings.Contains(meta.Title, "Australian Grand Prix (2026 Round 3)") {
		t.Errorf("expected title to include round and season, got %q", meta.Title)
	}
	if !strings.Contains(meta.Canonical, "/events/australian-grand-prix-2026") {
		t.Errorf("expected canonical URL with event slug, got %q", meta.Canonical)
	}
	if meta.OGType != "article" {
		t.Errorf("expected OGType article, got %q", meta.OGType)
	}

	jsonLD := string(meta.JSONLD)
	if !strings.Contains(jsonLD, `"@type":"SportsEvent"`) {
		t.Errorf("expected @type: SportsEvent in JSON-LD, got %s", jsonLD)
	}
	if !strings.Contains(jsonLD, `"name":"Australian Grand Prix"`) {
		t.Errorf("expected name in JSON-LD, got %s", jsonLD)
	}
	if !strings.Contains(jsonLD, `"sport":"Formula 1"`) {
		t.Errorf("expected sport: Formula 1 in JSON-LD, got %s", jsonLD)
	}
	if !strings.Contains(jsonLD, `"Albert Park Circuit"`) {
		t.Errorf("expected circuit name in JSON-LD, got %s", jsonLD)
	}
	if !strings.Contains(jsonLD, `"2026-03-15T04:00:00Z"`) {
		t.Errorf("expected RFC3339 startDate in JSON-LD, got %s", jsonLD)
	}
}

func TestDefaultHomeMetaSeriesFilters(t *testing.T) {
	f1Meta := views.DefaultHomeMeta("f1")
	if !strings.Contains(f1Meta.Title, "Formula 1 2026 Calendar") {
		t.Errorf("expected F1 in title, got %q", f1Meta.Title)
	}
	if !strings.Contains(f1Meta.Canonical, "?series=f1") {
		t.Errorf("expected ?series=f1 in canonical, got %q", f1Meta.Canonical)
	}

	motogpMeta := views.DefaultHomeMeta("motogp")
	if !strings.Contains(motogpMeta.Title, "MotoGP 2026 Calendar") {
		t.Errorf("expected MotoGP in title, got %q", motogpMeta.Title)
	}
	if !strings.Contains(motogpMeta.Canonical, "?series=motogp") {
		t.Errorf("expected ?series=motogp in canonical, got %q", motogpMeta.Canonical)
	}
}

