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
| Render | Free cloud hosting for your containerized Go binary so the site is public. |

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
- [x] **4. Templ views [Frontend — Assistant builds / Contract-first]** — done (views authored, generated, wired to handlers, verified on port 8081).
- [x] **5. HTMX + countdown [Frontend — Assistant builds]** — done (FIA 5-light gantry, dynamic HTMX tab swapping, countdown ticker).
- [x] **6. Timezone JS [Frontend — Assistant builds]** — done (client-side auto-detection via Intl API, manual override, dynamic localized formatting on [data-utc]).
- [x] **7. Refresh fetchers [Backend]** — done (Jolpica F1, Pulselive MotoGP, session timetables, race & sprint podium results, SYNC_RUN logging, cmd/refresh CLI, /admin/refresh handler).
- [x] **8. Deploy [DevOps — Assistant builds]** — done (multi-stage Dockerfile, render.yaml, GitHub Actions daily cron, verified with container run).

---

## 7. Step 7 Guide: Data Fetchers & Upstream Synchronization (Jolpica + Pulselive)

In Step 7, you implement the data ingestion engine that pulls motorsport schedules and results from external APIs into your own Supabase database.

### 7.1 Architecture & The Mental Model

Your web application never calls external APIs during a user page request. Instead, synchronization happens out-of-band:

```
[ POST /admin/refresh or GitHub Actions Cron ]
                      │
                      ▼
             internal/fetcher
       ┌──────────────┴──────────────┐
       ▼                             ▼
  Jolpica API                   Pulselive API
 (Formula 1)                      (MotoGP)
       │                             │
       └──────────────┬──────────────┘
                      ▼
               internal/db (sqlc)
                      ▼
              Supabase PostgreSQL
```

#### Recommended Package Layout
Create an `internal/fetcher/` package:
- `internal/fetcher/f1.go`: Jolpica Ergast client (schedules, sessions, race results).
- `internal/fetcher/motogp.go`: Dorna Pulselive client (events, timetables, standings).
- `internal/fetcher/sync.go`: Orchestrator coordinating series syncs, logging to `SYNC_RUN`.

---

### 7.2 F1 Fetcher: Jolpica API (`internal/fetcher/f1.go`)

Jolpica provides a drop-in replacement for the Ergast F1 API.

- **Season Schedule**: `http://api.jolpica.net/ergast/f1/current.json`
- **Race Results**: `http://api.jolpica.net/ergast/f1/current/{round}/results.json`

#### Parsing F1 Schedule & Sessions
Each race in Jolpica returns date/time strings in UTC (`YYYY-MM-DD` and `HH:MM:SSZ`).

```go
type F1ScheduleResponse struct {
	MRData struct {
		RaceTable struct {
			Season string `json:"season"`
			Races  []struct {
				Round    string `json:"round"`
				RaceName string `json:"raceName"`
				Circuit  struct {
					CircuitID   string `json:"circuitId"`
					CircuitName string `json:"circuitName"`
					Location    struct {
						Locality string `json:"locality"`
						Country  string `json:"country"`
						Lat      string `json:"lat"`
						Long     string `json:"long"`
					} `json:"Location"`
				} `json:"Circuit"`
				Date          string `json:"date"`
				Time          string `json:"time"`
				FirstPractice *struct {
					Date string `json:"date"`
					Time string `json:"time"`
				} `json:"FirstPractice"`
				Qualifying *struct {
					Date string `json:"date"`
					Time string `json:"time"`
				} `json:"Qualifying"`
				Sprint *struct {
					Date string `json:"date"`
					Time string `json:"time"`
				} `json:"Sprint"`
			} `json:"Races"`
		} `json:"RaceTable"`
	} `json:"MRData"`
}
```

#### Target Database Queries
1. **Circuit**: `s.queries.UpsertCircuit(ctx, db.UpsertCircuitParams{...})`
2. **Event**: `s.queries.UpsertEvent(ctx, db.UpsertEventParams{...})`
3. **Sessions**: `s.queries.DeleteSessionsByEventID(ctx, eventID)` followed by `s.queries.InsertSession(ctx, db.InsertSessionParams{...})` for FP1, Quali, Sprint, and Main Race.

---

### 7.3 MotoGP Fetcher: Pulselive API (`internal/fetcher/motogp.go`)

Dorna's Pulselive API powers MotoGP.com.

- **Current Season Events**: `https://api.motogp.pulselive.com/motogp/v1/results/events?seasonYear=2026`
- **Session Results**: `https://api.motogp.pulselive.com/motogp/v1/results/session/{session_id}/classification`

#### Mandatory Header
Pulselive requires a custom `User-Agent` header; generic Go HTTP clients without one may receive `403 Forbidden` or connection resets:
```go
req.Header.Set("User-Agent", "NutzMotorsportCalendar/1.0")
```

#### Defensive JSON Handling
Pulselive often returns `null` or omitted fields for unconfirmed venues or future sessions. Use pointer fields (`*string`, `*int`) in your Go structs to prevent unmarshaling failures.

---

### 7.4 Sync Orchestration & Audit Logging (`internal/fetcher/sync.go`)

Track each sync operation in the `SYNC_RUN` table so failures can be audited:

```go
func SyncSeries(ctx context.Context, queries *db.Queries, serieID string, fetchFn func(context.Context) error) error {
	// 1. Record sync start
	syncRun, err := queries.CreateSyncRun(ctx, db.CreateSyncRunParams{
		SerieID: serieID,
		Message: pgtype.Text{String: "Sync started", Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to create sync run: %w", err)
	}

	// 2. Execute fetch & upsert
	syncErr := fetchFn(ctx)

	// 3. Record outcome
	statusMsg := "Success"
	if syncErr != nil {
		statusMsg = syncErr.Error()
	}

	finishErr := queries.FinishSyncRun(ctx, db.FinishSyncRunParams{
		SyncID:  syncRun.SyncID,
		Ok:      syncErr == nil,
		Message: pgtype.Text{String: statusMsg, Valid: true},
	})
	if finishErr != nil {
		log.Printf("Failed to finish sync run: %v", finishErr)
	}

	return syncErr
}
```

---

### 7.5 Wiring into `handleRefresh` (`internal/web/handler.go`)

Connect your new fetcher to the secret-protected admin route in `internal/web/handler.go`:

```go
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	// Launch sync in background goroutine so HTTP request doesn't timeout
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if err := fetcher.SyncAll(ctx, s.queries); err != nil {
			log.Printf("Refresh error: %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintln(w, `{"status":"ok","message":"Refresh started in background"}`)
}
```

---

### 7.6 Standalone Verification CLI (`cmd/refresh/main.go`)

To test sync logic without starting the full HTTP server, create a small CLI tool:

```bash
go run ./cmd/refresh/main.go
```

Or trigger via curl when the server is running:
```bash
curl -i -X POST \
  -H "Authorization: Bearer YOUR_ADMIN_SECRET" \
  http://localhost:8081/admin/refresh
```

Verify newly inserted events and sessions in your database using:
```bash
go run ./cmd/testdb/main.go
```

---

## 8. Gotchas Cheat Sheet

Quick list of things that will waste your time if you don't already know them:

- **`templ generate` before building** — Templ compiles `.templ` to `_templ.go`; forget it
  and the build breaks.
- **`pgtype.Text` in Templ** — Use `TextString(t, fallback)` rather than printing `pgtype.Text` directly.
- **pgx multi-statement** — use `QueryExecModeSimpleProtocol` for migrations.
- **`TIMESTAMPTZ`, store UTC** — timezone conversion happens in the browser via `static/timezone.js`, not on the server.
- **Pulselive User-Agent** — Always provide a custom `User-Agent` header for MotoGP requests.
- **`.ics` output** needs `Content-Type: text/calendar` and UTC `Z` timestamps.
- **Never commit `.env`** — it holds your DB password. `.gitignore` excludes it.
- **Archive window** — home page filters to `starts_at > now() - interval '1 month' OR starts_at > now()`.

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
