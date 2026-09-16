package views

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

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
		return "bg-red-500/10 text-red-400 border border-red-500/30"
	case "motogp":
		return "bg-sky-500/10 text-sky-400 border border-sky-500/30"
	default:
		return "bg-slate-700/50 text-slate-300 border border-slate-600"
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
		return "bg-amber-400/20 text-amber-300 border-amber-400/50 font-bold"
	case 2:
		return "bg-slate-300/20 text-slate-200 border-slate-300/50 font-bold"
	case 3:
		return "bg-amber-700/20 text-amber-500 border-amber-700/50 font-bold"
	default:
		return "bg-slate-800 text-slate-400 border-slate-700"
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
