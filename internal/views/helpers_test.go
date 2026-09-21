package views_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/db"
	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/views"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestGenerateRaceSummary(t *testing.T) {
	t.Run("full podium with gap and sprint winner", func(t *testing.T) {
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
			{
				SessionType:     "sprint",
				ResultPosition:  1,
				DriverFirstName: "Max",
				DriverLastName:  "Verstappen",
				TeamName:        "Red Bull Racing",
				ResultTimeOrGap: pgtype.Text{String: "28:10.000", Valid: true},
			},
		}

		summary := views.GenerateRaceSummary(results)
		expected := "Max Verstappen took victory in the Grand Prix, followed by Lando Norris in 2nd (+2.456s) and Charles Leclerc in 3rd. Max Verstappen also claimed victory in Saturday's Sprint."
		if summary != expected {
			t.Errorf("expected %q, got %q", expected, summary)
		}
	})

	t.Run("full podium without sprint winner", func(t *testing.T) {
		results := []db.ListResultsByEventIDRow{
			{
				SessionType:     "race",
				ResultPosition:  1,
				DriverFirstName: "Lewis",
				DriverLastName:  "Hamilton",
				TeamName:        "Mercedes",
			},
			{
				SessionType:     "race",
				ResultPosition:  2,
				DriverFirstName: "George",
				DriverLastName:  "Russell",
				TeamName:        "Mercedes",
				ResultTimeOrGap: pgtype.Text{String: "+1.200s", Valid: true},
			},
			{
				SessionType:     "race",
				ResultPosition:  3,
				DriverFirstName: "Oscar",
				DriverLastName:  "Piastri",
				TeamName:        "McLaren",
			},
		}

		summary := views.GenerateRaceSummary(results)
		expected := "Lewis Hamilton took victory in the Grand Prix, followed by George Russell in 2nd (+1.200s) and Oscar Piastri in 3rd."
		if summary != expected {
			t.Errorf("expected %q, got %q", expected, summary)
		}
	})

	t.Run("winner only", func(t *testing.T) {
		results := []db.ListResultsByEventIDRow{
			{
				SessionType:     "race",
				ResultPosition:  1,
				DriverFirstName: "Carlos",
				DriverLastName:  "Sainz",
				TeamName:        "Ferrari",
			},
		}

		summary := views.GenerateRaceSummary(results)
		expected := "Carlos Sainz took victory in the Grand Prix."
		if summary != expected {
			t.Errorf("expected %q, got %q", expected, summary)
		}
	})

	t.Run("empty results", func(t *testing.T) {
		summary := views.GenerateRaceSummary(nil)
		if summary != "" {
			t.Errorf("expected empty string, got %q", summary)
		}
	})
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
			SessionType:     "race",
			ResultPosition:  2,
			DriverFirstName: "Lando",
			DriverLastName:  "Norris",
		},
		{
			SessionType:     "sprint",
			ResultPosition:  1,
			DriverFirstName: "Oscar",
			DriverLastName:  "Piastri",
		},
	}

	t.Run("race winner", func(t *testing.T) {
		raceWinner := views.GetSessionWinner("race", results)
		if raceWinner == nil || raceWinner.DriverLastName != "Verstappen" {
			t.Errorf("expected Verstappen as race winner")
		}
	})

	t.Run("sprint winner", func(t *testing.T) {
		sprintWinner := views.GetSessionWinner("sprint", results)
		if sprintWinner == nil || sprintWinner.DriverLastName != "Piastri" {
			t.Errorf("expected Piastri as sprint winner")
		}
	})

	t.Run("case insensitive", func(t *testing.T) {
		raceWinner := views.GetSessionWinner("Race", results)
		if raceWinner == nil || raceWinner.DriverLastName != "Verstappen" {
			t.Errorf("expected Verstappen for 'Race'")
		}
	})

	t.Run("practice returns nil when missing", func(t *testing.T) {
		practiceWinner := views.GetSessionWinner("practice", results)
		if practiceWinner != nil {
			t.Errorf("expected nil for practice winner")
		}
	})

	t.Run("qualifying and practice winners when present", func(t *testing.T) {
		extendedResults := append(results,
			db.ListResultsByEventIDRow{
				SessionType:     "qualifying",
				ResultPosition:  1,
				DriverFirstName: "George",
				DriverLastName:  "Russell",
			},
			db.ListResultsByEventIDRow{
				SessionType:     "practice",
				ResultPosition:  1,
				DriverFirstName: "Charles",
				DriverLastName:  "Leclerc",
			},
		)

		qualiWinner := views.GetSessionWinner("qualifying", extendedResults)
		if qualiWinner == nil || qualiWinner.DriverLastName != "Russell" {
			t.Errorf("expected Russell as qualifying pole winner")
		}

		practiceWinner := views.GetSessionWinner("practice", extendedResults)
		if practiceWinner == nil || practiceWinner.DriverLastName != "Leclerc" {
			t.Errorf("expected Leclerc as practice fastest driver")
		}
	})
}

func TestGetSessionStatus(t *testing.T) {
	now := time.Now()

	t.Run("completed event overrides timestamp", func(t *testing.T) {
		status := views.GetSessionStatus(pgtype.Timestamptz{Time: now.Add(24 * time.Hour), Valid: true}, "completed")
		if status != "Completed" {
			t.Errorf("expected Completed, got %q", status)
		}
	})

	t.Run("cancelled event", func(t *testing.T) {
		status := views.GetSessionStatus(pgtype.Timestamptz{Time: now.Add(24 * time.Hour), Valid: true}, "cancelled")
		if status != "Cancelled" {
			t.Errorf("expected Cancelled, got %q", status)
		}
	})

	t.Run("postponed event", func(t *testing.T) {
		status := views.GetSessionStatus(pgtype.Timestamptz{Time: now.Add(24 * time.Hour), Valid: true}, "postponed")
		if status != "Postponed" {
			t.Errorf("expected Postponed, got %q", status)
		}
	})

	t.Run("future scheduled session", func(t *testing.T) {
		status := views.GetSessionStatus(pgtype.Timestamptz{Time: now.Add(2 * time.Hour), Valid: true}, "scheduled")
		if status != "Scheduled" {
			t.Errorf("expected Scheduled, got %q", status)
		}
	})

	t.Run("live session started 30 mins ago", func(t *testing.T) {
		status := views.GetSessionStatus(pgtype.Timestamptz{Time: now.Add(-30 * time.Minute), Valid: true}, "scheduled")
		if status != "Live Now" {
			t.Errorf("expected Live Now, got %q", status)
		}
	})

	t.Run("past session ended over 2 hours ago", func(t *testing.T) {
		status := views.GetSessionStatus(pgtype.Timestamptz{Time: now.Add(-3 * time.Hour), Valid: true}, "scheduled")
		if status != "Completed" {
			t.Errorf("expected Completed, got %q", status)
		}
	})
}

func TestSessionStatusBadgeClass(t *testing.T) {
	if views.SessionStatusBadgeClass("Completed") == "" {
		t.Errorf("expected non-empty badge class for Completed")
	}
	if views.SessionStatusBadgeClass("Live Now") == "" {
		t.Errorf("expected non-empty badge class for Live Now")
	}
	if views.SessionStatusBadgeClass("Cancelled") == "" {
		t.Errorf("expected non-empty badge class for Cancelled")
	}
	if views.SessionStatusBadgeClass("Scheduled") == "" {
		t.Errorf("expected non-empty badge class for Scheduled")
	}
}

func TestEventDetailRenderWithSummaryAndWinner(t *testing.T) {
	event := db.GetEventBySlugRow{
		EventID:       1,
		SerieID:       "f1",
		EventSeason:   2026,
		EventRound:    1,
		EventSlug:     "bahrain-gp-2026",
		EventName:     "Bahrain Grand Prix",
		CircuitName:   "Bahrain International Circuit",
		EventStatus:   "completed",
		EventStartsAt: pgtype.Timestamptz{Time: time.Now().Add(-48 * time.Hour), Valid: true},
	}

	sessions := []db.Session{
		{
			SessionKind:     "race",
			SessionName:     "Grand Prix Race",
			SessionStartsAt: pgtype.Timestamptz{Time: time.Now().Add(-48 * time.Hour), Valid: true},
		},
	}

	results := []db.ListResultsByEventIDRow{
		{
			SessionType:     "race",
			ResultPosition:  1,
			DriverFirstName: "Max",
			DriverLastName:  "Verstappen",
			TeamName:        "Red Bull Racing",
			ResultTimeOrGap: pgtype.Text{String: "1:31:44.742", Valid: true},
		},
		{
			SessionType:     "race",
			ResultPosition:  2,
			DriverFirstName: "Sergio",
			DriverLastName:  "Perez",
			TeamName:        "Red Bull Racing",
			ResultTimeOrGap: pgtype.Text{String: "+22.457s", Valid: true},
		},
		{
			SessionType:     "race",
			ResultPosition:  3,
			DriverFirstName: "Carlos",
			DriverLastName:  "Sainz",
			TeamName:        "Ferrari",
			ResultTimeOrGap: pgtype.Text{String: "+25.110s", Valid: true},
		},
	}

	var buf bytes.Buffer
	component := views.EventDetail(event, sessions, results)
	err := component.Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}

	html := buf.String()
	if !strings.Contains(html, "Official Race Summary") {
		t.Errorf("expected 'Official Race Summary' banner in rendered HTML")
	}
	if !strings.Contains(html, "Max Verstappen took victory in the Grand Prix") {
		t.Errorf("expected summary text in rendered HTML")
	}
	if !strings.Contains(html, "🏆 Winner:") || !strings.Contains(html, "Max Verstappen") {
		t.Errorf("expected session winner badge in rendered HTML")
	}
	if !strings.Contains(html, "Completed") {
		t.Errorf("expected 'Completed' session status badge in rendered HTML")
	}
}

func TestEventDetailRenderWithQualifyingAndPracticeBadges(t *testing.T) {
	event := db.GetEventBySlugRow{
		EventID:       10,
		SerieID:       "f1",
		EventSeason:   2026,
		EventRound:    1,
		EventSlug:     "australian-gp-2026",
		EventName:     "Australian Grand Prix",
		CircuitName:   "Albert Park Circuit",
		EventStatus:   "completed",
		EventStartsAt: pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true},
	}

	sessions := []db.Session{
		{
			SessionKind:     "practice",
			SessionName:     "Practice 1",
			SessionStartsAt: pgtype.Timestamptz{Time: time.Now().Add(-48 * time.Hour), Valid: true},
		},
		{
			SessionKind:     "qualifying",
			SessionName:     "Qualifying",
			SessionStartsAt: pgtype.Timestamptz{Time: time.Now().Add(-30 * time.Hour), Valid: true},
		},
		{
			SessionKind:     "race",
			SessionName:     "Grand Prix Race",
			SessionStartsAt: pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true},
		},
	}

	results := []db.ListResultsByEventIDRow{
		{
			SessionType:     "practice",
			ResultPosition:  1,
			DriverFirstName: "Charles",
			DriverLastName:  "Leclerc",
			TeamName:        "Ferrari",
		},
		{
			SessionType:     "qualifying",
			ResultPosition:  1,
			DriverFirstName: "George",
			DriverLastName:  "Russell",
			TeamName:        "Mercedes",
		},
		{
			SessionType:     "race",
			ResultPosition:  1,
			DriverFirstName: "Lewis",
			DriverLastName:  "Hamilton",
			TeamName:        "Ferrari",
		},
	}

	var buf bytes.Buffer
	component := views.EventDetail(event, sessions, results)
	err := component.Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}

	html := buf.String()
	if !strings.Contains(html, "⏱️ Fastest:") || !strings.Contains(html, "Charles Leclerc") {
		t.Errorf("expected '⏱️ Fastest: Charles Leclerc' in rendered HTML")
	}
	if !strings.Contains(html, "🎯 Pole:") || !strings.Contains(html, "George Russell") {
		t.Errorf("expected '🎯 Pole: George Russell' in rendered HTML")
	}
	if !strings.Contains(html, "🏆 Winner:") || !strings.Contains(html, "Lewis Hamilton") {
		t.Errorf("expected '🏆 Winner: Lewis Hamilton' in rendered HTML")
	}
}

func TestFilterResultsBySession(t *testing.T) {
	results := []db.ListResultsByEventIDRow{
		{SessionType: "race", ResultPosition: 1, DriverFirstName: "Max"},
		{SessionType: "race", ResultPosition: 2, DriverFirstName: "Lando"},
		{SessionType: "sprint", ResultPosition: 1, DriverFirstName: "Oscar"},
		{SessionType: "sprint", ResultPosition: 2, DriverFirstName: "George"},
	}

	races := views.FilterResultsBySession(results, "race")
	if len(races) != 2 || races[0].DriverFirstName != "Max" || races[1].DriverFirstName != "Lando" {
		t.Errorf("unexpected race results: %+v", races)
	}

	sprints := views.FilterResultsBySession(results, "sprint")
	if len(sprints) != 2 || sprints[0].DriverFirstName != "Oscar" || sprints[1].DriverFirstName != "George" {
		t.Errorf("unexpected sprint results: %+v", sprints)
	}

	empty := views.FilterResultsBySession(results, "qualifying")
	if len(empty) != 0 {
		t.Errorf("expected 0 results, got %d", len(empty))
	}
}

func TestEventDetailRenderSprintWeekendSplit(t *testing.T) {
	event := db.GetEventBySlugRow{
		EventID:       2,
		SerieID:       "f1",
		EventSeason:   2026,
		EventRound:    2,
		EventSlug:     "chinese-gp-2026",
		EventName:     "Chinese Grand Prix",
		CircuitName:   "Shanghai International Circuit",
		EventStatus:   "completed",
		EventStartsAt: pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true},
	}

	sessions := []db.Session{
		{
			SessionKind:     "sprint",
			SessionName:     "Sprint",
			SessionStartsAt: pgtype.Timestamptz{Time: time.Now().Add(-28 * time.Hour), Valid: true},
		},
		{
			SessionKind:     "race",
			SessionName:     "Grand Prix Race",
			SessionStartsAt: pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true},
		},
	}

	results := []db.ListResultsByEventIDRow{
		{
			SessionType:     "race",
			ResultPosition:  1,
			DriverFirstName: "Andrea Kimi",
			DriverLastName:  "Antonelli",
			TeamName:        "Mercedes",
			ResultTimeOrGap: pgtype.Text{String: "1:33:15.607", Valid: true},
		},
		{
			SessionType:     "race",
			ResultPosition:  2,
			DriverFirstName: "George",
			DriverLastName:  "Russell",
			TeamName:        "Mercedes",
			ResultTimeOrGap: pgtype.Text{String: "+5.515", Valid: true},
		},
		{
			SessionType:     "sprint",
			ResultPosition:  1,
			DriverFirstName: "George",
			DriverLastName:  "Russell",
			TeamName:        "Mercedes",
			ResultTimeOrGap: pgtype.Text{String: "33:38.998", Valid: true},
		},
		{
			SessionType:     "sprint",
			ResultPosition:  2,
			DriverFirstName: "Charles",
			DriverLastName:  "Leclerc",
			TeamName:        "Ferrari",
			ResultTimeOrGap: pgtype.Text{String: "+0.674", Valid: true},
		},
	}

	var buf bytes.Buffer
	component := views.EventDetail(event, sessions, results)
	err := component.Render(context.Background(), &buf)
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}

	html := buf.String()

	// Verify split sub-sections exist
	if !strings.Contains(html, "id=\"results-race-group\"") {
		t.Errorf("expected 'results-race-group' container in HTML")
	}
	if !strings.Contains(html, "id=\"results-sprint-group\"") {
		t.Errorf("expected 'results-sprint-group' container in HTML")
	}

	// Verify section headers
	if !strings.Contains(html, "Grand Prix Race") {
		t.Errorf("expected 'Grand Prix Race' header")
	}
	if !strings.Contains(html, "Sprint Race") {
		t.Errorf("expected 'Sprint Race' header")
	}

	// Verify interactive tab switcher controls
	if !strings.Contains(html, "id=\"tab-btn-all\"") {
		t.Errorf("expected tab-btn-all button")
	}
	if !strings.Contains(html, "id=\"tab-btn-race\"") {
		t.Errorf("expected tab-btn-race button")
	}
	if !strings.Contains(html, "id=\"tab-btn-sprint\"") {
		t.Errorf("expected tab-btn-sprint button")
	}
	if !strings.Contains(html, "switchResultsTab") {
		t.Errorf("expected switchResultsTab script")
	}
}

func TestSeriesBadgeClass(t *testing.T) {
	f1Class := views.SeriesBadgeClass("f1")
	if !strings.Contains(f1Class, "text-red-600") || !strings.Contains(f1Class, "dark:text-red-400") {
		t.Errorf("expected light/dark red classes for f1, got %q", f1Class)
	}

	motogpClass := views.SeriesBadgeClass("motogp")
	if !strings.Contains(motogpClass, "text-sky-600") || !strings.Contains(motogpClass, "dark:text-sky-400") {
		t.Errorf("expected light/dark sky classes for motogp, got %q", motogpClass)
	}

	defaultClass := views.SeriesBadgeClass("other")
	if !strings.Contains(defaultClass, "text-slate-700") || !strings.Contains(defaultClass, "dark:text-slate-300") {
		t.Errorf("expected slate classes for default, got %q", defaultClass)
	}
}

func TestFindNextEvent(t *testing.T) {
	now := time.Now()

	t.Run("empty list returns nil", func(t *testing.T) {
		res := views.FindNextEvent(nil)
		if res != nil {
			t.Errorf("expected nil for empty slice, got %+v", res)
		}
	})

	t.Run("mixed list selects closest upcoming event", func(t *testing.T) {
		events := []db.ListUpcomingEventsRow{
			{
				EventID:       1,
				SerieID:       "motogp",
				EventName:     "Indonesian GP",
				EventStartsAt: pgtype.Timestamptz{Time: now.Add(48 * time.Hour), Valid: true},
				EventStatus:   "scheduled",
			},
			{
				EventID:       2,
				SerieID:       "f1",
				EventName:     "Singapore GP",
				EventStartsAt: pgtype.Timestamptz{Time: now.Add(96 * time.Hour), Valid: true},
				EventStatus:   "scheduled",
			},
		}

		next := views.FindNextEvent(events)
		if next == nil || next.EventID != 1 || next.SerieID != "motogp" {
			t.Fatalf("expected closest MotoGP event (id 1), got %+v", next)
		}
	})

	t.Run("f1 only list selects closest f1 event", func(t *testing.T) {
		f1Events := []db.ListUpcomingEventsRow{
			{
				EventID:       2,
				SerieID:       "f1",
				EventName:     "Singapore GP",
				EventStartsAt: pgtype.Timestamptz{Time: now.Add(96 * time.Hour), Valid: true},
				EventStatus:   "scheduled",
			},
			{
				EventID:       3,
				SerieID:       "f1",
				EventName:     "United States GP",
				EventStartsAt: pgtype.Timestamptz{Time: now.Add(240 * time.Hour), Valid: true},
				EventStatus:   "scheduled",
			},
		}

		next := views.FindNextEvent(f1Events)
		if next == nil || next.EventID != 2 || next.SerieID != "f1" {
			t.Fatalf("expected closest F1 event (id 2), got %+v", next)
		}
	})

	t.Run("motogp only list selects closest motogp event", func(t *testing.T) {
		motogpEvents := []db.ListUpcomingEventsRow{
			{
				EventID:       1,
				SerieID:       "motogp",
				EventName:     "Indonesian GP",
				EventStartsAt: pgtype.Timestamptz{Time: now.Add(48 * time.Hour), Valid: true},
				EventStatus:   "scheduled",
			},
			{
				EventID:       4,
				SerieID:       "motogp",
				EventName:     "Japanese GP",
				EventStartsAt: pgtype.Timestamptz{Time: now.Add(120 * time.Hour), Valid: true},
				EventStatus:   "scheduled",
			},
		}

		next := views.FindNextEvent(motogpEvents)
		if next == nil || next.EventID != 1 || next.SerieID != "motogp" {
			t.Fatalf("expected closest MotoGP event (id 1), got %+v", next)
		}
	})

	t.Run("skips cancelled events for scheduled upcoming", func(t *testing.T) {
		events := []db.ListUpcomingEventsRow{
			{
				EventID:       10,
				SerieID:       "f1",
				EventName:     "Cancelled GP",
				EventStartsAt: pgtype.Timestamptz{Time: now.Add(24 * time.Hour), Valid: true},
				EventStatus:   "cancelled",
			},
			{
				EventID:       11,
				SerieID:       "f1",
				EventName:     "Active GP",
				EventStartsAt: pgtype.Timestamptz{Time: now.Add(72 * time.Hour), Valid: true},
				EventStatus:   "scheduled",
			},
		}

		next := views.FindNextEvent(events)
		if next == nil || next.EventID != 11 {
			t.Fatalf("expected non-cancelled event 11, got %+v", next)
		}
	})

	t.Run("identifies live session in progress", func(t *testing.T) {
		events := []db.ListUpcomingEventsRow{
			{
				EventID:       20,
				SerieID:       "f1",
				EventName:     "Live GP",
				EventStartsAt: pgtype.Timestamptz{Time: now.Add(-1 * time.Hour), Valid: true},
				EventStatus:   "scheduled",
			},
			{
				EventID:       21,
				SerieID:       "f1",
				EventName:     "Next GP",
				EventStartsAt: pgtype.Timestamptz{Time: now.Add(100 * time.Hour), Valid: true},
				EventStatus:   "scheduled",
			},
		}

		next := views.FindNextEvent(events)
		if next == nil || next.EventID != 20 {
			t.Fatalf("expected live event 20, got %+v", next)
		}
	})
}

func TestEventsSectionCountdownSeriesSync(t *testing.T) {
	now := time.Now()
	f1Event := db.ListUpcomingEventsRow{
		EventID:       101,
		SerieID:       "f1",
		EventRound:    18,
		EventSeason:   2026,
		EventSlug:     "f1-2026-singapore-gp",
		EventName:     "Singapore Grand Prix",
		CircuitName:   "Marina Bay Street Circuit",
		EventStartsAt: pgtype.Timestamptz{Time: now.Add(72 * time.Hour), Valid: true},
		EventStatus:   "scheduled",
	}
	motogpEvent := db.ListUpcomingEventsRow{
		EventID:       201,
		SerieID:       "motogp",
		EventRound:    15,
		EventSeason:   2026,
		EventSlug:     "motogp-2026-indonesian-gp",
		EventName:     "Indonesian Grand Prix",
		CircuitName:   "Mandalika International Street Circuit",
		EventStartsAt: pgtype.Timestamptz{Time: now.Add(24 * time.Hour), Valid: true},
		EventStatus:   "scheduled",
	}

	// 1. "ALL" selected: should contain telemetry HUD for Indonesian GP (closer) and wrap in #calendar-section
	var allBuf bytes.Buffer
	err := views.EventsSection("", []db.ListUpcomingEventsRow{motogpEvent, f1Event}).Render(context.Background(), &allBuf)
	if err != nil {
		t.Fatalf("failed to render ALL EventsSection: %v", err)
	}
	allHTML := allBuf.String()
	if !strings.Contains(allHTML, `id="calendar-section"`) {
		t.Errorf("expected calendar-section ID container")
	}
	if !strings.Contains(allHTML, `id="telemetry-hud"`) {
		t.Errorf("expected telemetry-hud inside calendar-section for HTMX swap")
	}
	if !strings.Contains(allHTML, "Indonesian Grand Prix") {
		t.Errorf("expected closest race (Indonesian GP) in ALL countdown widget")
	}

	// 2. "f1" selected: should contain telemetry HUD for Singapore Grand Prix
	var f1Buf bytes.Buffer
	err = views.EventsSection("f1", []db.ListUpcomingEventsRow{f1Event}).Render(context.Background(), &f1Buf)
	if err != nil {
		t.Fatalf("failed to render F1 EventsSection: %v", err)
	}
	f1HTML := f1Buf.String()
	if !strings.Contains(f1HTML, "Singapore Grand Prix") {
		t.Errorf("expected Singapore GP in F1 countdown widget")
	}
	if strings.Contains(f1HTML, "Indonesian Grand Prix") {
		t.Errorf("did not expect MotoGP in F1 countdown widget")
	}

	// 3. "motogp" selected: should contain telemetry HUD for Indonesian Grand Prix
	var motogpBuf bytes.Buffer
	err = views.EventsSection("motogp", []db.ListUpcomingEventsRow{motogpEvent}).Render(context.Background(), &motogpBuf)
	if err != nil {
		t.Fatalf("failed to render MotoGP EventsSection: %v", err)
	}
	motogpHTML := motogpBuf.String()
	if !strings.Contains(motogpHTML, "Indonesian Grand Prix") {
		t.Errorf("expected Indonesian GP in MotoGP countdown widget")
	}
	if strings.Contains(motogpHTML, "Singapore Grand Prix") {
		t.Errorf("did not expect F1 in MotoGP countdown widget")
	}
}


