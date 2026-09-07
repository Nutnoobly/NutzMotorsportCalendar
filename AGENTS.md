# Grilling Session Tracker

This file tracks an active grill-me interview for **NutzMotosportCalendar**.
It is the session memory: read it at the start of every session.

On session start:
1. Read this file top to bottom.
2. Resume grilling from "Open Frontier" — ask the whole frontier each round.
3. After each round, update "Decisions Locked", "Open Frontier", and "Session Log".
4. Do not start building until the ending gate passes:
   frontier empty AND the user confirms shared understanding.

## The Idea

A small public, mobile-first fan site (English) showing an F1 + MotoGP calendar with
auto-fetched schedules and results — a learning/portfolio project credited to
github.com/Nutnoobly/NutzMotosportCalendar.

## Design Tree

- Scope & audience: multi-series hub (F1 + MotoGP; WEC dropped), fans + self, portfolio
- Content model: race-day list on home, per-event detail pages, timezone handling, countdown
- Results: podium top-3 + generated summary, link out for full classification
- Data pipeline: scheduled fetch from upstream APIs into own Supabase store
- Platform: Go stdlib + Templ + HTMX + Tailwind on Fly.io, no auth v1
- UI: responsive all devices, portrait & landscape

## Decisions Locked

| Round | Decision | Rationale |
|-------|----------|-----------|
| 1 | Multi-series hub, small public site, auto-fetch data, results in scope, learning/portfolio goal | Root intent |
| 2 | Series = F1 + MotoGP + WEC; race day on main page, detail pages carry more info; timezone auto-detect + manual override; homepage countdown; daily refresh into own storage; stack = Go net/http, Supabase, sqlc/pgx, Templ, HTMX, Tailwind; English | User's picks |
| 3 | PaaS host + GHA daily cron → secret-guarded /admin/refresh; detail page = circuit/location + official & ticket links + results block; summaries template-generated; no auth (localStorage prefs); month-grouped vertical list w/ All/F1/MotoGP tabs; .ics export; archive window = past 1 month w/ notice; footer credits repo link | Frontier R3 |
| 4 | WEC dropped entirely → F1 + MotoGP only; host = Fly.io; off-season homepage shows last completed race + "no future event scheduled"; canceled/postponed events shown grayed-out with status badge | Clarity |
| 5 | Fully responsive on all devices — small phone to big desktop, both portrait and landscape layouts | Hard requirement before build |
| 6 | Normalized drivers, teams, and circuits tables; results attach to events with session_type ('race'/'sprint'); strict top-3 check; natural key (series_id, season, round) + external_id; dual diagram (Mermaid ERD + UML class diagram) | DB architecture locked |
| 7 | Results hold driver_id + team_id FKs, drivers hold current_team_id FK; circuits use surrogate BIGSERIAL + slug; summary is on-the-fly Go/Templ; sessions table kept for weekend timetables | Schema details locked |

## Build Status

- Grilling complete for product scope and database design. User codes the backend; assistant guides/reviews only.
- Backend roadmap was delivered in chat on 2026-08-25 (NOT saved as a file — user chose chat-only).
- Toolchain ready: Go 1.27.0 at ~/.local/go; templ v0.3.1020 + sqlc v1.31.1 at ~/go/bin;
  go.mod initialized (module github.com/Nutnoobly/NutzMotosportCalendar). Shell needs:
  `export PATH=$HOME/.local/go/bin:$HOME/go/bin:$PATH`
- Roadmap steps: [done] 0 toolchain → [in progress] 1 migrations (USER-authored):
  go.mod re-created by user themselves (module github.com/Nutnoobly/NutzMotosportCalendar,
  go 1.26.6; Motorsport→Motosport typo caught in review and fixed by user).
  User created EMPTY supabase/migrations/0001_init.sql using Supabase CLI convention path
  (not repo-root migrations/) — runner must point at that folder. SQL content still to be written.
  → then 2 sqlc/pgx queries → 3 net/http routes → 4 templ views → 5 htmx/countdown
  → 6 timezone JS → 7 refresh fetchers (Jolpica F1, Pulselive MotoGP) → 8 deploy (Fly.io + GHA cron).
- Resume point: finalize DB schema & UML, then user fills in supabase/migrations/0001_init.sql.
- Docs written 2026-08-26: README.md (public portfolio — features, stack, quickstart, structure)
  and DEVELOPMENT.md (private dev guide — DB design, migrations, data sources, build order 0–8
  checklist, gotchas). User authors SQL/code; both docs align with locked decisions.
- 2026-08-26: go.mod/go.sum deleted at user request — they will re-run `go mod init` and author
  all module setup themselves. A .env (user-created) exists locally; never read or commit it.

## Open Frontier

- (none — database design grilling completed; awaiting user confirmation of shared understanding)

## Session Log

- 2026-08-25: grilling session started (project: NutzMotosportCalendar)
- 2026-08-25: rounds 1–5 completed; sources verified (Jolpica api.jolpi.ca for F1,
  api.pulselive.motogp.com for MotoGP); frontier emptied; tracker synced after plan mode lifted;
  ending-gate confirmation pending.
- 2026-08-25: ending gate passed (user confirmed + added R5 responsiveness). Build phase started,
  then pivoted by user: THEY will code the backend themselves — my role becomes guidance/review.
  Installed Go 1.27.0 to ~/.local/go for local verification. Do not implement the backend for them
  unless explicitly asked.
- 2026-08-25: session saved at user request. Step 0 (toolchain + go.mod) completed by assistant.
  Next session: resume from "Build Status → Resume point" — user brings Supabase DATABASE_URL
  and/or migrations draft; assistant reviews, then guides step 2 (sqlc).
- 2026-08-26: user confirmed Supabase project created (db.ctiwpxwvnpmglwsafnzp). Said "go" →
  plan mode lifted; assistant wired pgx v5.10 + migration runner (scaffolding only — migration
  SQL remains the user's job per their learning goal). Build passes (vet+build). Next: user drafts
  migrations/0001_init.sql, adds real DATABASE_URL to .env, runs go run ./cmd/migrate.
- 2026-08-26: user asked to redo DB/backend themselves → assistant reverted its scaffolding
  (internal/db, cmd/migrate, pgx dep) back to bare go.mod; kept .env.example + .gitignore.
  Assistant role re-confirmed: review-only for ALL backend code.
- 2026-08-26: user re-ran go mod init (typo fixed after review), created empty
  supabase/migrations/0001_init.sql. Awaiting SQL content for review before step 2 (sqlc).
- 2026-08-26: assistant wrote README.md (public) + DEVELOPMENT.md (dev guide) at user request;
  tracker updated. Next: user fills in supabase/migrations/0001_init.sql for review.
- 2026-08-26: AGENTS.md file was deleted from disk (git commit "Remove agents from git").
  Recreated at user request ("add agent.md to my project file"). Warning: user repo commits show
  a prior "Remove agents from git" — confirm whether AGENTS.md should be git-tracked or kept
  local-only before any commit.
- 2026-09-07: user invoked /grill-me for database design + UML diagram. Opened Round 6 frontier on schema architecture and normalization.
- 2026-09-07: Round 6 completed. Locked normalized drivers/teams/circuits, session_type for sprint/race results, strict top-3 check, natural key + external_id, and dual ERD+UML diagram. Opened Round 7 for schema child details.
- 2026-09-07: Round 7 completed. Locked driver/team dual foreign keys + driver current_team_id, surrogate circuit PK + slug, on-the-fly Go summary, and timetable sessions table. Frontier emptied.



