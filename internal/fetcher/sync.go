package fetcher

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts an arbitrary string into a URL-friendly lowercase slug.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// SyncAll coordinates syncing data for all supported motorsport series.
func SyncAll(ctx context.Context, queries *db.Queries) error {
	var errs []string

	if err := SyncSeries(ctx, queries, "f1", func(ctx context.Context) error {
		return SyncF1(ctx, queries)
	}); err != nil {
		errs = append(errs, fmt.Sprintf("f1: %v", err))
	}

	if err := SyncSeries(ctx, queries, "motogp", func(ctx context.Context) error {
		return SyncMotoGP(ctx, queries)
	}); err != nil {
		errs = append(errs, fmt.Sprintf("motogp: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("sync errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// SyncSeries runs a series fetcher and tracks the run inside the SYNC_RUN audit table.
func SyncSeries(ctx context.Context, queries *db.Queries, serieID string, fetchFn func(context.Context) error) error {
	syncRun, err := queries.CreateSyncRun(ctx, db.CreateSyncRunParams{
		SerieID: serieID,
		Message: pgtype.Text{String: "Sync started", Valid: true},
	})
	if err != nil {
		return fmt.Errorf("failed to create sync run for %s: %w", serieID, err)
	}

	log.Printf("[fetcher] Starting sync for series: %s (run #%d)", serieID, syncRun.SyncID)
	syncErr := fetchFn(ctx)

	statusMsg := "Success"
	ok := true
	if syncErr != nil {
		statusMsg = syncErr.Error()
		if len(statusMsg) > 250 {
			statusMsg = statusMsg[:250]
		}
		ok = false
		log.Printf("[fetcher] Sync failed for series %s: %v", serieID, syncErr)
	} else {
		log.Printf("[fetcher] Sync succeeded for series %s", serieID)
	}

	finishErr := queries.FinishSyncRun(ctx, db.FinishSyncRunParams{
		SyncID:  syncRun.SyncID,
		Ok:      ok,
		Message: pgtype.Text{String: statusMsg, Valid: true},
	})
	if finishErr != nil {
		log.Printf("[fetcher] Failed to finish sync run for %s: %v", serieID, finishErr)
	}

	return syncErr
}

// EnsureTeam looks up an existing team by external ID or name, or inserts it.
func EnsureTeam(ctx context.Context, queries *db.Queries, serieID, teamName, externalID string, cache map[string]int32) (int32, error) {
	teamName = strings.TrimSpace(teamName)
	if teamName == "" {
		teamName = "Unknown Team"
	}
	cacheKey := fmt.Sprintf("%s:%s:%s", serieID, teamName, externalID)
	if id, ok := cache[cacheKey]; ok {
		return id, nil
	}

	if externalID != "" {
		team, err := queries.GetTeamByExternalID(ctx, db.GetTeamByExternalIDParams{
			SerieID:        serieID,
			TeamExternalID: pgtype.Text{String: externalID, Valid: true},
		})
		if err == nil {
			cache[cacheKey] = team.TeamID
			return team.TeamID, nil
		}
	}

	team, err := queries.GetTeamByName(ctx, db.GetTeamByNameParams{
		SerieID:  serieID,
		TeamName: teamName,
	})
	if err == nil {
		cache[cacheKey] = team.TeamID
		return team.TeamID, nil
	}

	var extID pgtype.Text
	if externalID != "" {
		extID = pgtype.Text{String: externalID, Valid: true}
	}

	teamID, err := queries.InsertTeam(ctx, db.InsertTeamParams{
		SerieID:        serieID,
		TeamName:       teamName,
		TeamExternalID: extID,
	})
	if err != nil {
		// Fallback check if inserted concurrently
		if t, err2 := queries.GetTeamByName(ctx, db.GetTeamByNameParams{SerieID: serieID, TeamName: teamName}); err2 == nil {
			cache[cacheKey] = t.TeamID
			return t.TeamID, nil
		}
		return 0, fmt.Errorf("failed to insert team %s: %w", teamName, err)
	}

	cache[cacheKey] = teamID
	return teamID, nil
}

// EnsureDriver looks up an existing driver by external ID or inserts them, updating team membership if needed.
func EnsureDriver(ctx context.Context, queries *db.Queries, serieID string, teamID int32, firstName, lastName, code string, number int, externalID string, cache map[string]int32) (int32, error) {
	firstName = strings.TrimSpace(firstName)
	lastName = strings.TrimSpace(lastName)
	if firstName == "" && lastName == "" {
		lastName = "Unknown Driver"
	}
	cacheKey := fmt.Sprintf("%s:%s:%s", serieID, externalID, code)
	if id, ok := cache[cacheKey]; ok {
		if teamID > 0 {
			_ = queries.UpdateDriverTeam(ctx, db.UpdateDriverTeamParams{
				DriverID: id,
				TeamID:   pgtype.Int4{Int32: teamID, Valid: true},
			})
		}
		return id, nil
	}

	if externalID != "" {
		driver, err := queries.GetDriverByExternalID(ctx, db.GetDriverByExternalIDParams{
			SerieID:          serieID,
			DriverExternalID: pgtype.Text{String: externalID, Valid: true},
		})
		if err == nil {
			if teamID > 0 && (!driver.TeamID.Valid || driver.TeamID.Int32 != teamID) {
				_ = queries.UpdateDriverTeam(ctx, db.UpdateDriverTeamParams{
					DriverID: driver.DriverID,
					TeamID:   pgtype.Int4{Int32: teamID, Valid: true},
				})
			}
			cache[cacheKey] = driver.DriverID
			return driver.DriverID, nil
		}
	}

	var teamIDParam pgtype.Int4
	if teamID > 0 {
		teamIDParam = pgtype.Int4{Int32: teamID, Valid: true}
	}
	var driverCode pgtype.Text
	if code != "" {
		driverCode = pgtype.Text{String: code, Valid: true}
	}
	var driverNum pgtype.Int4
	if number > 0 {
		driverNum = pgtype.Int4{Int32: int32(number), Valid: true}
	}
	var extID pgtype.Text
	if externalID != "" {
		extID = pgtype.Text{String: externalID, Valid: true}
	}

	driverID, err := queries.InsertDriver(ctx, db.InsertDriverParams{
		SerieID:          serieID,
		TeamID:           teamIDParam,
		DriverFirstName:  firstName,
		DriverLastName:   lastName,
		DriverCode:       driverCode,
		DriverNumber:     driverNum,
		DriverExternalID: extID,
	})
	if err != nil {
		if externalID != "" {
			if d, err2 := queries.GetDriverByExternalID(ctx, db.GetDriverByExternalIDParams{
				SerieID:          serieID,
				DriverExternalID: pgtype.Text{String: externalID, Valid: true},
			}); err2 == nil {
				cache[cacheKey] = d.DriverID
				return d.DriverID, nil
			}
		}
		return 0, fmt.Errorf("failed to insert driver %s %s: %w", firstName, lastName, err)
	}

	cache[cacheKey] = driverID
	return driverID, nil
}
