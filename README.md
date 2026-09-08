# NutzMotosportCalendar

A small public fan site showing an **F1 + MotoGP** race calendar with auto-fetched schedules and results. Built as a learning and portfolio project.

**Live:** _deployed soon_

## Features

- Race-day calendar on homepage, grouped by month with filter tabs (All / F1 / MotoGP)
- Event detail pages with session times, circuit info, official/ticket links
- Live countdown to the next session (ticks every second)
- Timezone auto-detect with manual override
- Podium top-3 results with auto-generated summaries
- `.ics` calendar export per series
- Responsive on all devices — phone portrait/landscape, tablet, desktop
- Canceled/postponed events shown grayed out with status badge
- Archive shows past 1 month; older events hidden with notice

## Tech Stack

- **Backend:** Go (`net/http`)
- **Templates:** [Templ](https://templ.guide/)
- **Frontend:** [HTMX](https://htmx.org/) + [Tailwind CSS](https://tailwindcss.com/)
- **Database:** [Supabase](https://supabase.com/) (PostgreSQL)
- **Data Access:** [sqlc](https://sqlc.dev/) + [pgx](https://github.com/jackc/pgx)
- **Hosting:** [Fly.io](https://fly.io/)
- **Data Sources:** [Jolpica](https://api.jolpi.ca/) (F1) · [Dorna Pulselive](https://api.pulselive.motogp.com/) (MotoGP)

## Getting Started

```bash
# 1. Clone
git clone https://github.com/Nutnoobly/NutzMotosportCalendar.git
cd NutzMotosportCalendar

# 2. Install tools (if you haven't already)
go install github.com/a-h/templ/cmd/templ@latest
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# 3. Set up environment
cp .env.example .env
# Edit .env and fill in your Supabase connection pooler DATABASE_URL

# 4. Generate type-safe Go database layer
sqlc generate

# 5. Verify database connection
go run ./cmd/testdb/main.go

# 6. Start dev server (coming in Step 3)
# go run ./cmd/server
```

## Project Structure

```
NutzMotosportCalendar/
├── cmd/                  # Entry points (server, testdb)
├── internal/
│   ├── db/               # Generated database queries & pgxpool helper
│   ├── refresh/          # Fetchers for F1 and MotoGP APIs
│   └── web/              # HTTP handlers
├── db/
│   └── queries/          # Raw SQL query definitions for sqlc
├── supabase/
│   └── migrations/       # Supabase SQL schema migrations
├── views/                # Templ template components
├── static/               # CSS, JS, images
├── sqlc.yaml             # sqlc configuration file
└── .env.example          # Environment variable template
```

## How It Works

1. A daily GitHub Actions cron hits `POST /admin/refresh`
2. Fetchers pull schedules from Jolpica (F1) and Pulselive (MotoGP)
3. Data is upserted into Supabase Postgres
4. Go server renders pages via Templ + HTMX
5. Visitors see the calendar — timezone-aware, countdown ticking

## Credits

Built by [Nutnoobly](https://github.com/Nutnoobly) as a learning project.

Source code: [github.com/Nutnoobly/NutzMotosportCalendar](https://github.com/Nutnoobly/NutzMotosportCalendar)
