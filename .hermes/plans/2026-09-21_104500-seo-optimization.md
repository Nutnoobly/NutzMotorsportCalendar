# SEO Optimization Plan for NutzMotorsportCalendar

## Goal
Implement end-to-end technical and on-page SEO for NutzMotorsportCalendar to ensure search engines (Google, Bing) and social platforms (OpenGraph, Twitter/X, Discord) discover, index, and properly display rich snippets for race calendars, weekend session schedules, and Grand Prix results.

---

## Current Context & Assumptions
- **Stack**: Go 1.22+ `net/http` router, Templ template engine (`internal/views/`), Tailwind CSS, HTMX, Supabase (PostgreSQL accessed via `sqlc`).
- **Active Domain**: `https://nutzmotorsportcalendar.onrender.com` (configured on Render free tier and GitHub Actions cron).
- **Current Gaps**:
  - `internal/views/layout.templ` only defines basic `<title>` and `<meta name="viewport">`. It lacks `<meta name="description">`, `<link rel="canonical">`, OpenGraph (`og:*`), and Twitter card (`twitter:*`) tags.
  - No `robots.txt` exists to guide search crawlers or block private endpoints like `/admin/`.
  - No dynamic `sitemap.xml` exists to index individual race event URLs (`/events/{slug}`).
  - No Schema.org structured data (`SportsEvent` JSON-LD) exists to qualify for Google rich cards/events carousel.
- **Dual-Role Contract**: Frontend templates (`.templ`) and deployment configs are authored directly by the assistant; backend Go route additions follow contract-first design.

---

## Proposed Architecture & Approach
1. **Structured Metadata Model (`SEOMeta`)**: Define a typed metadata struct in `internal/views/helpers.go` containing title, description, canonical URL, OG image, OG type, and raw JSON-LD payload. Provide constructors for both default home and dynamic event pages.
2. **Enhanced Master Layout**: Refactor `internal/views/layout.templ` to accept `SEOMeta`, injecting high-priority tags into `<head>`:
   - `<meta name="description">`
   - `<link rel="canonical">`
   - OpenGraph protocol tags (`og:site_name`, `og:type`, `og:title`, `og:description`, `og:url`, `og:image`)
   - Twitter card tags (`twitter:card`, `twitter:title`, `twitter:description`, `twitter:image`)
   - Embedded `<script type="application/ld+json">` for schema markup.
3. **Schema.org `SportsEvent` Markup**: On `internal/views/event_detail.templ`, render JSON-LD declaring the Grand Prix name, sport (Formula 1 or MotoGP), start/end ISO timestamps, circuit venue (`Place` with address/country), and organizer.
4. **Crawler Directives (`robots.txt`)**: Serve a standard `robots.txt` via Go `net/http` allowing public routes, disallowing `/admin/`, and referencing `sitemap.xml`.
5. **Dynamic Sitemap Generation (`sitemap.xml`)**: Add a Go handler in `internal/web/handler.go` that queries all events from the database and streams a valid XML `<urlset>` containing `/`, `/?series=f1`, `/?series=motogp`, and every `/events/{slug}`.

---

## Step-by-Step Implementation Tasks

### Task 1: Create `SEOMeta` Struct & Helper Functions
**File**: `internal/views/helpers.go`
Define the metadata container and generator functions for home and event detail pages.

```go
package views

import (
	"encoding/json"
	"fmt"
	"html/template"
	"time"

	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/db"
)

const (
	SiteName        = "Nutz Motorsport Calendar"
	DefaultSiteDesc = "Live countdowns, weekend session timetables, and race results for Formula 1 and MotoGP fans."
	BaseURL         = "https://nutzmotorsportcalendar.onrender.com"
	DefaultOGImage  = BaseURL + "/static/og-banner.png"
)

// SEOMeta encapsulates page-level metadata, social tags, and structured data.
type SEOMeta struct {
	Title       string
	Description string
	Canonical   string
	OGImage     string
	OGType      string
	JSONLD      template.JS
}

// DefaultHomeMeta generates SEO metadata for the homepage and series filters.
func DefaultHomeMeta(seriesFilter string) SEOMeta {
	title := "F1 & MotoGP Race Calendar & Countdown"
	desc := DefaultSiteDesc
	canonical := BaseURL + "/"

	if seriesFilter == "f1" {
		title = "Formula 1 2026 Calendar & Race Schedules"
		desc = "Complete 2026 Formula 1 race calendar, weekend session timetables, and live countdowns."
		canonical = BaseURL + "/?series=f1"
	} else if seriesFilter == "motogp" {
		title = "MotoGP 2026 Calendar & Race Schedules"
		desc = "Complete 2026 MotoGP Grand Prix calendar, sprint & race schedules, and live countdowns."
		canonical = BaseURL + "/?series=motogp"
	}

	return SEOMeta{
		Title:       title,
		Description: desc,
		Canonical:   canonical,
		OGImage:     DefaultOGImage,
		OGType:      "website",
	}
}

// EventDetailMeta builds dynamic SEO tags and Schema.org SportsEvent JSON-LD for an event.
func EventDetailMeta(event db.GetEventBySlugRow) SEOMeta {
	title := fmt.Sprintf("%s (%d Round %d)", event.EventName, event.Season, event.Round)
	desc := fmt.Sprintf("Schedule, session timetables, circuit info, and results for the %s at %s.", event.EventName, event.CircuitName)
	canonical := fmt.Sprintf("%s/events/%s", BaseURL, event.EventSlug)

	schema := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "SportsEvent",
		"name":        event.EventName,
		"sport":       event.SerieName,
		"startDate":   event.StartDate.Time.Format(time.RFC3339),
		"endDate":     event.EndDate.Time.Format(time.RFC3339),
		"eventStatus": "https://schema.org/EventScheduled",
		"location": map[string]any{
			"@type": "Place",
			"name":  event.CircuitName,
			"address": map[string]any{
				"@type":           "PostalAddress",
				"addressCountry":  event.Country,
				"addressLocality": event.City,
			},
		},
		"organizer": map[string]any{
			"@type": "SportsOrganization",
			"name":  event.SerieName,
			"url":   event.OfficialUrl,
		},
	}

	jsonBytes, _ := json.Marshal(schema)

	return SEOMeta{
		Title:       title,
		Description: desc,
		Canonical:   canonical,
		OGImage:     DefaultOGImage,
		OGType:      "article",
		JSONLD:      template.JS(jsonBytes),
	}
}
```

---

### Task 2: Refactor `internal/views/layout.templ` to Render SEO Tags
**File**: `internal/views/layout.templ`
Update `Layout(meta SEOMeta)` to output complete `<head>` tags and conditional JSON-LD.

```templ
package views

templ Layout(meta SEOMeta) {
	<!DOCTYPE html>
	<html lang="en" class="h-full dark">
		<head>
			<meta charset="UTF-8"/>
			<meta name="viewport" content="width=device-width, initial-scale=1.0"/>
			<title>{ meta.Title } | Nutz Motorsport Calendar</title>
			<meta name="description" content={ meta.Description }/>
			<meta name="robots" content="index, follow"/>
			<link rel="canonical" href={ meta.Canonical }/>

			<!-- OpenGraph Tags -->
			<meta property="og:site_name" content="Nutz Motorsport Calendar"/>
			<meta property="og:type" content={ meta.OGType }/>
			<meta property="og:title" content={ meta.Title + " | Nutz Motorsport Calendar" }/>
			<meta property="og:description" content={ meta.Description }/>
			<meta property="og:url" content={ meta.Canonical }/>
			<meta property="og:image" content={ meta.OGImage }/>

			<!-- Twitter Card Tags -->
			<meta name="twitter:card" content="summary_large_image"/>
			<meta name="twitter:title" content={ meta.Title + " | Nutz Motorsport Calendar" }/>
			<meta name="twitter:description" content={ meta.Description }/>
			<meta name="twitter:image" content={ meta.OGImage }/>

			if meta.JSONLD != "" {
				<script type="application/ld+json">
					@templ.Raw(string(meta.JSONLD))
				</script>
			}

			<script>
				if (localStorage.getItem('theme') === 'light') {
					document.documentElement.classList.remove('dark');
				} else {
					document.documentElement.classList.add('dark');
				}
			</script>
			<script src="https://cdn.tailwindcss.com"></script>
			<script src="https://unpkg.com/htmx.org@2.0.4"></script>
			<link rel="stylesheet" href="/static/style.css"/>
			<script src="/static/theme.js"></script>
			<script src="/static/countdown.js" defer></script>
			<script src="/static/timezone.js" defer></script>
...
```

---

### Task 3: Connect Metadata in `home.templ` & `event_detail.templ`
- **`internal/views/home.templ`**: Swap `@Layout("Home")` to `@Layout(DefaultHomeMeta(selectedSeries))`.
- **`internal/views/event_detail.templ`**: Swap `@Layout(event.EventName)` to `@Layout(EventDetailMeta(event))`.

---

### Task 4: Add `robots.txt` Route
**File**: `internal/web/server.go`
Register the standard `robots.txt` route in `Routes()`:

```go
mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "User-agent: *")
	fmt.Fprintln(w, "Allow: /")
	fmt.Fprintln(w, "Disallow: /admin/")
	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "Sitemap: %s/sitemap.xml\n", views.BaseURL)
})
```

---

### Task 5: Add Dynamic `sitemap.xml` Route
**File**: `internal/web/server.go` & `internal/web/handler.go`
Register `GET /sitemap.xml` and implement `handleSitemap`:

```go
// handleSitemap dynamically generates a valid XML sitemap of all active events.
func (s *Server) handleSitemap(w http.ResponseWriter, r *http.Request) {
	// Fetch events to enumerate all slugs
	events, err := s.queries.ListUpcomingEvents(r.Context(), pgtype.Timestamptz{Time: time.Time{}, Valid: true})
	if err != nil {
		http.Error(w, "Failed to generate sitemap", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintln(w, `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`)
	
	// Core static landing paths
	fmt.Fprintf(w, "  <url><loc>%s/</loc><changefreq>daily</changefreq><priority>1.0</priority></url>\n", views.BaseURL)
	fmt.Fprintf(w, "  <url><loc>%s/?series=f1</loc><changefreq>daily</changefreq><priority>0.9</priority></url>\n", views.BaseURL)
	fmt.Fprintf(w, "  <url><loc>%s/?series=motogp</loc><changefreq>daily</changefreq><priority>0.9</priority></url>\n", views.BaseURL)

	// Dynamic event pages
	for _, e := range events {
		fmt.Fprintf(w, "  <url><loc>%s/events/%s</loc><changefreq>weekly</changefreq><priority>0.8</priority></url>\n", views.BaseURL, e.EventSlug)
	}

	fmt.Fprintln(w, `</urlset>`)
}
```

---

### Task 6: Update Tests & Recompile Views
1. **`internal/views/layout_test.go`**: Update the test invocation from `views.Layout("Test Page")` to `views.Layout(views.DefaultHomeMeta(""))`. Verify that `<meta name="description">` and `<link rel="canonical">` render in the output buffer.
2. **Regenerate Templ files**: Run `templ generate`.
3. **Verify compilation**: Run `go test ./internal/views/...` and `go build ./... && go vet ./...`.

---

## Verification & Testing Plan

### 1. Unit Tests
```bash
# Run views tests with SEO metadata checks
go test -v ./internal/views/...
```
*Expected*: PASS for `TestLayoutThemeAndNavbar` with SEO tags asserted.

### 2. Manual Verification
Start local server on port 8081 (`go run ./cmd/server`):
```bash
# Check robots.txt
curl -s http://localhost:8081/robots.txt
# Expected: Disallow: /admin/, Sitemap: https://nutzmotorsportcalendar.onrender.com/sitemap.xml

# Check sitemap.xml
curl -s http://localhost:8081/sitemap.xml
# Expected: Valid XML with <urlset>, <loc> for /, /?series=f1, /?series=motogp, and /events/...

# Check OpenGraph and JSON-LD on event page
curl -s http://localhost:8081/events/bahrain-grand-prix-2026 | grep -E "og:|twitter:|application/ld\+json"
```

---

## Risks & Tradeoffs
- **Static Asset for OG Image**: Until an automated OG image generator (e.g., via headless Chrome or SVG rendering) is added, a static high-res motorsport banner (`/static/og-banner.png`) serves as the default fallback.
- **Sitemap Caching**: For high-traffic production, `sitemap.xml` can be cached in-memory with a 1-hour TTL rather than executing a DB query on every crawler hit.
