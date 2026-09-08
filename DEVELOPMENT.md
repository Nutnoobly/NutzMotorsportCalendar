# Development Guide

This is your working reference for building NutzMotosportCalendar — written for a first-time
mini-project. Read it top to bottom once, then jump to sections as you work. It explains the
*what* and the *why*, not just the how, so you can make your own decisions.

> One rule that guides everything: **you do the work.** This guide tells you what to build and
> why, but writing the code is yours. If you get stuck, that's normal — it means you're learning.

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

The complete schema specification, ERD, and UML class diagrams are located in [`DATABASE.md`](file:///home/nutnoobly/User/Code/Project/NutzMotosportCalendar/DATABASE.md).

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

See [`DATABASE.md`](file:///home/nutnoobly/User/Code/Project/NutzMotosportCalendar/DATABASE.md) for full column definitions, data types, constraints, and Mermaid diagrams.

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
- [x] **2. sqlc queries & DB layer** — configure `sqlc.yaml`, write queries, run `sqlc generate`, wire `pgxpool`, verify with `testdb`.
- [ ] **3. HTTP routes** — `net/http` mux: home, event detail, `.ics`, refresh endpoint.
- [ ] **4. Templ views** — layout, home, event pages; `templ generate` after each edit.
- [ ] **5. HTMX + countdown** — filter tabs swap sections; countdown JS ticks each second.
- [ ] **6. Timezone JS** — render UTC, convert to visitor's zone client-side.
- [ ] **7. Refresh fetchers** — Jolpica + Pulselive, upsert into DB.
- [ ] **8. Deploy** — Dockerfile, Fly.io, GitHub Actions daily cron.

---

## 7. Step 2 Guide: Database Access Layer (sqlc + pgx)

This section is your hands-on learning guide for Step 2. You will write real Go code, learn Go idioms, and understand how type-safe database access works.

### 7.1 The Mental Model: Why sqlc + pgx?

In languages like JavaScript (Prisma/TypeORM) or Python (SQLAlchemy/Django ORM), developers often use ORMs that generate SQL on the fly. In production Go, ORMs are often avoided:
- **ORMs hide queries**: You don't know what SQL is actually running, making performance optimization difficult.
- **`sqlc` is SQL-first**: You write standard PostgreSQL queries. `sqlc` parses your schema and queries at compile time, and generates native, type-safe Go structs and functions.
- **`pgx/v5`**: The standard high-performance PostgreSQL driver for Go.

```
[SQL schema] + [SQL queries]
            │
            ▼ (sqlc generate)
[Generated Go structs & methods] (internal/db/)
            │
            ▼ (called by your Go code with pgxpool)
[Supabase PostgreSQL Database]
```

### 7.2 The Configuration: `sqlc.yaml`

In your project root, `sqlc.yaml` tells `sqlc` where your database schema and query files live, and what Go code to output:

```yaml
version: "2"
sql:
  - schema: "supabase/migrations"
    queries: "db/queries"
    gen:
      go:
        package: "db"
        out: "internal/db"
        sql_package: "pgx/v5"
```

- `schema`: Points to your DDL directory (`supabase/migrations`). `sqlc` reads all `.sql` migration files in alphabetical order (`0001_...`, `0002_...`) so you don't need to change `sqlc.yaml` for new migrations.
- `queries`: Directory containing your raw SQL files (`.sql`).
- `package: "db"`: All generated Go files will start with `package db`.
- `out: "internal/db"`: Directory where generated Go code will be placed. In Go, code in `internal/` is private to this module and cannot be imported by external packages.
- `sql_package: "pgx/v5"`: Tells `sqlc` to generate code using `github.com/jackc/pgx/v5`.

### 7.3 Writing Queries: `db/queries/`

`sqlc` uses special comments above queries to know what Go function signature to generate:

| sqlc Annotation | Go Return Type | When to use |
|---|---|---|
| `-- name: GetItem :one` | `(Item, error)` | Exactly one row expected (e.g. `WHERE id = $1`). Returns `pgx.ErrNoRows` if not found. |
| `-- name: ListItems :many` | `([]Item, error)` | Zero or more rows expected (e.g. `SELECT * FROM items`). |
| `-- name: DeleteItem :exec` | `error` | No rows returned (e.g. `DELETE` or `UPDATE`). |
| `-- name: DeleteItem :execrows` | `(int64, error)` | Returns number of affected rows. |

#### Query Files Overview

1. **`db/queries/series.sql`**: Lookup queries for championship series:
   ```sql
   -- name: ListSeries :many
   SELECT series_id, series_name, series_slug
   FROM series
   ORDER BY series_id;

   -- name: GetSeriesBySlug :one
   SELECT series_id, series_name, series_slug
   FROM series
   WHERE series_slug = $1;
   ```

2. **`db/queries/events.sql`**: Calendar and weekend details:
   - `ListEventsBySeason`: Gets calendar events for a series and year.
   - `GetUpcomingEvents`: Finds next races for countdown and archive window.
   - `GetEventBySlug`: Detailed Grand Prix page.
   - `ListSessionsByEvent`: Timetable for FP1, Quali, Sprint, Race.
   - `UpsertEvent`: Ingestion query to insert or update event details.

3. **`db/queries/results.sql`**: Podium finishes:
   - `GetPodiumByEventAndSession`: Returns positions 1–3 joined with driver name/code and team name/color.
   - `UpsertResult`: Ingestion query with strict top-3 position constraint.

4. **`db/queries/sync.sql`**: Entities and audit logging:
   - `UpsertCircuit`, `UpsertTeam`, `UpsertDriver`: Foreign key parents.
   - `CreateSyncRun`, `CompleteSyncRun`: Refresh job logging.

### 7.4 Running `sqlc generate`

Once your `sqlc.yaml` and query files in `db/queries/` are ready, run:

```bash
sqlc generate
```

`sqlc` parses your schema and queries and generates the following inside `internal/db/`:
- **`models.go`**: Go structs matching your database tables (`Series`, `Circuit`, `Team`, `Driver`, `Event`, `Session`, `Result`, `SyncRun`).
- **`db.go`**: Contains the `DBTX` interface and `Queries` struct constructor `New(db DBTX) *Queries`.
- **`*.sql.go`**: Typed Go methods (e.g., `queries.ListEventsBySeason(ctx, arg)`).

### 7.5 Installing `pgx/v5` Dependency

Add the PostgreSQL driver to your Go module:

```bash
go get github.com/jackc/pgx/v5
go mod tidy
```

This updates `go.mod` and generates `go.sum` with verified checksums.

### 7.6 Writing the Connection Pool Helper: `internal/db/conn.go`

In a web application, opening a new database connection for each request is inefficient (handshake overhead). Instead, we use a **connection pool** (`pgxpool.Pool`) which manages a set of reusable connections.

Create `internal/db/conn.go`:

```go
package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool initializes and tests a PostgreSQL connection pool.
func NewPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection immediately with a ping
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}
```

#### Go Concepts Explained in `conn.go`:
- **`context.Context`**: Go's standard mechanism for deadlines, timeouts, and cancellation. Every database call in Go accepts a context.
- **Error Wrapping (`%w`)**: `fmt.Errorf("...: %w", err)` wraps the underlying error, preserving error inspection via `errors.Is()` or `errors.As()`.
- **Pointers (`*pgxpool.Pool`)**: The pool is a shared, concurrent-safe reference. Passing a pointer avoids copying the internal state.

### 7.7 Supabase Connection Gotcha (IPv4 vs IPv6 & Session Mode)

> [!WARNING]
> **Supabase Direct Connection (`:5432` on `db.<ref>.supabase.co`) resolves to IPv6 only.**
> If your local network or ISP does not have active IPv6 routing, connecting directly will result in `dial tcp ... network is unreachable` or a timeout.
>
> **Solution**: Use Supabase's **Connection Pooler** (`aws-0-<region>.pooler.supabase.com`), which supports **IPv4**!
>
> **Important - Choose Session Mode (`:5432`)**:
> - **Session Mode (Port 5432)**: **Recommended for Go / `pgx`**. Behaves like a direct PostgreSQL connection and supports prepared statements out-of-the-box.
> - **Transaction Mode (Port 6543)**: Intended for stateless serverless environments (e.g. AWS Lambda). Does not support prepared statements without additional `pgx` configuration (`default_query_exec_mode=exec`).
>
> Find your connection string in Supabase Dashboard: **Project Settings -> Database -> Connection string -> URI**, then toggle **Mode: Session** (port 5432).

### 7.8 Testing Your DB Layer: `cmd/testdb/main.go`

Create a small CLI program to verify your database connection and test a query:

```go
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Nutnoobly/NutzMotosportCalendar/internal/db"
)

// loadDotEnv reads key=value lines from a local .env file
func loadDotEnv(filepath string) {
	file, err := os.Open(filepath)
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

func main() {
	loadDotEnv(".env")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	ctx := context.Background()

	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer pool.Close()

	fmt.Println("Successfully connected to Supabase PostgreSQL!")

	queries := db.New(pool)
	seriesList, err := queries.ListSeries(ctx)
	if err != nil {
		log.Fatalf("Failed to query series: %v", err)
	}

	fmt.Printf("Found %d series in database:\n", len(seriesList))
	for _, s := range seriesList {
		fmt.Printf(" - [%s] %s (slug: %s)\n", s.SeriesID, s.SeriesName, s.SeriesSlug)
	}
}
```

Run the test:
```bash
go run ./cmd/testdb/main.go
```

If you see your series listed (or `Found 0 series in database` if not yet seeded), your database access layer is working!

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
3. Ask your assistant for a review — paste your code or the error.
4. Describe the problem out loud; often that surfaces the fix.
