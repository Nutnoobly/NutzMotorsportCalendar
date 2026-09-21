# Implementation Plan: Completed Grand Prix Race & Session Summaries

**File:** `.hermes/plans/2026-09-21_105800-completed-race-session-summaries.md`  
**Date:** 2026-09-21  
**Author:** Hermes Agent & User  
**Status:** Approved (Grilling Round 10 Locked)

---

## 1. Goal

Provide an automated, on-the-fly editorial summary banner for completed Grand Prix events and inline lifecycle/winner summaries for each session in the weekend timetable without schema changes.

---

## 2. Current Context & Assumptions

- **Dual-Role Agreement (`AGENTS.md`)**: The agent is the active developer for frontend templates (`.templ`), Tailwind styling, and client helpers. The Go backend/DB is user-owned (review-only).
- **Existing Data**:
  - `EVENTS` table stores `event_status` ('scheduled', 'completed', 'cancelled', 'postponed').
  - `RESULTS` table stores top-3 classifications for `session_type IN ('race', 'sprint')` with `driver_first_name`, `driver_last_name`, `driver_code`, `team_name`, and `result_time_or_gap`.
  - `SESSIONS` table stores `session_kind` ('practice', 'qualifying', 'sprint', 'race', 'other'), `session_name`, and `session_starts_at`.
- **Codebase Baseline**:
  - `internal/views/helpers.go` has an unused, basic `GeneratePodiumSummary(p1, p2, p3 string) string`.
  - `internal/views/event_detail.templ` renders the timetable list and podium cards but has no editorial summary block, no session completion indicators, and no winner badges in the timetable.

---

## 3. Architecture & Proposed Approach

1. **On-The-Fly Summary Engine (`internal/views/helpers.go`)**:
   - Upgrade `GeneratePodiumSummary` into a comprehensive `GenerateRaceSummary(results []db.ListResultsByEventIDRow) string` that inspects both Grand Prix and Sprint results to construct a fluent editorial summary sentence.
   - Introduce `GetSessionWinner(sessionKind string, results []db.ListResultsByEventIDRow) *db.ListResultsByEventIDRow` to match Sprint and Race sessions to their respective P1 winner.
   - Introduce `GetSessionStatus(startsAt pgtype.Timestamptz, eventStatus string) string` to calculate lifecycle status (`"Completed"`, `"Live Now"`, `"Scheduled"`) based on event status and timestamp.
2. **Event Detail View Enhancements (`internal/views/event_detail.templ`)**:
   - Insert a **Race Recap Hero Banner** between the Event Header and the 2-column grid when an event is marked completed and has results.
   - Enhance the **Weekend Timetable** rows to display session lifecycle status chips and inline winner callouts for Sprint and Race sessions.
   - Integrate an editorial recap sentence at the top of the **Podium & Results** section.
3. **Pure Frontend Scope**:
   - Zero SQL migrations, zero sqlc changes, zero HTTP route changes.
   - Built with Templ components and styled with existing Tailwind design tokens.

---

## 4. Step-by-Step Implementation Tasks

### Task 1: Add Unit Tests for Summary and Session Helpers
**File**: `internal/views/helpers_test.go`
- Write comprehensive tests for `GenerateRaceSummary`, `GetSessionWinner`, and `GetSessionStatus`.

```go
package views

import (
	"testing"
	"time"

	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestGenerateRaceSummary(t *testing.T) {
	results := []db.ListResultsByEventIDRow{
		{
			SessionType:     "race",
			ResultPosition:  1,
			DriverFirstName: "Max",
			DriverLastName:  "Verstappen",
			TeamName:        "Red Bull Racing",
			ResultTimeOrGap: pgtype.Text{String: "1:25:30.123", Valid: true},
		},
		{
			SessionType:     "race",
			ResultPosition:  2,
			DriverFirstName: "Lando",
			DriverLastName:  "Norris",
			TeamName:        "McLaren",
			ResultTimeOrGap: pgtype.Text{String: "+2.456s", Valid: true},
		},
		{
			SessionType:     "race",
			ResultPosition:  3,
			DriverFirstName: "Charles",
			DriverLastName:  "Leclerc",
			TeamName:        "Ferrari",
			ResultTimeOrGap: pgtype.Text{String: "+5.120s", Valid: true},
		},
	}

	summary := GenerateRaceSummary(results)
	expected := "Max Verstappen took victory in the Grand Prix, followed by Lando Norris in 2nd (+2.456s) and Charles Leclerc in 3rd."
	if summary != expected {
		t.Errorf("expected %q, got %q", expected, summary)
	}
}

func TestGetSessionWinner(t *testing.T) {
	results := []db.ListResultsByEventIDRow{
		{
			SessionType:     "race",
			ResultPosition:  1,
			DriverFirstName: "Max",
			DriverLastName:  "Verstappen",
		},
		{
			SessionType:     "sprint",
			ResultPosition:  1,
			DriverFirstName: "Lando",
			DriverLastName:  "Norris",
		},
	}

	raceWinner := GetSessionWinner("race", results)
	if raceWinner == nil || raceWinner.DriverLastName != "Verstappen" {
		t.Errorf("expected Verstappen as race winner")
	}

	sprintWinner := GetSessionWinner("sprint", results)
	if sprintWinner == nil || sprintWinner.DriverLastName != "Norris" {
		t.Errorf("expected Norris as sprint winner")
	}

	practiceWinner := GetSessionWinner("practice", results)
	if practiceWinner != nil {
		t.Errorf("expected nil for practice winner")
	}
}
```
**Verification Command**:
```bash
go test -v ./internal/views/ -run "TestGenerateRaceSummary|TestGetSessionWinner"
```
*(Fails initially as functions are not yet implemented)*

---

### Task 2: Implement Logic in `internal/views/helpers.go`
**File**: `internal/views/helpers.go`
- Implement `GenerateRaceSummary(results []db.ListResultsByEventIDRow) string`
- Implement `GetSessionWinner(sessionKind string, results []db.ListResultsByEventIDRow) *db.ListResultsByEventIDRow`
- Implement `GetSessionStatus(startsAt pgtype.Timestamptz, eventStatus string) string`
- Implement `SessionStatusBadgeClass(status string) string`

```go
// GenerateRaceSummary produces a descriptive editorial recap from top-3 results.
func GenerateRaceSummary(results []db.ListResultsByEventIDRow) string {
	var p1, p2, p3 *db.ListResultsByEventIDRow
	var sprintP1 *db.ListResultsByEventIDRow

	for i := range results {
		r := &results[i]
		if r.SessionType == "race" {
			switch r.ResultPosition {
			case 1:
				p1 = r
			case 2:
				p2 = r
			case 3:
				p3 = r
			}
		} else if r.SessionType == "sprint" && r.ResultPosition == 1 {
			sprintP1 = r
		}
	}

	if p1 == nil {
		return ""
	}

	summary := fmt.Sprintf("%s %s took victory in the Grand Prix", p1.DriverFirstName, p1.DriverLastName)
	if p2 != nil && p3 != nil {
		gap := ""
		if p2.ResultTimeOrGap.Valid && p2.ResultTimeOrGap.String != "" {
			gap = fmt.Sprintf(" (%s)", p2.ResultTimeOrGap.String)
		}
		summary += fmt.Sprintf(", followed by %s %s in 2nd%s and %s %s in 3rd.", 
			p2.DriverFirstName, p2.DriverLastName, gap, p3.DriverFirstName, p3.DriverLastName)
	} else if p2 != nil {
		summary += fmt.Sprintf(", with %s %s in 2nd.", p2.DriverFirstName, p2.DriverLastName)
	} else {
		summary += "."
	}

	if sprintP1 != nil {
		summary += fmt.Sprintf(" %s %s also claimed victory in Saturday's Sprint.", sprintP1.DriverFirstName, sprintP1.DriverLastName)
	}

	return summary
}

// GetSessionWinner finds the P1 winner for a given session kind ('race' or 'sprint').
func GetSessionWinner(sessionKind string, results []db.ListResultsByEventIDRow) *db.ListResultsByEventIDRow {
	targetType := ""
	switch sessionKind {
	case "race":
		targetType = "race"
	case "sprint":
		targetType = "sprint"
	default:
		return nil
	}

	for i := range results {
		if results[i].SessionType == targetType && results[i].ResultPosition == 1 {
			return &results[i]
		}
	}
	return nil
}

// GetSessionStatus determines if a session is Completed, Live Now, or Scheduled.
func GetSessionStatus(startsAt pgtype.Timestamptz, eventStatus string) string {
	if eventStatus == "completed" {
		return "Completed"
	}
	if eventStatus == "cancelled" {
		return "Cancelled"
	}
	if eventStatus == "postponed" {
		return "Postponed"
	}
	if !startsAt.Valid {
		return "Scheduled"
	}

	now := time.Now().UTC()
	start := startsAt.Time.UTC()
	if now.Before(start) {
		return "Scheduled"
	}
	// Sessions typically run 1 to 2.5 hours
	if now.Sub(start) < 2*time.Hour {
		return "Live Now"
	}
	return "Completed"
}

// SessionStatusBadgeClass returns Tailwind badge styles for session states.
func SessionStatusBadgeClass(status string) string {
	switch status {
	case "Completed":
		return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300 border-slate-300 dark:border-slate-700"
	case "Live Now":
		return "bg-emerald-100 text-emerald-800 dark:bg-emerald-950/70 dark:text-emerald-400 border-emerald-300 dark:border-emerald-700 animate-pulse"
	case "Cancelled":
		return "bg-rose-100 text-rose-800 dark:bg-rose-950/70 dark:text-rose-400 border-rose-300 dark:border-rose-800"
	default:
		return "bg-slate-50 text-slate-600 dark:bg-slate-900/50 dark:text-slate-400 border-slate-200 dark:border-slate-800"
	}
}
```

**Verification Command**:
```bash
go test -v ./internal/views/
```
Expected output: `PASS`

---

### Task 3: Update `internal/views/event_detail.templ`
**File**: `internal/views/event_detail.templ`

1. **Insert Race Recap Hero Banner**:
```templ
<!-- Race Recap Hero Banner (if event is completed & has results) -->
if event.EventStatus == "completed" && GenerateRaceSummary(results) != "" {
    <div class="rounded-2xl border border-amber-500/30 bg-gradient-to-r from-amber-500/10 via-amber-500/5 to-transparent p-6 sm:p-7 shadow-sm">
        <div class="flex items-start gap-4">
            <div class="w-10 h-10 rounded-xl bg-amber-500/20 border border-amber-500/40 flex items-center justify-center text-xl shrink-0">
                🏆
            </div>
            <div class="space-y-1.5">
                <div class="flex items-center gap-2">
                    <span class="text-xs font-mono font-bold uppercase tracking-wider text-amber-600 dark:text-amber-400">
                        Official Race Summary
                    </span>
                </div>
                <p class="text-base sm:text-lg font-medium text-slate-900 dark:text-slate-100 leading-snug">
                    { GenerateRaceSummary(results) }
                </p>
            </div>
        </div>
    </div>
}
```

2. **Enhance Weekend Timetable**:
Inside `for _, sess := range sessions`:
```templ
<div class="p-4 flex items-center justify-between hover:bg-slate-50 dark:hover:bg-slate-900/60 transition-colors">
    <div class="space-y-1">
        <div class="flex items-center gap-2">
            <span class="text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 block">
                { sess.SessionKind }
            </span>
            <span class={ "text-[10px] font-mono font-semibold px-2 py-0.5 rounded border", SessionStatusBadgeClass(GetSessionStatus(sess.SessionStartsAt, event.EventStatus)) }>
                { GetSessionStatus(sess.SessionStartsAt, event.EventStatus) }
            </span>
        </div>
        <span class="text-sm font-semibold text-slate-900 dark:text-white block">
            { sess.SessionName }
        </span>
        if winner := GetSessionWinner(sess.SessionKind, results); winner != nil {
            <div class="text-xs text-amber-600 dark:text-amber-400 font-medium flex items-center gap-1.5 pt-0.5">
                <span>🏆 Winner:</span>
                <span class="font-bold">{ winner.DriverFirstName } { winner.DriverLastName }</span>
                <span class="text-slate-400">({ winner.TeamName })</span>
            </div>
        }
    </div>
    <div class="text-right shrink-0">
        <span class="text-xs font-mono font-medium text-slate-700 dark:text-slate-300 block" data-utc={ ISOTimestamp(sess.SessionStartsAt) } data-format="date">
            { FormatDate(sess.SessionStartsAt) }
        </span>
        <span class="text-xs font-mono text-slate-500 dark:text-slate-400" data-utc={ ISOTimestamp(sess.SessionStartsAt) } data-format="time">
            { FormatTimeUTC(sess.SessionStartsAt) }
        </span>
    </div>
</div>
```

**Verification Command**:
```bash
templ generate
go build ./... && go vet ./...
```
Expected output: Clean compilation (exit code 0).

---

## 5. Verification & Test Plan

1. **Automated Unit Tests**:
   - `go test -v ./internal/views/...` verifies summary formatting, edge cases (no sprint, missing gaps, missing P3), and status classification.
2. **Visual & Browser Verification**:
   - Run `go run ./cmd/server/main.go`
   - Visit `http://localhost:8081/events/<completed-grand-prix-slug>` (e.g., Bahrain or Monza).
   - Verify:
     - Prominent Gold Race Summary Hero Banner displays at the top.
     - Timetable lists "Completed" badges and displays `🏆 Winner: [Driver Name]` on Race and Sprint sessions.
     - Scheduled upcoming races continue to render clean timetable rows without phantom winners.

---

## 6. Risks, Tradeoffs & Alternatives Considered

- **Tradeoff: Practice / Qualifying Results**:
  - *Decision*: Practice and qualifying sessions will show lifecycle status (`Completed`) rather than pole position / fastest lap because the current `RESULTS` table only captures `race` and `sprint`.
  - *Future Enhancement*: When upstream fetchers ingest qualifying times, expanding `RESULTS` to support `qualifying` will automatically plug into this session winner architecture.
