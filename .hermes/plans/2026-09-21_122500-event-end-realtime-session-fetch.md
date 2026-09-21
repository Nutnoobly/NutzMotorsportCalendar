# Real-Time Event-End & Session Fetching Architecture Plan

## Goal
Implement an event-aware data synchronization engine that automatically detects when race weekend sessions (Practice, Qualifying, Sprint, and Grand Prix) conclude, ingests classification results from upstream APIs as soon as they become published, and stores Practice and Qualifying top finishers alongside Race/Sprint results.

---

## Current Context & Assumptions

1. **Deployment & Sleeping Constraints**:
   - The Go server runs on Render's Free Tier (`https://nutz-motorsport-calendar.onrender.com`).
   - Render spins down web services after 15 minutes of HTTP inactivity. In-memory Go background loops or cron tickers will sleep when the instance sleeps unless woken up externally.
   - Current synchronization relies exclusively on a single daily GitHub Actions cron (`.github/workflows/refresh-cron.yml` at `04:00 UTC`), creating up to a 24-hour lag after sessions end.

2. **Database Schema Constraints**:
   - The `RESULTS` table (`supabase/migrations/0001_init.sql:88`) enforces `CONSTRAINT results_session_type_check CHECK (session_type IN ('race', 'sprint'))` and `results_position_check CHECK (result_position BETWEEN 1 AND 3)`.
   - Practice and Qualifying results cannot currently be inserted without violating the table check constraint.

3. **Upstream API Capabilities & Latency**:
   - **Formula 1 (Jolpica Ergast API)**:
     - `https://api.jolpi.ca/ergast/f1/current/qualifying.json`: Provides official Qualifying classifications (Q1, Q2, Q3 lap times and grid positions) for all 2026 rounds, published 30–60 minutes after Qualifying concludes.
     - Jolpica does **not** provide Practice (FP1/FP2/FP3) lap times.
   - **Formula 1 (OpenF1 API)**:
     - `https://api.openf1.org/v1/session_result?session_key=...`: Free community real-time telemetry API providing classifications for Practice 1, 2, 3, Qualifying, and Race within 5–15 minutes of chequered flag.
   - **MotoGP (Pulselive API)**:
     - `https://api.motogp.pulselive.com/motogp/v1/results/session/{sessionUuid}/classification`: Provides official classification tables for Free Practice (`FP`), Practice (`PR`), Qualifying (`Q`), Sprint (`SPR`), and Grand Prix (`RAC`), published 15–30 minutes after session conclusion.

4. **Dual-Role Boundary**:
   - The user owns Go backend routes and database migrations for learning.
   - This plan provides exact, self-contained, copy-pasteable SQL, Go fetchers, GitHub Action configs, and Templ components.

---

## Architecture & Proposed Approach

1. **Database Schema Extension**:
   Create migration `0002_add_qualifying_practice_results.sql` to relax `results_session_type_check` to `CHECK (session_type IN ('race', 'sprint', 'qualifying', 'practice'))`, enabling storage of Pole Positions, Qualifying top-3, and Practice fastest lap holders.
2. **Multi-Source Fetcher Pipeline**:
   - **F1**: Ingest Qualifying classifications from Jolpica (`f1QualifyingURL`) into `RESULTS` with `session_type = 'qualifying'`. For Practice sessions, query OpenF1 `session_result` to capture P1–P3 fastest lap drivers under `session_type = 'practice'`.
   - **MotoGP**: Expand `SyncMotoGP` session loop to fetch classifications for `sess.Type IN ('Q', 'FP', 'PR')` in addition to `'RAC'` and `'SPR'`.
3. **Hybrid Event-End Synchronization (Scheduled + On-Demand Lazy Sync)**:
   - **Race Weekend GitHub Actions Cron**: Update `.github/workflows/refresh-cron.yml` with a high-frequency cron schedule (`*/15 06-22 * * 5,6,0`) running every 15 minutes during race weekend broadcast windows (Friday–Sunday), while keeping the daily `0 4 * * *` fallback.
   - **On-Demand Lazy Sync Trigger**: In `internal/web/handler.go` (`handleEventDetail`), if a user loads an event where sessions have started or concluded within the last 3 hours and results are still empty in Supabase, the server asynchronously dispatches a targeted refresh to check if upstream APIs published the results, with a 5-minute debounce to prevent rate-limiting.
4. **Timetable UI Enhancements**:
   Update `internal/views/helpers.go` (`GetSessionWinner`) and `internal/views/event_detail.templ` so timetable rows display `🏆 Pole: [Driver] ([Team])` for Qualifying and `⏱️ Fastest: [Driver] ([Team])` for Practice.

---

## Step-by-Step Implementation Tasks

### Task 1: Database Migration - Expand `RESULTS.session_type`
**File**: `supabase/migrations/0002_add_qualifying_practice_results.sql` (New file)
**Purpose**: Allow `qualifying` and `practice` session types in `RESULTS`.

```sql
-- Migration 0002: Allow qualifying and practice in RESULTS session_type
ALTER TABLE RESULTS DROP CONSTRAINT IF EXISTS results_session_type_check;

ALTER TABLE RESULTS ADD CONSTRAINT results_session_type_check 
    CHECK (session_type IN ('race', 'sprint', 'qualifying', 'practice'));

-- Add index on session_type for fast timetable lookup
CREATE INDEX IF NOT EXISTS idx_results_event_type_pos 
    ON RESULTS(event_id, session_type, result_position);
```

**Verification**:
Execute via Supabase SQL Editor or CLI migration runner.
Verify constraint:
```sql
SELECT conname, pg_get_constraintdef(oid) 
FROM pg_constraint 
WHERE conname = 'results_session_type_check';
```
Expected output: `CHECK ((session_type)::text = ANY (ARRAY['race'::character varying, 'sprint'::character varying, 'qualifying'::character varying, 'practice'::character varying]::text[]))`

---

### Task 2: Update SQL Queries for Practice & Qualifying
**File**: `db/queries/results.sql`
**Changes**: Update comments and add targeted query for session winner lookup.

```sql
-- name: GetSessionResultP1 :one
SELECT 
    r.event_id,
    r.session_type,
    r.result_position,
    r.driver_id,
    r.team_id,
    r.result_time_or_gap,
    r.result_points,
    d.driver_first_name,
    d.driver_last_name,
    d.driver_code,
    d.driver_number,
    t.team_name
FROM RESULTS r
JOIN DRIVERS d ON r.driver_id = d.driver_id
JOIN TEAMS t ON r.team_id = t.team_id
WHERE r.event_id = $1 AND r.session_type = $2 AND r.result_position = 1
LIMIT 1;
```

**Verification**:
```bash
sqlc generate
```
Expected output: Exit code 0, generated `internal/db/results.sql.go` with `GetSessionResultP1`.

---

### Task 3: Ingest F1 Qualifying Results from Jolpica
**File**: `internal/fetcher/f1.go`
**Changes**: Add `f1QualifyingURL`, response types, and `syncF1QualifyingResults`.

```go
var f1QualifyingURL = "https://api.jolpi.ca/ergast/f1/current/qualifying.json"

type f1QualifyingResponse struct {
	MRData struct {
		Total     string `json:"total"`
		RaceTable struct {
			Races []struct {
				Season            string `json:"season"`
				Round             string `json:"round"`
				QualifyingResults []struct {
					Number   string `json:"number"`
					Position string `json:"position"`
					Driver   struct {
						DriverID string `json:"driverId"`
					} `json:"Driver"`
					Constructor struct {
						ConstructorID string `json:"constructorId"`
					} `json:"Constructor"`
					Q1 string `json:"Q1"`
					Q2 string `json:"Q2"`
					Q3 string `json:"Q3"`
				} `json:"QualifyingResults"`
			} `json:"Races"`
		} `json:"RaceTable"`
	} `json:"MRData"`
}

func syncF1QualifyingResults(ctx context.Context, client *http.Client, queries *db.Queries, season int, roundEventMap map[int]int32, teamCache, driverCache map[string]int32) error {
	offset := 0
	const limit = 100

	for {
		url := fmt.Sprintf("%s?limit=%d&offset=%d", f1QualifyingURL, limit, offset)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("qualifying results status: %d", resp.StatusCode)
		}

		var qData f1QualifyingResponse
		err = json.NewDecoder(resp.Body).Decode(&qData)
		resp.Body.Close()
		if err != nil {
			return err
		}

		for _, race := range qData.MRData.RaceTable.Races {
			round, _ := strconv.Atoi(race.Round)
			eventID, exists := roundEventMap[round]
			if !exists {
				continue
			}

			for _, q := range race.QualifyingResults {
				pos, _ := strconv.Atoi(q.Position)
				if pos < 1 || pos > 3 {
					continue
				}

				teamID, hasTeam := teamCache[q.Constructor.ConstructorID]
				driverID, hasDriver := driverCache[q.Driver.DriverID]
				if !hasTeam || !hasDriver {
					continue
				}

				bestTime := q.Q3
				if bestTime == "" {
					bestTime = q.Q2
				}
				if bestTime == "" {
					bestTime = q.Q1
				}

				_ = queries.InsertResult(ctx, db.InsertResultParams{
					EventID:          eventID,
					SessionType:      "qualifying",
					ResultPosition:   int32(pos),
					DriverID:         driverID,
					TeamID:           teamID,
					ResultTimeOrGap:  pgtype.Text{String: bestTime, Valid: bestTime != ""},
					ResultPoints:     pgtype.Float8{Float64: 0, Valid: false},
				})
			}
		}

		total, _ := strconv.Atoi(qData.MRData.Total)
		offset += limit
		if offset >= total || len(qData.MRData.RaceTable.Races) == 0 {
			break
		}
	}
	return nil
}
```

**Verification**:
Wire `syncF1QualifyingResults` into `SyncF1` inside `internal/fetcher/f1.go`.
Run `go test ./... && go vet ./...`.

---

### Task 4: Ingest MotoGP Practice & Qualifying Classifications
**File**: `internal/fetcher/motogp.go`
**Changes**: Update the session classification loop in `SyncMotoGP` to process `Q`, `FP`, and `PR` sessions.

```go
// Allow Q, FP, PR in addition to RAC and SPR
isRaceOrSprint := sess.Type == "RAC" || sess.Type == "SPR"
isQualiOrPractice := strings.HasPrefix(sess.Type, "Q") || strings.HasPrefix(sess.Type, "FP") || sess.Type == "PR"

if (isRaceOrSprint || isQualiOrPractice) && (eventStatus == "completed" || sess.Status == "FINISHED") {
	classURL := fmt.Sprintf("%s/session/%s/classification", motogpBaseURL, sess.ID)
	var classResp motogpClassificationResponse
	if err := motogpGet(ctx, client, classURL, &classResp); err == nil && len(classResp.Classification) > 0 {
		sessionType := "race"
		switch {
		case sess.Type == "SPR":
			sessionType = "sprint"
		case strings.HasPrefix(sess.Type, "Q"):
			sessionType = "qualifying"
		case strings.HasPrefix(sess.Type, "FP") || sess.Type == "PR":
			sessionType = "practice"
		}

		for _, row := range classResp.Classification {
			if row.Position < 1 || row.Position > 3 {
				continue
			}
			// ensure team & driver and call InsertResult with sessionType
			// ...
		}
	}
}
```

**Verification**:
Run `go test -v ./...` to verify clean compilation.

---

### Task 5: Race Weekend High-Frequency Cron Schedule
**File**: `.github/workflows/refresh-cron.yml`
**Changes**: Add cron triggers targeting active weekend hours (06:00 UTC to 22:00 UTC on Friday, Saturday, Sunday) every 15 minutes.

```yaml
name: Scheduled Data Refresh

on:
  schedule:
    # 1. Standard daily sync at 04:00 UTC (weekdays)
    - cron: "0 4 * * 1-4"
    # 2. Weekend active broadcast window: Every 15 minutes Friday through Sunday (06:00 to 22:00 UTC)
    - cron: "*/15 6-22 * * 5,6,0"
  workflow_dispatch:
    inputs:
      reason:
        description: "Reason for triggering manual refresh"
        required: false
        default: "Manual run via GitHub Actions"
```

**Verification**:
Inspect workflow syntax: `actionlint` or YAML validation.

---

### Task 6: On-Demand Lazy Refresh Trigger for Live/Recent Events
**File**: `internal/web/handler.go`
**Changes**: In `handleEventDetail`, check if an active or recently finished event needs a background refresh.

```go
var (
	lastEventRefreshMu sync.Mutex
	lastEventRefresh   = make(map[string]time.Time)
)

func shouldTriggerEventRefresh(event db.GetEventBySlugRow, results []db.ListResultsByEventIDRow) bool {
	if len(results) > 0 && event.EventStatus == "completed" {
		return false
	}
	now := time.Now()
	// If event started within last 6 hours or is scheduled for today
	if event.EventStartsAt.Valid {
		diff := now.Sub(event.EventStartsAt.Time)
		if diff >= -2*time.Hour && diff <= 6*time.Hour {
			lastEventRefreshMu.Lock()
			defer lastEventRefreshMu.Unlock()
			lastTime, seen := lastEventRefresh[event.EventSlug]
			if !seen || now.Sub(lastTime) > 5*time.Minute {
				lastEventRefresh[event.EventSlug] = now
				return true
			}
		}
	}
	return false
}
```

In `handleEventDetail`:
```go
if shouldTriggerEventRefresh(event, results) {
	go func(serieID string) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if serieID == "f1" {
			_ = fetcher.SyncF1(ctx, s.queries)
		} else if serieID == "motogp" {
			_ = fetcher.SyncMotoGP(ctx, s.queries)
		}
	}(event.SerieID)
}
```

**Verification**:
Run `go build ./... && go vet ./...`.

---

### Task 7: Update Session Winner Badges for Practice & Qualifying
**File**: `internal/views/helpers.go`
**Changes**: Update `GetSessionWinner` to support `qualifying` and `practice`.

```go
// GetSessionWinner finds the P1 winner for race, sprint, qualifying, or practice.
func GetSessionWinner(sessionKind string, results []db.ListResultsByEventIDRow) *db.ListResultsByEventIDRow {
	targetType := ""
	switch strings.ToLower(sessionKind) {
	case "race":
		targetType = "race"
	case "sprint":
		targetType = "sprint"
	case "qualifying":
		targetType = "qualifying"
	case "practice":
		targetType = "practice"
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
```

**File**: `internal/views/event_detail.templ`
**Changes**: Update timetable winner pill to label Pole Position vs Fastest Lap vs Winner:

```templ
if winner := GetSessionWinner(sess.SessionKind, results); winner != nil {
	<div class="mt-1.5 flex items-center gap-1.5 text-xs">
		if sess.SessionKind == "qualifying" {
			<span class="px-1.5 py-0.5 rounded bg-amber-500/10 text-amber-600 dark:text-amber-400 font-semibold border border-amber-500/20">
				🎯 Pole: { winner.DriverFirstName } { winner.DriverLastName }
			</span>
		} else if sess.SessionKind == "practice" {
			<span class="px-1.5 py-0.5 rounded bg-sky-500/10 text-sky-600 dark:text-sky-400 font-semibold border border-sky-500/20">
				⏱️ Fastest: { winner.DriverFirstName } { winner.DriverLastName }
			</span>
		} else {
			<span class="px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 font-semibold border border-emerald-500/20">
				🏆 Winner: { winner.DriverFirstName } { winner.DriverLastName }
			</span>
		}
	</div>
}
```

**Verification**:
```bash
templ generate
go test -v ./internal/views
```
Expected output: All unit tests pass, exit code 0.

---

## Tests & Validation Strategy

1. **Unit Tests (TDD)**:
   - Add `TestGetSessionWinnerQualifyingAndPractice` in `internal/views/helpers_test.go` verifying that `GetSessionWinner("qualifying", results)` and `GetSessionWinner("practice", results)` return the correct P1 driver.
   - Add test in `internal/web/server_test.go` verifying `shouldTriggerEventRefresh` returns true only within the active race window and respects the 5-minute debounce.
2. **Integration Verification**:
   - Run `go run ./cmd/refresh/main.go` locally against Supabase to ingest 2026 Qualifying and Practice data.
   - Verify `RESULTS` table contains rows with `session_type = 'qualifying'` and `session_type = 'practice'`.
   - Start local server (`go run ./cmd/server/main.go`) and open `http://localhost:8081/events/f1-2026-australian-grand-prix` to confirm `🎯 Pole: George Russell` appears on Saturday's Qualifying timetable row.

---

## Risks, Tradeoffs & Open Questions

1. **Render Free-Tier Sleeping**:
   - *Risk*: A 15-minute GitHub Action cron request might wake up the sleeping Render instance with a 30–50 second cold start delay.
   - *Mitigation*: The GHA curl step has `-L` and a 60-second timeout, ensuring the cold boot finishes and the refresh is accepted (`202 Accepted`).
2. **Upstream API Rate Limiting**:
   - *Risk*: Polling Jolpica and Pulselive every 15 minutes all weekend could risk HTTP 429 rate limits.
   - *Mitigation*: Only poll active rounds rather than re-scanning the whole 24-round season every turn. Jolpica is cached behind Cloudflare and accommodates regular queries; Pulselive has generous limits.
3. **OpenF1 vs Jolpica for Practice**:
   - *Tradeoff*: Jolpica does not provide practice session classifications. OpenF1 has practice results, but its API endpoints are newer and can change across F1 seasons.
   - *Recommendation*: Phase 1: Ingest Jolpica Qualifying (reliable, standard Ergast format) and MotoGP Practice/Qualifying (Pulselive). Phase 2: Add OpenF1 for live F1 FP1–FP3 fastest laps.
