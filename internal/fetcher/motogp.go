package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	motogpBaseURL   = "https://api.motogp.pulselive.com/motogp/v1/results"
	motogpUserAgent = "NutzMotorsportCalendar/1.0"
)

type motogpSeason struct {
	ID      string `json:"id"`
	Year    int    `json:"year"`
	Current bool   `json:"current"`
}

type motogpCategory struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	LegacyID int    `json:"legacy_id"`
}

type motogpEvent struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	SponsoredName string `json:"sponsored_name"`
	ShortName     string `json:"short_name"`
	DateStart     string `json:"date_start"`
	DateEnd       string `json:"date_end"`
	Status        string `json:"status"`
	Test          bool   `json:"test"`
	Country       struct {
		ISO  string `json:"iso"`
		Name string `json:"name"`
	} `json:"country"`
	Circuit struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Place    string `json:"place"`
		Nation   string `json:"nation"`
		LegacyID int    `json:"legacy_id"`
	} `json:"circuit"`
}

type motogpSession struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Date   string `json:"date"`
	Number *int   `json:"number"`
	Status string `json:"status"`
}

type motogpClassificationResponse struct {
	Classification []struct {
		ID       string `json:"id"`
		Position int    `json:"position"`
		Rider    struct {
			ID       string `json:"id"`
			FullName string `json:"full_name"`
			Number   int    `json:"number"`
		} `json:"rider"`
		Team struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"team"`
		Time   string `json:"time"`
		Gap    struct {
			First string `json:"first"`
		} `json:"gap"`
		Points float64 `json:"points"`
		Status string  `json:"status"`
	} `json:"classification"`
}

func motogpGet(ctx context.Context, client *http.Client, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", motogpUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d from %s: %s", resp.StatusCode, url, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

// SyncMotoGP pulls current season MotoGP calendar, timetables, and race classifications.
func SyncMotoGP(ctx context.Context, queries *db.Queries) error {
	client := &http.Client{Timeout: 30 * time.Second}

	teamCache := make(map[string]int32)
	driverCache := make(map[string]int32)

	// 1. Fetch Seasons and pick the current season
	var seasons []motogpSeason
	if err := motogpGet(ctx, client, motogpBaseURL+"/seasons", &seasons); err != nil {
		return fmt.Errorf("fetch motogp seasons: %w", err)
	}

	var activeSeason *motogpSeason
	currentYear := time.Now().Year()
	for i := range seasons {
		if seasons[i].Current || seasons[i].Year == currentYear {
			activeSeason = &seasons[i]
			break
		}
	}
	if activeSeason == nil && len(seasons) > 0 {
		activeSeason = &seasons[0]
	}
	if activeSeason == nil {
		return fmt.Errorf("no active motogp season found")
	}

	log.Printf("[fetcher-motogp] Active season: %d (UUID: %s)", activeSeason.Year, activeSeason.ID)

	// 2. Fetch Categories for this season and find premier class (MotoGP™)
	var categories []motogpCategory
	catURL := fmt.Sprintf("%s/categories?seasonUuid=%s", motogpBaseURL, activeSeason.ID)
	if err := motogpGet(ctx, client, catURL, &categories); err != nil {
		return fmt.Errorf("fetch motogp categories: %w", err)
	}

	var premierCategory *motogpCategory
	for i := range categories {
		if categories[i].LegacyID == 3 || strings.Contains(strings.ToLower(categories[i].Name), "motogp") {
			premierCategory = &categories[i]
			break
		}
	}
	if premierCategory == nil && len(categories) > 0 {
		premierCategory = &categories[0]
	}
	if premierCategory == nil {
		return fmt.Errorf("premier MotoGP class category not found")
	}

	log.Printf("[fetcher-motogp] Premier category: %s (UUID: %s)", premierCategory.Name, premierCategory.ID)

	// 3. Fetch Events
	var allEvents []motogpEvent
	eventsURL := fmt.Sprintf("%s/events?seasonUuid=%s", motogpBaseURL, activeSeason.ID)
	if err := motogpGet(ctx, client, eventsURL, &allEvents); err != nil {
		return fmt.Errorf("fetch motogp events: %w", err)
	}

	// Filter out test events
	var officialEvents []motogpEvent
	for _, ev := range allEvents {
		if !ev.Test {
			officialEvents = append(officialEvents, ev)
		}
	}

	// Sort events chronologically by DateStart
	sort.Slice(officialEvents, func(i, j int) bool {
		return officialEvents[i].DateStart < officialEvents[j].DateStart
	})

	log.Printf("[fetcher-motogp] Found %d official Grand Prix events for %d", len(officialEvents), activeSeason.Year)

	now := time.Now()

	for idx, ev := range officialEvents {
		round := idx + 1

		// A. Upsert Circuit
		circuitSlug := "motogp-" + Slugify(ev.Circuit.Name)
		if len(circuitSlug) > 50 {
			circuitSlug = circuitSlug[:50]
		}
		if circuitSlug == "motogp" || circuitSlug == "motogp-" {
			circuitSlug = "motogp-" + Slugify(ev.Circuit.Place)
			if len(circuitSlug) > 50 {
				circuitSlug = circuitSlug[:50]
			}
		}

		country := ev.Circuit.Nation
		if country == "" {
			country = ev.Country.Name
		}

		circuitID, err := queries.UpsertCircuit(ctx, db.UpsertCircuitParams{
			CircuitSlug:       circuitSlug,
			CircuitName:       ev.Circuit.Name,
			CircuitLocality:   pgtype.Text{String: ev.Circuit.Place, Valid: ev.Circuit.Place != ""},
			CircuitCountry:    pgtype.Text{String: country, Valid: country != ""},
			CircuitLatitude:   pgtype.Float8{},
			CircuitLongitude:  pgtype.Float8{},
			CircuitExternalID: pgtype.Text{String: ev.Circuit.ID, Valid: ev.Circuit.ID != ""},
		})
		if err != nil {
			log.Printf("[fetcher-motogp] Failed to upsert circuit %q: %v", ev.Circuit.Name, err)
			continue
		}

		// B. Parse StartsAt & Status
		var startsAt time.Time
		if parsed, err := time.Parse("2006-01-02", ev.DateStart); err == nil {
			startsAt = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 12, 0, 0, 0, time.UTC)
		} else {
			startsAt = now
		}

		eventStatus := "scheduled"
		switch strings.ToUpper(strings.TrimSpace(ev.Status)) {
		case "FINISHED":
			eventStatus = "completed"
		case "CANCELLED", "CANCELED":
			eventStatus = "cancelled"
		case "POSTPONED":
			eventStatus = "postponed"
		default:
			if startsAt.Add(72 * time.Hour).Before(now) {
				eventStatus = "completed"
			}
		}

		slugBase := ev.ShortName
		if slugBase == "" {
			slugBase = ev.Name
		}
		eventSlug := fmt.Sprintf("motogp-%d-%s", activeSeason.Year, Slugify(slugBase))
		if len(eventSlug) > 100 {
			eventSlug = eventSlug[:100]
		}

		eventName := ev.Name
		if eventName == "" {
			eventName = ev.SponsoredName
		}

		eventID, err := queries.UpsertEvent(ctx, db.UpsertEventParams{
			SerieID:          "motogp",
			CircuitID:        circuitID,
			EventSeason:      int32(activeSeason.Year),
			EventRound:       int32(round),
			EventSlug:        eventSlug,
			EventName:        eventName,
			EventStartsAt:    pgtype.Timestamptz{Time: startsAt, Valid: true},
			EventStatus:      eventStatus,
			EventOfficialUrl: pgtype.Text{},
			EventTicketUrl:   pgtype.Text{},
			EventExternalID:  pgtype.Text{String: ev.ID, Valid: ev.ID != ""},
		})
		if err != nil {
			log.Printf("[fetcher-motogp] Failed to upsert event %q: %v", eventName, err)
			continue
		}

		// C. Fetch Sessions for Event
		sessionsURL := fmt.Sprintf("%s/sessions?eventUuid=%s&categoryUuid=%s", motogpBaseURL, ev.ID, premierCategory.ID)
		var sessions []motogpSession
		if err := motogpGet(ctx, client, sessionsURL, &sessions); err != nil {
			log.Printf("[fetcher-motogp] Could not load sessions for %s: %v", ev.Name, err)
			continue
		}

		if len(sessions) > 0 {
			_ = queries.DeleteSessionsByEventID(ctx, eventID)

			// Sort sessions chronologically so final sessions (e.g. Q2) overwrite preliminary ones (e.g. Q1)
			sort.Slice(sessions, func(i, j int) bool {
				return sessions[i].Date < sessions[j].Date
			})

			var raceSessionTime time.Time

			for _, sess := range sessions {
				sessKind := "other"
				sessName := sess.Type
				numStr := ""
				if sess.Number != nil && *sess.Number > 0 {
					numStr = fmt.Sprintf(" %d", *sess.Number)
				}

				switch strings.ToUpper(sess.Type) {
				case "FP":
					sessKind = "practice"
					sessName = "Free Practice" + numStr
				case "PR":
					sessKind = "practice"
					sessName = "Practice" + numStr
				case "Q":
					sessKind = "qualifying"
					sessName = "Qualifying" + numStr
				case "SPR":
					sessKind = "sprint"
					sessName = "Sprint"
				case "WUP":
					sessKind = "other"
					sessName = "Warm Up"
				case "RAC":
					sessKind = "race"
					sessName = "Grand Prix Race"
				}

				sessTime, err := time.Parse(time.RFC3339, sess.Date)
				if err != nil {
					continue
				}

				if strings.ToUpper(sess.Type) == "RAC" {
					raceSessionTime = sessTime
				}

				_, _ = queries.InsertSession(ctx, db.InsertSessionParams{
					EventID:         eventID,
					SessionKind:     sessKind,
					SessionName:     sessName,
					SessionStartsAt: pgtype.Timestamptz{Time: sessTime, Valid: true},
				})

				// D. If session is RAC, SPR, Q, FP, or PR and completed, fetch classification
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

							teamID, err := EnsureTeam(ctx, queries, "motogp", row.Team.Name, row.Team.ID, teamCache)
							if err != nil {
								continue
							}

							nameParts := strings.Split(strings.TrimSpace(row.Rider.FullName), " ")
							firstName := ""
							lastName := row.Rider.FullName
							if len(nameParts) > 1 {
								firstName = nameParts[0]
								lastName = strings.Join(nameParts[1:], " ")
							}

							driverID, err := EnsureDriver(ctx, queries, "motogp", teamID, firstName, lastName, "", row.Rider.Number, row.Rider.ID, driverCache)
							if err != nil {
								continue
							}

							timeOrGap := row.Time
							if timeOrGap == "" && row.Gap.First != "" {
								timeOrGap = "+" + row.Gap.First
							}
							if timeOrGap == "" {
								timeOrGap = row.Status
							}

							_ = queries.UpsertResult(ctx, db.UpsertResultParams{
								EventID:         eventID,
								SessionType:     sessionType,
								ResultPosition:  int32(row.Position),
								DriverID:        driverID,
								TeamID:          teamID,
								ResultTimeOrGap: pgtype.Text{String: timeOrGap, Valid: timeOrGap != ""},
								ResultPoints:    pgtype.Float8{Float64: row.Points, Valid: true},
							})
						}
					}
				}
			}

			// If we found a real race session start time, update the event starts_at
			if !raceSessionTime.IsZero() {
				_, _ = queries.UpsertEvent(ctx, db.UpsertEventParams{
					SerieID:          "motogp",
					CircuitID:        circuitID,
					EventSeason:      int32(activeSeason.Year),
					EventRound:       int32(round),
					EventSlug:        eventSlug,
					EventName:        eventName,
					EventStartsAt:    pgtype.Timestamptz{Time: raceSessionTime, Valid: true},
					EventStatus:      eventStatus,
					EventOfficialUrl: pgtype.Text{},
					EventTicketUrl:   pgtype.Text{},
					EventExternalID:  pgtype.Text{String: ev.ID, Valid: ev.ID != ""},
				})
			}
		}
	}

	return nil
}
