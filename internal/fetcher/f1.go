package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	f1ScheduleURL = "https://api.jolpi.ca/ergast/f1/current.json?limit=100"
	f1ResultsURL  = "https://api.jolpi.ca/ergast/f1/current/results.json"
	f1SprintURL   = "https://api.jolpi.ca/ergast/f1/current/sprint.json"
)

type f1ScheduleResponse struct {
	MRData struct {
		RaceTable struct {
			Season string `json:"season"`
			Races  []struct {
				Season   string `json:"season"`
				Round    string `json:"round"`
				URL      string `json:"url"`
				RaceName string `json:"raceName"`
				Circuit  struct {
					CircuitID   string `json:"circuitId"`
					URL         string `json:"url"`
					CircuitName string `json:"circuitName"`
					Location    struct {
						Lat      string `json:"lat"`
						Long     string `json:"long"`
						Locality string `json:"locality"`
						Country  string `json:"country"`
					} `json:"Location"`
				} `json:"Circuit"`
				Date             string         `json:"date"`
				Time             string         `json:"time"`
				FirstPractice    *f1SessionItem `json:"FirstPractice"`
				SecondPractice   *f1SessionItem `json:"SecondPractice"`
				ThirdPractice    *f1SessionItem `json:"ThirdPractice"`
				Qualifying       *f1SessionItem `json:"Qualifying"`
				Sprint           *f1SessionItem `json:"Sprint"`
				SprintQualifying *f1SessionItem `json:"SprintQualifying"`
				SprintShootout   *f1SessionItem `json:"SprintShootout"`
			} `json:"Races"`
		} `json:"RaceTable"`
	} `json:"MRData"`
}

type f1SessionItem struct {
	Date string `json:"date"`
	Time string `json:"time"`
}

type f1ResultsResponse struct {
	MRData struct {
		Total     string `json:"total"`
		Limit     string `json:"limit"`
		Offset    string `json:"offset"`
		RaceTable struct {
			Season string `json:"season"`
			Races  []struct {
				Season  string `json:"season"`
				Round   string `json:"round"`
				Results []struct {
					Number   string `json:"number"`
					Position string `json:"position"`
					Points   string `json:"points"`
					Driver   struct {
						DriverID        string `json:"driverId"`
						PermanentNumber string `json:"permanentNumber"`
						Code            string `json:"code"`
						GivenName       string `json:"givenName"`
						FamilyName      string `json:"familyName"`
					} `json:"Driver"`
					Constructor struct {
						ConstructorID string `json:"constructorId"`
						Name          string `json:"name"`
					} `json:"Constructor"`
					Status string `json:"status"`
					Time   *struct {
						Millis string `json:"millis"`
						Time   string `json:"time"`
					} `json:"Time"`
				} `json:"Results"`
			} `json:"Races"`
		} `json:"RaceTable"`
	} `json:"MRData"`
}

type f1SprintResponse struct {
	MRData struct {
		Total     string `json:"total"`
		Limit     string `json:"limit"`
		Offset    string `json:"offset"`
		RaceTable struct {
			Season string `json:"season"`
			Races  []struct {
				Season        string `json:"season"`
				Round         string `json:"round"`
				SprintResults []struct {
					Number   string `json:"number"`
					Position string `json:"position"`
					Points   string `json:"points"`
					Driver   struct {
						DriverID        string `json:"driverId"`
						PermanentNumber string `json:"permanentNumber"`
						Code            string `json:"code"`
						GivenName       string `json:"givenName"`
						FamilyName      string `json:"familyName"`
					} `json:"Driver"`
					Constructor struct {
						ConstructorID string `json:"constructorId"`
						Name          string `json:"name"`
					} `json:"Constructor"`
					Status string `json:"status"`
					Time   *struct {
						Millis string `json:"millis"`
						Time   string `json:"time"`
					} `json:"Time"`
				} `json:"SprintResults"`
			} `json:"Races"`
		} `json:"RaceTable"`
	} `json:"MRData"`
}

func parseF1DateTime(dateStr, timeStr string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)
	timeStr = strings.TrimSpace(timeStr)
	if dateStr == "" {
		return time.Time{}, fmt.Errorf("empty date string")
	}
	if timeStr == "" {
		return time.Parse("2006-01-02T15:04:05Z", dateStr+"T12:00:00Z")
	}
	if !strings.HasSuffix(timeStr, "Z") {
		timeStr += "Z"
	}
	return time.Parse(time.RFC3339, dateStr+"T"+timeStr)
}

// SyncF1 fetches the current season Formula 1 calendar, sessions, and race results from Jolpica.
func SyncF1(ctx context.Context, queries *db.Queries) error {
	client := &http.Client{Timeout: 30 * time.Second}

	teamCache := make(map[string]int32)
	driverCache := make(map[string]int32)
	roundEventMap := make(map[int]int32)

	// 1. Fetch Season Calendar
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f1ScheduleURL, nil)
	if err != nil {
		return fmt.Errorf("create schedule request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch f1 schedule: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("fetch f1 schedule status %d: %s", resp.StatusCode, string(body))
	}

	var scheduleData f1ScheduleResponse
	if err := json.NewDecoder(resp.Body).Decode(&scheduleData); err != nil {
		return fmt.Errorf("decode f1 schedule json: %w", err)
	}

	races := scheduleData.MRData.RaceTable.Races
	seasonStr := scheduleData.MRData.RaceTable.Season
	seasonInt, _ := strconv.Atoi(seasonStr)
	if seasonInt == 0 {
		seasonInt = time.Now().Year()
	}

	log.Printf("[fetcher-f1] Found %d races for season %d", len(races), seasonInt)

	now := time.Now()

	for _, race := range races {
		roundInt, err := strconv.Atoi(race.Round)
		if err != nil {
			log.Printf("[fetcher-f1] Skipping race %q due to invalid round: %v", race.RaceName, err)
			continue
		}

		// A. Upsert Circuit
		circuitSlug := "f1-" + Slugify(race.Circuit.CircuitID)
		if len(circuitSlug) > 50 {
			circuitSlug = circuitSlug[:50]
		}
		if circuitSlug == "f1" || circuitSlug == "f1-" {
			circuitSlug = "f1-" + Slugify(race.Circuit.CircuitName)
			if len(circuitSlug) > 50 {
				circuitSlug = circuitSlug[:50]
			}
		}

		var latFloat, longFloat pgtype.Float8
		if lat, err := strconv.ParseFloat(race.Circuit.Location.Lat, 64); err == nil {
			latFloat = pgtype.Float8{Float64: lat, Valid: true}
		}
		if lon, err := strconv.ParseFloat(race.Circuit.Location.Long, 64); err == nil {
			longFloat = pgtype.Float8{Float64: lon, Valid: true}
		}

		circuitID, err := queries.UpsertCircuit(ctx, db.UpsertCircuitParams{
			CircuitSlug:       circuitSlug,
			CircuitName:       race.Circuit.CircuitName,
			CircuitLocality:   pgtype.Text{String: race.Circuit.Location.Locality, Valid: race.Circuit.Location.Locality != ""},
			CircuitCountry:    pgtype.Text{String: race.Circuit.Location.Country, Valid: race.Circuit.Location.Country != ""},
			CircuitLatitude:   latFloat,
			CircuitLongitude:  longFloat,
			CircuitExternalID: pgtype.Text{String: race.Circuit.CircuitID, Valid: race.Circuit.CircuitID != ""},
		})
		if err != nil {
			log.Printf("[fetcher-f1] Failed to upsert circuit %q: %v", race.Circuit.CircuitName, err)
			continue
		}

		// B. Parse Race Timestamp & Event Status
		raceStartsAt, err := parseF1DateTime(race.Date, race.Time)
		if err != nil {
			log.Printf("[fetcher-f1] Failed to parse race time for %q: %v", race.RaceName, err)
			continue
		}

		eventStatus := "scheduled"
		if raceStartsAt.Add(4 * time.Hour).Before(now) {
			eventStatus = "completed"
		}

		eventSlug := fmt.Sprintf("f1-%d-%s", seasonInt, Slugify(race.RaceName))
		if len(eventSlug) > 100 {
			eventSlug = eventSlug[:100]
		}

		eventID, err := queries.UpsertEvent(ctx, db.UpsertEventParams{
			SerieID:          "f1",
			CircuitID:        circuitID,
			EventSeason:      int32(seasonInt),
			EventRound:       int32(roundInt),
			EventSlug:        eventSlug,
			EventName:        race.RaceName,
			EventStartsAt:    pgtype.Timestamptz{Time: raceStartsAt, Valid: true},
			EventStatus:      eventStatus,
			EventOfficialUrl: pgtype.Text{String: race.URL, Valid: race.URL != ""},
			EventTicketUrl:   pgtype.Text{},
			EventExternalID:  pgtype.Text{String: race.Circuit.CircuitID, Valid: race.Circuit.CircuitID != ""},
		})
		if err != nil {
			log.Printf("[fetcher-f1] Failed to upsert event %q (round %d): %v", race.RaceName, roundInt, err)
			continue
		}

		roundEventMap[roundInt] = eventID

		// C. Upsert Weekend Sessions
		if err := queries.DeleteSessionsByEventID(ctx, eventID); err != nil {
			log.Printf("[fetcher-f1] Failed to delete existing sessions for event %d: %v", eventID, err)
		}

		addSession := func(kind, name string, s *f1SessionItem) {
			if s == nil || s.Date == "" {
				return
			}
			t, err := parseF1DateTime(s.Date, s.Time)
			if err != nil {
				return
			}
			_, _ = queries.InsertSession(ctx, db.InsertSessionParams{
				EventID:         eventID,
				SessionKind:     kind,
				SessionName:     name,
				SessionStartsAt: pgtype.Timestamptz{Time: t, Valid: true},
			})
		}

		addSession("practice", "Practice 1", race.FirstPractice)
		addSession("practice", "Practice 2", race.SecondPractice)
		addSession("practice", "Practice 3", race.ThirdPractice)
		addSession("qualifying", "Sprint Qualifying", race.SprintQualifying)
		if race.SprintQualifying == nil {
			addSession("qualifying", "Sprint Shootout", race.SprintShootout)
		}
		addSession("sprint", "Sprint", race.Sprint)
		addSession("qualifying", "Qualifying", race.Qualifying)
		// Main Grand Prix Race session
		_, _ = queries.InsertSession(ctx, db.InsertSessionParams{
			EventID:         eventID,
			SessionKind:     "race",
			SessionName:     "Grand Prix",
			SessionStartsAt: pgtype.Timestamptz{Time: raceStartsAt, Valid: true},
		})
	}

	// 2. Fetch Race Results
	if err := syncF1Results(ctx, client, queries, seasonInt, roundEventMap, teamCache, driverCache); err != nil {
		log.Printf("[fetcher-f1] Non-fatal results sync warning: %v", err)
	}

	// 3. Fetch Sprint Results
	if err := syncF1SprintResults(ctx, client, queries, seasonInt, roundEventMap, teamCache, driverCache); err != nil {
		log.Printf("[fetcher-f1] Non-fatal sprint sync warning: %v", err)
	}

	return nil
}

func syncF1Results(ctx context.Context, client *http.Client, queries *db.Queries, season int, roundEventMap map[int]int32, teamCache, driverCache map[string]int32) error {
	limit := 100
	offset := 0

	for {
		url := fmt.Sprintf("%s?limit=%d&offset=%d", f1ResultsURL, limit, offset)
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
			return fmt.Errorf("results status: %d", resp.StatusCode)
		}

		var resultsData f1ResultsResponse
		if err := json.NewDecoder(resp.Body).Decode(&resultsData); err != nil {
			resp.Body.Close()
			return err
		}
		resp.Body.Close()

		for _, race := range resultsData.MRData.RaceTable.Races {
			roundInt, err := strconv.Atoi(race.Round)
			if err != nil {
				continue
			}
			eventID := roundEventMap[roundInt]
			if eventID == 0 {
				row, err := queries.GetEventBySeasonRound(ctx, db.GetEventBySeasonRoundParams{
					SerieID:     "f1",
					EventSeason: int32(season),
					EventRound:  int32(roundInt),
				})
				if err != nil {
					continue
				}
				eventID = row.EventID
			}

			for _, res := range race.Results {
				pos, err := strconv.Atoi(res.Position)
				if err != nil || pos < 1 || pos > 3 {
					// Only top 3 positions per DB constraint
					continue
				}

				teamID, err := EnsureTeam(ctx, queries, "f1", res.Constructor.Name, res.Constructor.ConstructorID, teamCache)
				if err != nil {
					log.Printf("[fetcher-f1] Failed to ensure team %q: %v", res.Constructor.Name, err)
					continue
				}

				driverNum, _ := strconv.Atoi(res.Driver.PermanentNumber)
				if driverNum == 0 {
					driverNum, _ = strconv.Atoi(res.Number)
				}

				driverID, err := EnsureDriver(ctx, queries, "f1", teamID, res.Driver.GivenName, res.Driver.FamilyName, res.Driver.Code, driverNum, res.Driver.DriverID, driverCache)
				if err != nil {
					log.Printf("[fetcher-f1] Failed to ensure driver %s %s: %v", res.Driver.GivenName, res.Driver.FamilyName, err)
					continue
				}

				timeOrGap := ""
				if res.Time != nil && res.Time.Time != "" {
					timeOrGap = res.Time.Time
				} else if res.Status != "" {
					timeOrGap = res.Status
				}

				pts, _ := strconv.ParseFloat(res.Points, 64)

				_ = queries.UpsertResult(ctx, db.UpsertResultParams{
					EventID:         eventID,
					SessionType:     "race",
					ResultPosition:  int32(pos),
					DriverID:        driverID,
					TeamID:          teamID,
					ResultTimeOrGap: pgtype.Text{String: timeOrGap, Valid: timeOrGap != ""},
					ResultPoints:    pgtype.Float8{Float64: pts, Valid: true},
				})
			}
		}

		total, _ := strconv.Atoi(resultsData.MRData.Total)
		offset += limit
		if offset >= total || len(resultsData.MRData.RaceTable.Races) == 0 {
			break
		}
	}

	return nil
}

func syncF1SprintResults(ctx context.Context, client *http.Client, queries *db.Queries, season int, roundEventMap map[int]int32, teamCache, driverCache map[string]int32) error {
	limit := 100
	offset := 0

	for {
		url := fmt.Sprintf("%s?limit=%d&offset=%d", f1SprintURL, limit, offset)
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
			return fmt.Errorf("sprint results status: %d", resp.StatusCode)
		}

		var sprintData f1SprintResponse
		if err := json.NewDecoder(resp.Body).Decode(&sprintData); err != nil {
			resp.Body.Close()
			return err
		}
		resp.Body.Close()

		for _, race := range sprintData.MRData.RaceTable.Races {
			roundInt, err := strconv.Atoi(race.Round)
			if err != nil {
				continue
			}
			eventID := roundEventMap[roundInt]
			if eventID == 0 {
				row, err := queries.GetEventBySeasonRound(ctx, db.GetEventBySeasonRoundParams{
					SerieID:     "f1",
					EventSeason: int32(season),
					EventRound:  int32(roundInt),
				})
				if err != nil {
					continue
				}
				eventID = row.EventID
			}

			for _, res := range race.SprintResults {
				pos, err := strconv.Atoi(res.Position)
				if err != nil || pos < 1 || pos > 3 {
					continue
				}

				teamID, err := EnsureTeam(ctx, queries, "f1", res.Constructor.Name, res.Constructor.ConstructorID, teamCache)
				if err != nil {
					continue
				}

				driverNum, _ := strconv.Atoi(res.Driver.PermanentNumber)
				if driverNum == 0 {
					driverNum, _ = strconv.Atoi(res.Number)
				}

				driverID, err := EnsureDriver(ctx, queries, "f1", teamID, res.Driver.GivenName, res.Driver.FamilyName, res.Driver.Code, driverNum, res.Driver.DriverID, driverCache)
				if err != nil {
					continue
				}

				timeOrGap := ""
				if res.Time != nil && res.Time.Time != "" {
					timeOrGap = res.Time.Time
				} else if res.Status != "" {
					timeOrGap = res.Status
				}

				pts, _ := strconv.ParseFloat(res.Points, 64)

				_ = queries.UpsertResult(ctx, db.UpsertResultParams{
					EventID:         eventID,
					SessionType:     "sprint",
					ResultPosition:  int32(pos),
					DriverID:        driverID,
					TeamID:          teamID,
					ResultTimeOrGap: pgtype.Text{String: timeOrGap, Valid: timeOrGap != ""},
					ResultPoints:    pgtype.Float8{Float64: pts, Valid: true},
				})
			}
		}

		total, _ := strconv.Atoi(sprintData.MRData.Total)
		offset += limit
		if offset >= total || len(sprintData.MRData.RaceTable.Races) == 0 {
			break
		}
	}

	return nil
}
