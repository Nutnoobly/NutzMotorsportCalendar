# Development Guide

This is your working reference for building NutzMotorsportCalendar — written for a first-time
mini-project. Read it top to bottom once, then jump to sections as you work. It explains the
*what* and the *why*, not just the how, so you can make your own decisions.

> ### Operating Rules: Dual-Role Split
> - **Backend (`.go`, SQL, DB)**: **You build, Assistant reviews.** You write the backend Go code, router, and queries for your learning and portfolio goals. The assistant reviews and mentors.
> - **Frontend & DevOps (`.templ`, HTMX, Tailwind, JS, Deploy)**: **Assistant builds.** The assistant authors all UI templates, styles, client scripts, and deployment configurations.
> - **Interface Protocol**: **Contract-First.** You define the Go handler signatures and view-model structs; the assistant writes the matching Templ components.

---

## 1. Big Picture

A website shows an F1 + MotoGP race calendar + results.

Data comes from external APIs. You **do not** fetch from those APIs on every page view (slow,
rate-limited, fragile). Instead:

1. A background job periodically pulls data from the upstream APIs **into your own database**.
2. Your website reads from **your database**, which you fully control.

```
Upstream API → [cron job] → Your Supabase DB → [Go server] → Visitor browser
```

This is a classic pattern: **own your data, don't proxy someone else's.**

---

## 2. Tech Stack — Why Each Piece

| Tool | Why you chose it |
|------|------------------|
| Go `net/http` | The backend engine. No framework needed — Go's standard library is enough. |
| Supabase (Postgres) | Your database, hosted for you. You write raw SQL. |
| `sqlc` + `pgx` | Write SQL once; sqlc generates type-safe Go so DB errors surface at compile time. |
| Templ | Type-safe HTML components compiled to Go. No messy string templates. |
| HTMX | Add interactivity (tabs, swapping) with a small HTML attribute, not heavy JS. |
| Tailwind CSS | Utility classes for styling, makes responsive design easy. |
| Fly.io | Hosts your Go binary so the site is public. |

---

## 3. Database Design (the core)

The complete schema specification, ERD, and UML class diagrams are located in [`DATABASE.md`](file:///home/nutnoobly/User/Code/Project/NutzMotorsportCalendar/DATABASE.md).

### 3.1 The eight tables

```
series       -- 'f1', 'motogp' lookup
circuits     -- normalized track venues (Monza, Silverstone, etc.)
teams        -- constructors per series (Ferrari, Red Bull, Ducati, etc.)
drivers      -- racers with permanent numbers, timing codes, and current team FK
events       -- Grand Prix weekends (season, round, circuit FK, status, slug)
sessions     -- full weekend timetable (FP1, Quali, Sprint, Main Race)
results      -- podium top-3 finishes (event FK, session_type, position 1-3, driver FK, team FK)
sync_runs    -- daily API ingestion audit log
```

See [`DATABASE.md`](file:///home/nutnoobly/User/Code/Project/NutzMotorsportCalendar/DATABASE.md) for full column definitions, data types, constraints, and Mermaid diagrams.

---


## 4. Migrations

A **migration** is a SQL file that changes your database schema over time. Your DB evolves:

```
0001_init.sql   → creates tables
0002_xxx.sql    → later change
```

**Why not just run SQL once?** Because as the project grows you make changes. Migrations are
a log of changes. Each is applied in order, exactly once.

**Key design**: run each migration inside a **transaction** (all-or-nothing). If a migration
fails halfway, the whole thing rolls back and your DB is unchanged — no half-applied state.

**Bookkeeping**: track which files already ran in a `schema_migrations` table. A migration
runner:

1. Reads all `*.sql` files sorted by name.
2. Checks `schema_migrations` for ones already applied → skips them.
3. Applies each new one in a transaction, records it.

**pgx gotcha that WILL bite you**: pgx by default uses the "extended query protocol", which
rejects a single string containing multiple statements. Migration files have many statements.
Fix: set `QueryExecModeSimpleProtocol` on the connection (or split statements yourself).

---

## 5. Data Sources

Two upstream APIs. You call these **only** from your refresh job, not from visitors' requests.

### 5.1 F1 — Jolpica (`api.jolpi.ca`)

- Endpoints: `/ergast/f1/current.json` (schedule) and `/ergast/f1/current/results.json` (results).
- Returns JSON nestings: each race has `date`, `time`, plus FirstPractice/SecondPractice/
  Qualifying/Sprint sub-objects → those map to your `sessions` table.
- Rate limit ~200 requests/hr unauthenticated — plenty for a daily cron.

### 5.2 MotoGP — Dorna Pulselive (`api.pulselive.motogp.com`)

- Less documented and messier. Set a **User-Agent** header (some APIs reject empty ones).
- **Defensive parsing is mandatory here**: fields can be `null` or missing. Never assume a
  field exists — check before using it.

### 5.3 Defensive programming rule

External APIs change without warning. Every piece of data you read should be treated as
"maybe absent, maybe the wrong type." When in doubt, log and skip, don't crash the whole refresh.

---

## 6. Build Order (do it in this order)

Each step is a small shippable chunk. Don't jump ahead — each builds on the previous.

- [x] **0. Toolchain** — done (Go, templ, sqlc, go.mod).
- [x] **1. Migrations** — done (schema written in `supabase/migrations/0001_init.sql` and applied in Supabase).
- [x] **2. sqlc queries & DB layer** — done (`sqlc.yaml`, queries in `db/queries/`, `internal/db/`, `pgxpool`, verified with `cmd/testdb`).
- [x] **3. HTTP routes [Backend — You build / Assistant reviews]** — done (`net/http` mux: home, event detail, `.ics`, refresh endpoint).
- [ ] **4. Templ views [Frontend — Assistant builds / Contract-first]** — layout, home, event pages; `templ generate` after each edit.
- [ ] **5. HTMX + countdown [Frontend — Assistant builds]** — filter tabs swap sections; countdown JS ticks each second.
- [ ] **6. Timezone JS [Frontend — Assistant builds]** — render UTC, convert to visitor's zone client-side.
- [ ] **7. Refresh fetchers [Backend — You build / Assistant reviews]** — Jolpica + Pulselive, upsert into DB.
- [ ] **8. Deploy [DevOps — Assistant builds]** — Dockerfile, Fly.io, GitHub Actions daily cron.

---

## 7. Step 4 Guide: Templ Views & Backend Handler Integration

In Step 4, we replace the plain-text responses in our HTTP handlers with type-safe HTML components built with **Templ** and styled with **Tailwind CSS**.

### 7.1 The Mental Model: What is Templ?

Templ is a component-based templating language for Go. Unlike standard `html/template` (which parses strings at runtime and can fail unexpectedly), Templ templates are **compiled directly into Go code**:

```
internal/views/*.templ ────( templ generate )────▶ internal/views/*_templ.go
```

Key benefits:
- **Compile-time safety**: Type errors in templates fail at build time, not in production.
- **Composable components**: Components render child content using `{ children... }`.
- **Zero runtime reflection**: Fast rendering directly to an `http.ResponseWriter`.

---

### 7.2 Authored View Components (`internal/views/`)

The following view components have been authored by your assistant and compiled:

1. **`layout.templ`**: Base layout containing the HTML5 boilerplate, Tailwind CDN, HTMX, navigation header with motorsport branding, and footer.
2. **`home.templ`**: Race weekend calendar grouped by month, series filter tabs (`All`, `F1`, `MotoGP`), `.ics` calendar sync buttons, and event cards with status badges.
3. **`event_detail.templ`**: Detailed Grand Prix view with circuit metadata, weekend timetable sessions, and official podium standings.
4. **`helpers.go`**: Formatters for `pgtype.Timestamptz`, `pgtype.Text`, status badge colors, and generated race summaries.

---

### 7.3 Next Action (Backend Integration): Wiring Handlers (`internal/web/handler.go`)

Per the **Contract-First** Dual-Role Split, you (the backend developer) connect the database queries to the Templ components in `internal/web/handler.go`.

Every Templ component implements `templ.Component`, providing a `.Render(ctx, w)` method.

#### 1. Wiring `handleHome`:
Replace the plain-text output in `handleHome` with:

```go
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
	if err := views.Home(seriesFilter, events).Render(r.Context(), w); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}
```

#### 2. Wiring `handleEventDetail`:
Replace the plain-text output in `handleEventDetail` with:

```go
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
```

---

### 7.4 Verifying Step 4

1. Start your local server:
   ```bash
   go run ./cmd/server/main.go
   ```

2. Test the web pages in your browser or with curl:
   - **Calendar Home**: Open `http://localhost:8080/` (or `curl -i http://localhost:8080/`)
   - **Filtered Calendar**: Open `http://localhost:8080/?series=f1` and `http://localhost:8080/?series=motogp`
   - **Event Detail**: Open `http://localhost:8080/events/<slug>` (e.g., `http://localhost:8080/events/monza-2026` or whatever slug is seeded in your DB)

3. Verify HTML rendering: Ensure HTML markup renders with Tailwind CSS and responsive layout.

---

### 7.5 Build Workflow

Whenever `.templ` files are modified:
```bash
templ generate
go build ./...
```

---

## 8. Gotchas Cheat Sheet

Quick list of things that will waste your time if you don't already know them:

- **`templ generate` before building** — Templ compiles `.templ` to `_templ.go`; forget it
  and the build breaks.
- **pgx multi-statement** — use `QueryExecModeSimpleProtocol` for migrations.
- **`TIMESTAMPTZ`, store UTC** — timezone conversion happens in the browser, not by hand.
- **`.ics` output** needs `Content-Type: text/calendar` and UTC `Z` timestamps.
- **Never commit `.env`** — it holds your DB password. `.gitignore` excludes it.
- **Archive window** — home page filters to `starts_at > now() - interval '1 month' OR starts_at > now()`.
- **sqlc picks up housekeeping tables too** — you may want to tell it to ignore `sync_runs`/`schema_migrations`.

---

## 9. Ideas to Add Later (after v1 works)

Don't build these now. Park them here.

- WEC series (needs a data source)
- Full results classification pages
- Driver/team standings
- User accounts (Supabase Auth is already available)
- Month-grid calendar view

---

## 10. If You're Stuck

1. Re-read the relevant section here.
2. Check the API docs for whatever's failing (Jolpica, Pulselive, Supabase, pgx, Templ).
3. Ask your assistant for a review — paste your code or the error. For backend Go code, your assistant acts as mentor/reviewer; for frontend (.templ, styles) and DevOps, your assistant implements directly.
4. Describe the problem out loud; often that surfaces the fix.
