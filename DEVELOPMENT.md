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

This is your first big backend task. Data is stored in **tables**. A table has **columns**
(like a spreadsheet's columns) and **rows** (entries).

### 3.1 The five tables

```
series
  id          TEXT PRIMARY KEY   -- 'f1' or 'motogp'
  name        TEXT NOT NULL

events
  id            BIGSERIAL PRIMARY KEY
  series_id     TEXT REFERENCES series(id)
  season        INT
  round         INT
  slug          TEXT UNIQUE       -- human-friendly url, e.g. '2026-monza-gp'
  name          TEXT              -- 'Italian Grand Prix'
  circuit_name  TEXT
  locality      TEXT              -- city, e.g. 'Monza'
  country       TEXT              -- e.g. 'Italy'
  starts_at     TIMESTAMPTZ       -- race start time
  status        TEXT              -- 'scheduled' | 'completed' | 'cancelled' | 'postponed'
  official_url  TEXT
  ticket_url    TEXT

sessions
  id         BIGSERIAL PRIMARY KEY
  event_id   BIGINT REFERENCES events(id) ON DELETE CASCADE
  kind       TEXT                 -- 'practice' | 'qualifying' | 'race' | 'sprint'
  name       TEXT
  starts_at  TIMESTAMPTZ

results
  event_id     BIGINT REFERENCES events(id) ON DELETE CASCADE
  position     INT
  driver_name  TEXT
  team_name    TEXT
  PRIMARY KEY (event_id, position)

sync_runs
  id          BIGSERIAL PRIMARY KEY
  series_id   TEXT
  started_at  TIMESTAMPTZ DEFAULT now()
  ok          BOOLEAN
  message     TEXT
```

### 3.2 Relationships (how tables connect)

```
series 1 ──── many events ──── many sessions
            event ──── many results
```

- A **series** (F1 or MotoGP) has many **events** (races).
- An **event** has many **sessions** (practice, quali, race) and many **results** positions.
- `series_id` on events is a **foreign key**: it says "this event belongs to which series?"
- `ON DELETE CASCADE`: delete an event → its sessions and results go too (cleanup for free).

### 3.3 Rules that keep you out of trouble

- **All timestamps are `TIMESTAMPTZ`.** This stores an instant in time with timezone info,
  in UTC internally. If you use plain `TIMESTAMP`, you'll hit timezone bugs later.
- **`slug` is `UNIQUE`** — it becomes part of the URL (`/events/{slug}`). Duplicates would
  break routing.
- **Composite primary key `(event_id, position)`** on results — one event can't have two
  drivers in the same position. This is the correct key, not an auto-id.
- **Standing: `status` as text** makes it simple to gray out canceled events.

### 3.4 Why a separate `sessions` table?

An F1 weekend has practice, quali, race — each at a different time. If you crammed them into
`events` as separate columns, every event would have empty columns (MotoGP has different
session structures). A child table handles "maybe many, maybe different" cleanly.

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

- [ ] **0. Toolchain** — done (Go, templ, sqlc, go.mod).
- [ ] **1. Migrations** — write `supabase/migrations/0001_init.sql` + your runner (`cmd/migrate`).
- [ ] **2. sqlc queries** — write `SELECT`/`INSERT` SQL, run `sqlc generate`, get typed Go.
- [ ] **3. HTTP routes** — `net/http` mux: home, event detail, `.ics`, refresh endpoint.
- [ ] **4. Templ views** — layout, home, event pages; `templ generate` after each edit.
- [ ] **5. HTMX + countdown** — filter tabs swap sections; countdown JS ticks each second.
- [ ] **6. Timezone JS** — render UTC, convert to visitor's zone client-side.
- [ ] **7. Refresh fetchers** — Jolpica + Pulselive, upsert into DB.
- [ ] **8. Deploy** — Dockerfile, Fly.io, GitHub Actions daily cron.

---

## 7. Gotchas Cheat Sheet

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

## 8. Ideas to Add Later (after v1 works)

Don't build these now. Park them here.

- WEC series (needs a data source)
- Full results classification pages
- Driver/team standings
- User accounts (Supabase Auth is already available)
- Month-grid calendar view

---

## 9. If You're Stuck

1. Re-read the relevant section here.
2. Check the API docs for whatever's failing (Jolpica, Pulselive, Supabase, pgx, Templ).
3. Ask your assistant for a review — paste your code or the error.
4. Describe the problem out loud; often that surfaces the fix.
