# Development Guide

This is the technical and architectural reference for **NutzMotorsportCalendar**.

> ### Operating Rules: Dual-Role Split
> - **Backend (`.go`, SQL, DB)**: **User builds, Assistant reviews.** The user authors backend Go code, routers, queries, and migrations for learning and portfolio purposes. The assistant reviews and mentors.
> - **Frontend & DevOps (`.templ`, HTMX, Tailwind, JS, Deploy)**: **Assistant builds.** The assistant authors all UI templates, styles, client scripts, container configs, and deployment pipelines.
> - **Interface Protocol**: **Contract-First.** The user defines Go handler signatures and view-model structs; the assistant writes the matching Templ components.

---

## 1. System Architecture

A lightweight, mobile-first motorsport calendar and results hub for **Formula 1** and **MotoGP**.

External APIs are never queried during client page requests. Synchronization happens out-of-band:

```
[ GitHub Actions Daily Cron (04:00 UTC) / Manual POST /admin/refresh ]
                               │
                               ▼
                    internal/fetcher orchestrator
                    ┌──────────┴──────────┐
                    ▼                     ▼
               Jolpica API           Pulselive API
               (Formula 1)             (MotoGP)
                    │                     │
                    └──────────┬──────────┘
                               ▼
                    Supabase PostgreSQL (via sqlc)
                               │
                               ▼
               Go HTTP Server (Render Web Service)
                               │
                               ▼
               Visitor Browser (HTMX + Templ + JS)
```

---

## 2. Tech Stack

| Layer | Technology | Role |
| :--- | :--- | :--- |
| **Backend** | Go `net/http` (1.27) | Standard library HTTP server and middleware. No external router framework. |
| **Database** | Supabase (PostgreSQL 17) | Cloud Postgres database accessed via Session Mode Pooler (IPv4, port 5432). |
| **Data Access** | `sqlc` + `pgx/v5` | Type-safe SQL compiler producing idiomatic Go queries. |
| **Templates** | [Templ](https://templ.guide/) | Component-based, type-safe HTML compiled directly to Go functions. |
| **Interactivity** | [HTMX](https://htmx.org/) | Server-driven dynamic swaps for tab filtering and lazy loading. |
| **Styling** | [Tailwind CSS](https://tailwindcss.com/) | Mobile-first responsive styling and typography. |
| **Client JS** | Vanilla JS | Client-side timezone auto-conversion (`timezone.js`) and ticker HUD (`countdown.js`). |
| **Hosting** | [Render](https://render.com/) | Containerized web service running on the 100% Free Plan. |
| **Automation** | GitHub Actions | Daily refresh cron at `04:00 UTC` and automated deployment hooks. |

---

## 3. Database Design

The full schema definition, column types, and Mermaid entity-relationship diagrams are documented in [`DATABASE.md`](DATABASE.md).

### Core Tables
1. `SERIES`: Lookup table (`f1`, `motogp`).
2. `CIRCUITS`: Normalized venue data (name, country, city, coordinates, slug).
3. `TEAMS`: Constructor / team entities per series.
4. `DRIVERS`: Competitors with permanent racing numbers, timing codes, and active team links.
5. `EVENTS`: Grand Prix weekend rounds per season.
6. `SESSIONS`: Weekend timetables (Practice, Qualifying, Sprint, Main Race) stored with UTC timestamps.
7. `RESULTS`: Verified top-3 podium finishes attached to events and session types.
8. `SYNC_RUN`: Audit trail tracking API synchronization execution, duration, and status.

---

## 4. Data Ingestion & Sync Pipeline

Located in `internal/fetcher/`:

- [`internal/fetcher/f1.go`](internal/fetcher/f1.go): Jolpica Ergast client.
  - Schedule & sessions: `https://api.jolpi.ca/ergast/f1/current.json`
  - Results: `https://api.jolpi.ca/ergast/f1/current/results.json` and `sprint.json`
- [`internal/fetcher/motogp.go`](internal/fetcher/motogp.go): Dorna Pulselive client.
  - Requires custom `User-Agent: NutzMotorsportCalendar/1.0`.
  - Resolves active season UUID (`/seasons`) and premier class UUID (`/categories`).
  - Grand Prix events: `https://api.motogp.pulselive.com/motogp/v1/results/events?seasonYear=2026`
  - Classification: `https://api.motogp.pulselive.com/motogp/v1/results/session/{session_id}/classification`
- [`internal/fetcher/sync.go`](internal/fetcher/sync.go): Multi-series coordinator with in-memory team/driver deduplication caching and `SYNC_RUN` audit logging.
- [`cmd/refresh/main.go`](cmd/refresh/main.go): Standalone CLI utility for manual and local out-of-band sync (`go run ./cmd/refresh/main.go`).
- `POST /admin/refresh`: Secret-guarded endpoint (`Authorization: Bearer <ADMIN_SECRET>`) for remote triggers.

---

## 5. Deployment & Infrastructure

- **Multi-Stage Docker**: [`Dockerfile`](Dockerfile) builds a static binary with `templ` in Alpine and packages it into a 21.6 MB runtime container running as non-root on port `8081`.
- **Render Config**: Defined in [`render.yaml`](render.yaml) for automated Blueprint deployment.
- **Keep-Alive & Sync**: Supabase free-tier pauses databases after 7 days of inactivity. The daily GitHub Actions cron (`.github/workflows/refresh-cron.yml`) hits `/admin/refresh` at `04:00 UTC`, keeping the database active and data up to date.

---

## 6. Gotchas & Engineering Tips

1. **`templ generate` First**: Always run `templ generate` before `go build` or `go test` whenever `.templ` files change.
2. **UTC Timestamps in DB**: Always store session and event timestamps as UTC `TIMESTAMPTZ`. Client-side formatting and timezone shifts are handled dynamically in the user's browser by `static/timezone.js`.
3. **Supabase Connection Modes**: Use the **IPv4 Session Mode Pooler** (port `5432`) for `pgxpool`. Do not use Transaction Mode (port `6543`) with `sqlc` prepared queries.
4. **Pulselive User-Agent**: MotoGP Pulselive API returns `403 Forbidden` if the `User-Agent` header is absent or uses default Go HTTP client strings.
5. **Archive Window Query**: The homepage displays current season events where `event_date_end >= NOW() - INTERVAL '1 month'` to keep recent race results visible while hiding older past rounds.

---

## 7. Future Roadmap & Enhancement Milestones

### 7.1 Driver & Constructor Standings
* **Motivation**: Give visitors a complete championship picture without leaving the calendar.
* **Data Sources**:
  - **F1**: Jolpica Ergast endpoints `/ergast/f1/{season}/driverStandings.json` and `/ergast/f1/{season}/constructorStandings.json`.
  - **MotoGP**: Dorna Pulselive standings endpoint or computed dynamically from classified finishes.
* **Backend & Database**:
  - Add `STANDINGS` table tracking `(serie_id, season, entity_type, entity_id, position, points, wins, updated_at)`.
  - Alternatively, compute championship points on-the-fly from historical `RESULTS` via a SQL view.
  - Add `GetDriverStandings(serie_id, season)` and `GetTeamStandings(serie_id, season)` queries to `db/queries/`.
* **Frontend & UI**:
  - Standings tab in navigation and homepage sub-view (swappable via HTMX).
  - Responsive leaderboard table showing rank changes, team color badges, driver numbers, and point gaps.

### 7.2 Full Race Classifications (Beyond Top 3)
* **Current State**: Stores and displays only P1, P2, and P3 podium finishers.
* **Expansion**:
  - Relax position constraints in `RESULTS` table to store all classified positions (P1–P20+), DNFs, DSQs, and fastest laps.
  - Add dedicated Grand Prix results breakdown page under `/events/{slug}/results`.
  - Display gap to leader, pit stop counts, and points earned per driver.

### 7.3 Month-Grid Calendar View
* **Current State**: Chronological vertical card list grouped by month.
* **Expansion**:
  - Add view switcher toggle (List View vs. Calendar Grid View) in header.
  - Traditional 7-column month grid highlighting race weekends with series badge indicators (Red for F1, Teal for MotoGP).
  - Interactive popover or drawer previewing session timetables when clicking a race date.

### 7.4 Live Calendar Subscriptions (WebCal / CalDAV)
* **Current State**: Static `.ics` download for individual series.
* **Expansion**:
  - Provide dynamic live subscription feed URLs (`webcal://<domain>/calendar/f1.ics`).
  - Allow users to customize subscription filters: Main Races only, Qualifying + Races, or Full Weekend (including Free Practice).
  - Include reminder alarms (`VALARM` 30 minutes before session start) in generated iCalendar payloads.

### 7.5 Circuit Weather & Telemetry HUD
* **Motivation**: Enhance event detail pages with real-time conditions.
* **Implementation**:
  - Integrate [Open-Meteo](https://open-meteo.com/) free weather API using circuit latitude and longitude coordinates already stored in `CIRCUITS`.
  - Display weekend track temperature, rain probability, wind speed, and weather icons on event detail pages.

### 7.6 Series Expansion (WEC, IndyCar, Formula E)
* **WEC (World Endurance Championship)**: Multi-class endurance calendar (Hypercar, LMGT3) with 6h/8h/24h race formats.
* **IndyCar**: North American open-wheel schedule and Indy 500 session breakdown.
* **Architecture**: Extend `SERIES` table and implement dedicated fetcher clients adhering to `fetcher.SeriesFetcher` interface.

### 7.7 User Preferences & Race Alerts
* **Local-First Preferences**: Store series filter preferences and favorite drivers in browser `localStorage`.
* **Browser Push Notifications**: Web Push API notifications for upcoming sessions based on client-configured reminders.
