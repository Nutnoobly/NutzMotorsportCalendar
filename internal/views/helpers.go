package views

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strings"
	"time"

	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	SiteName        = "Nutz Motorsport Calendar"
	DefaultSiteDesc = "Live countdowns, weekend session timetables, and race results for Formula 1 and MotoGP fans."
	DefaultBaseURL  = "https://nutzmotorsportcalendar.onrender.com"
)

// GetBaseURL returns the configured base URL from BASE_URL env var, or the production default.
func GetBaseURL() string {
	if u := os.Getenv("BASE_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return DefaultBaseURL
}

// DefaultOGImage returns the absolute URL to the default OpenGraph social card image.
func DefaultOGImage() string {
	return GetBaseURL() + "/static/og-banner.png"
}

// SEOMeta encapsulates page-level metadata, social tags, and structured data.
type SEOMeta struct {
	Title       string
	Description string
	Canonical   string
	OGImage     string
	OGType      string
	JSONLD      template.JS
}

// DefaultHomeMeta generates SEO metadata and WebSite JSON-LD for the homepage and series filters.
func DefaultHomeMeta(seriesFilter string) SEOMeta {
	baseURL := GetBaseURL()
	title := "F1 & MotoGP Race Calendar & Countdown"
	desc := DefaultSiteDesc
	canonical := baseURL + "/"

	if seriesFilter == "f1" {
		title = "Formula 1 2026 Calendar & Race Schedules"
		desc = "Complete 2026 Formula 1 race calendar, weekend session timetables, and live countdowns."
		canonical = baseURL + "/?series=f1"
	} else if seriesFilter == "motogp" {
		title = "MotoGP 2026 Calendar & Race Schedules"
		desc = "Complete 2026 MotoGP Grand Prix calendar, sprint & race schedules, and live countdowns."
		canonical = baseURL + "/?series=motogp"
	}

	schema := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "WebSite",
		"name":        SiteName,
		"url":         baseURL,
		"description": desc,
	}
	jsonBytes, _ := json.Marshal(schema)

	return SEOMeta{
		Title:       title,
		Description: desc,
		Canonical:   canonical,
		OGImage:     DefaultOGImage(),
		OGType:      "website",
		JSONLD:      template.JS(jsonBytes),
	}
}

// EventDetailMeta builds dynamic SEO tags and Schema.org SportsEvent JSON-LD for an event.
func EventDetailMeta(event db.GetEventBySlugRow) SEOMeta {
	baseURL := GetBaseURL()
	title := fmt.Sprintf("%s (%d Round %d)", event.EventName, event.EventSeason, event.EventRound)
	desc := fmt.Sprintf("Schedule, session timetables, circuit info, and results for the %s at %s.", event.EventName, event.CircuitName)
	canonical := fmt.Sprintf("%s/events/%s", baseURL, event.EventSlug)

	serieName := "Motorsport"
	switch strings.ToLower(event.SerieID) {
	case "f1":
		serieName = "Formula 1"
	case "motogp":
		serieName = "MotoGP"
	}

	schema := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "SportsEvent",
		"name":        event.EventName,
		"sport":       serieName,
		"eventStatus": "https://schema.org/EventScheduled",
		"location": map[string]any{
			"@type": "Place",
			"name":  event.CircuitName,
			"address": map[string]any{
				"@type":           "PostalAddress",
				"addressCountry":  TextString(event.CircuitCountry, ""),
				"addressLocality": TextString(event.CircuitLocality, ""),
			},
		},
	}

	if event.EventStartsAt.Valid {
		schema["startDate"] = event.EventStartsAt.Time.UTC().Format(time.RFC3339)
		schema["endDate"] = event.EventStartsAt.Time.UTC().Add(2 * time.Hour).Format(time.RFC3339)
	}

	if offUrl := TextString(event.EventOfficialUrl, ""); offUrl != "" {
		schema["organizer"] = map[string]any{
			"@type": "SportsOrganization",
			"name":  serieName,
			"url":   offUrl,
		}
	} else {
		schema["organizer"] = map[string]any{
			"@type": "SportsOrganization",
			"name":  serieName,
		}
	}

	jsonBytes, _ := json.Marshal(schema)

	return SEOMeta{
		Title:       title,
		Description: desc,
		Canonical:   canonical,
		OGImage:     DefaultOGImage(),
		OGType:      "article",
		JSONLD:      template.JS(jsonBytes),
	}
}


// TextString returns the string value if Valid, otherwise fallback.
func TextString(t pgtype.Text, fallback string) string {
	if t.Valid && t.String != "" {
		return t.String
	}
	return fallback
}

// FormatDate formats a pgtype.Timestamptz to "02 Jan 2006".
func FormatDate(t pgtype.Timestamptz) string {
	if !t.Valid {
		return "TBD"
	}
	return t.Time.Format("02 Jan 2006")
}

// FormatTimeUTC formats a pgtype.Timestamptz to "15:04 UTC".
func FormatTimeUTC(t pgtype.Timestamptz) string {
	if !t.Valid {
		return "TBD"
	}
	return t.Time.UTC().Format("15:04 UTC")
}

// FormatDateTime formats a pgtype.Timestamptz to "02 Jan 2006, 15:04 UTC".
func FormatDateTime(t pgtype.Timestamptz) string {
	if !t.Valid {
		return "TBD"
	}
	return t.Time.UTC().Format("02 Jan 2006, 15:04 UTC")
}

// FormatMonthYear formats a pgtype.Timestamptz to "January 2006".
func FormatMonthYear(t pgtype.Timestamptz) string {
	if !t.Valid {
		return "Unscheduled"
	}
	return t.Time.Format("January 2006")
}

// ISOTimestamp returns an RFC3339 timestamp string for client-side JS timezone conversions.
func ISOTimestamp(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339)
}

// SeriesBadgeClass returns styling classes depending on the championship series.
func SeriesBadgeClass(serieID string) string {
	switch serieID {
	case "f1":
		return "bg-red-500/10 text-red-600 dark:text-red-400 border border-red-500/20 dark:border-red-500/30"
	case "motogp":
		return "bg-sky-500/10 text-sky-600 dark:text-sky-400 border border-sky-500/20 dark:border-sky-500/30"
	default:
		return "bg-slate-500/10 text-slate-700 dark:text-slate-300 border border-slate-500/20 dark:border-slate-600"
	}
}

// StatusBadgeClass returns styling classes depending on event status.
func StatusBadgeClass(status string) string {
	switch status {
	case "completed":
		return "bg-emerald-500/10 text-emerald-400 border border-emerald-500/30"
	case "in_progress", "live":
		return "bg-amber-500/10 text-amber-400 border border-amber-500/30 animate-pulse"
	case "canceled", "cancelled":
		return "bg-rose-500/10 text-rose-400 border border-rose-500/30 line-through opacity-75"
	case "postponed":
		return "bg-orange-500/10 text-orange-400 border border-orange-500/30 opacity-75"
	default: // upcoming / scheduled
		return "bg-indigo-500/10 text-indigo-400 border border-indigo-500/30"
	}
}

// PositionBadgeClass returns podium highlight classes for top 3 finishes.
func PositionBadgeClass(pos int32) string {
	switch pos {
	case 1:
		return "bg-amber-400/20 text-amber-600 dark:text-amber-300 border-amber-400/50 font-bold"
	case 2:
		return "bg-slate-300/20 text-slate-700 dark:text-slate-200 border-slate-300/50 font-bold"
	case 3:
		return "bg-amber-700/20 text-amber-700 dark:text-amber-500 border-amber-700/50 font-bold"
	default:
		return "bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 border-slate-200 dark:border-slate-700"
	}
}

// FloatString formats float8 with 1 decimal place or fallback.
func FloatString(f pgtype.Float8) string {
	if !f.Valid {
		return "0"
	}
	return fmt.Sprintf("%.1f", f.Float64)
}

// GeneratePodiumSummary generates a readable top-3 summary from event results.
func GeneratePodiumSummary(p1, p2, p3 string) string {
	if p1 != "" && p2 != "" && p3 != "" {
		return fmt.Sprintf("%s took victory, followed by %s in 2nd and %s in 3rd.", p1, p2, p3)
	} else if p1 != "" {
		return fmt.Sprintf("%s claimed victory in the Grand Prix.", p1)
	}
	return ""
}

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

// FilterResultsBySession filters results by session_type ('race' or 'sprint').
func FilterResultsBySession(results []db.ListResultsByEventIDRow, sessionType string) []db.ListResultsByEventIDRow {
	var filtered []db.ListResultsByEventIDRow
	for _, r := range results {
		if strings.EqualFold(r.SessionType, sessionType) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}


