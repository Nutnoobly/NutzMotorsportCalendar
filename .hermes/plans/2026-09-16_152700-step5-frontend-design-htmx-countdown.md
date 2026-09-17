# Plan: Step 5 Frontend Design, HTMX Tab Swapping & Live Countdown Widget

## Goal
Design and implement dynamic client-side interactions for Step 5—transforming static series tabs into instant, transition-smoothed HTMX swaps and converting the placeholder timestamp into a multi-state motorsport telemetry countdown widget.

---

## Current Context & Assumptions

1. **Current State (Step 4 Complete)**:
   - Full calendar UI is currently rendered statically by `internal/views/home.templ` using Go stdlib `net/http` on port `8081`.
   - The hero section contains a static timestamp string placeholder (`#countdown-timer` with `data-utc`).
   - The series filter tabs (`All`, `F1`, `MotoGP`) are standard HTML anchor tags (`<a href="/?series=...">`) that cause full page refreshes.
   - `static/style.css` exists but is currently 0 bytes.
   - `internal/views/layout.templ` already loads Tailwind CSS and HTMX 2.0.4 via CDN.
   - `internal/web/server.go` correctly serves `/static/` via `http.FileServer`.

2. **Role Boundaries (`AGENTS.md`)**:
   - **Frontend (Templ, HTMX, Tailwind, JS)**: **Agent implements directly**.
   - **Backend (Go handlers, router)**: **Contract-First**. The agent specifies exact handler signatures and partial-rendering logic; the user applies it to `internal/web/handler.go` (or authorizes agent via imperative command).

3. **Core Design Decision: "Do we need to design front end for step 5?"**:
   - **Yes**. Without dedicated front-end design, HTMX swaps will cause jarring layout flashes and the countdown will look like a raw unstyled text counter. 
   - Step 5 requires deliberate UI design for:
     1. **Telemetry Countdown HUD**: 4-digit segmented blocks (Days, Hours, Minutes, Seconds) with status indicators for 4 race lifecycle states: *Upcoming*, *Race Week / Urgent (< 24h)*, *Live / In Progress*, and *Season Concluded*.
     2. **HTMX Tab Transitions**: Smooth CSS crossfades (`opacity` and `transform`) during `htmx-swapping` and `htmx-settling` so cards don't pop abruptly.
     3. **Component Decomposition**: Splitting `home.templ` into full-page vs partial (`EventsSection`) so HTMX only transfers the updated cards and tabs instead of the full HTML boilerplate.

---

## Architecture & Proposed Approach

```
┌────────────────────────────────────────────────────────────────────────┐
│ Visitor Browser                                                        │
│                                                                        │
│  [Hero / Telemetry Countdown Widget]  <-- static/countdown.js (1s tick)│
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐                           │
│  │ 04 DYS │ │ 18 HRS │ │ 32 MIN │ │ 15 SEC │                           │
│  └────────┘ └────────┘ └────────┘ └────────┘                           │
│                                                                        │
│  [Series Tabs]                                                         │
│  [ All ] [ 🏎️ F1 ] [ 🏍️ MotoGP ]                                      │
│      │                                                                 │
│      │ hx-get="/?series=f1"                                            │
│      │ hx-target="#calendar-section"                                   │
│      │ hx-swap="outerHTML transition:true"                             │
│      ▼                                                                 │
│  [#calendar-section (Event Cards + Tabs)]                              │
└──────┬─────────────────────────────────────────────────────────────────┘
       │ HTTP GET /?series=f1 (HX-Request: true)
       ▼
┌────────────────────────────────────────────────────────────────────────┐
│ Go Backend (internal/web/handler.go)                                   │
│                                                                        │
│  if r.Header.Get("HX-Request") == "true" {                             │
│      views.EventsSection(tab, events).Render(...) // ~3KB partial      │
│  } else {                                                              │
│      views.Home(tab, events).Render(...)          // ~12KB full page   │
│  }                                                                     │
└────────────────────────────────────────────────────────────────────────┘
```

1. **Scope of HTMX Swap**: We wrap both the tab bar and the event list in an outer `<section id="calendar-section">`. Swapping `outerHTML` updates the event cards **and** instantly updates the tab active colors (`bg-red-600` / `bg-sky-600`) in one atomic swap—without complex out-of-band swaps.
2. **Countdown Architecture**: Pure client-side JavaScript (`static/countdown.js`) reading `data-utc` ISO strings. Handles clock differences gracefully, transitions dynamically between 4 visual states, and cleans up intervals on page navigation.

---

## Step-by-Step Implementation Tasks

### Task 1: Design CSS Transitions & HTMX Loading States
**File**: `static/style.css`  
**Description**: Add smooth fade transitions for HTMX content swaps and styling for active indicators.

```css
/* static/style.css */

/* HTMX Swapping & Settling Crossfade Animation */
.htmx-swapping {
    opacity: 0;
    transition: opacity 120ms ease-out;
}

#calendar-section {
    transition: opacity 150ms ease-in;
}

/* Subtle loading indicator pulse when network latency occurs */
.htmx-request .tab-loading-indicator {
    display: inline-block;
}

.tab-loading-indicator {
    display: none;
}

/* Tabular numbers for countdown to prevent layout jitter on second ticks */
.tabular-nums {
    font-variant-numeric: tabular-nums;
}

/* Live pulsating indicator */
@keyframes pulse-dot {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.4; transform: scale(0.85); }
}

.animate-pulse-dot {
    animation: pulse-dot 1.5s cubic-bezier(0.4, 0, 0.6, 1) infinite;
}
```

---

### Task 2: Implement Live Countdown JS Ticker (`static/countdown.js`)
**File**: `static/countdown.js`  
**Description**: Pure vanilla JavaScript module that discovers `#countdown-widget`, reads the target UTC timestamp, calculates the breakdown, and updates digit blocks each second with 4 state variations.

```javascript
// static/countdown.js
(function () {
    let countdownInterval = null;

    function initCountdown() {
        if (countdownInterval) {
            clearInterval(countdownInterval);
            countdownInterval = null;
        }

        const widget = document.getElementById("countdown-widget");
        if (!widget) return;

        const targetUtc = widget.getAttribute("data-target-utc");
        if (!targetUtc) return;

        const targetTime = new Date(targetUtc).getTime();
        if (isNaN(targetTime)) return;

        const daysEl = document.getElementById("cd-days");
        const hoursEl = document.getElementById("cd-hours");
        const minsEl = document.getElementById("cd-mins");
        const secsEl = document.getElementById("cd-secs");
        const statusBadge = document.getElementById("cd-status-badge");
        const statusText = document.getElementById("cd-status-text");

        function update() {
            const now = new Date().getTime();
            const distance = targetTime - now;

            // State 1: Race In Progress or Concluded (started within last 3 hours)
            if (distance <= 0) {
                const elapsed = Math.abs(distance);
                const threeHoursMs = 3 * 60 * 60 * 1000;

                if (elapsed < threeHoursMs) {
                    // LIVE STATE
                    if (statusBadge) {
                        statusBadge.className = "inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-bold bg-red-500/20 text-red-400 border border-red-500/40 animate-pulse";
                        statusBadge.innerHTML = '<span class="w-2 h-2 rounded-full bg-red-500"></span> LIVE NOW';
                    }
                    if (statusText) statusText.textContent = "Grand Prix session underway";
                    if (daysEl) daysEl.textContent = "00";
                    if (hoursEl) hoursEl.textContent = "00";
                    if (minsEl) minsEl.textContent = "00";
                    if (secsEl) secsEl.textContent = "00";
                } else {
                    // COMPLETED STATE
                    if (statusBadge) {
                        statusBadge.className = "inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-slate-800 text-slate-400 border border-slate-700";
                        statusBadge.innerHTML = 'COMPLETED';
                    }
                    if (statusText) statusText.textContent = "Awaiting next round schedule";
                    clearInterval(countdownInterval);
                }
                return;
            }

            // Calculation
            const days = Math.floor(distance / (1000 * 60 * 60 * 24));
            const hours = Math.floor((distance % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
            const minutes = Math.floor((distance % (1000 * 60 * 60)) / (1000 * 60));
            const seconds = Math.floor((distance % (1000 * 60)) / 1000);

            // State 2: Race Imminent / Urgent (< 24 hours away)
            if (days === 0 && hours < 24) {
                if (statusBadge && !statusBadge.classList.contains("cd-imminent")) {
                    statusBadge.className = "cd-imminent inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-bold bg-amber-500/20 text-amber-400 border border-amber-500/40";
                    statusBadge.innerHTML = '<span class="w-1.5 h-1.5 rounded-full bg-amber-400 animate-pulse-dot"></span> RACE WEEKEND';
                }
            }

            if (daysEl) daysEl.textContent = String(days).padStart(2, "0");
            if (hoursEl) hoursEl.textContent = String(hours).padStart(2, "0");
            if (minsEl) minsEl.textContent = String(minutes).padStart(2, "0");
            if (secsEl) secsEl.textContent = String(seconds).padStart(2, "0");
        }

        update();
        countdownInterval = setInterval(update, 1000);
    }

    // Initialize on initial page load
    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", initCountdown);
    } else {
        initCountdown();
    }

    // Re-initialize after HTMX content settlement if widget was swapped
    document.body.addEventListener("htmx:afterSettle", function (evt) {
        if (evt.target && evt.target.querySelector && evt.target.querySelector("#countdown-widget")) {
            initCountdown();
        }
    });
})();
```

---

### Task 3: Decompose `internal/views/home.templ` with HTMX Attributes
**File**: `internal/views/home.templ`  
**Description**:
- Break `home.templ` into modular components: `Home` (full layout wrapper), `CountdownWidget` (telemetry UI), and `EventsSection` (HTMX swap target containing tabs, export links, and event list).
- Add `hx-get`, `hx-target="#calendar-section"`, `hx-swap="outerHTML"`, and `hx-push-url="true"` to tabs.

```templ
package views

import (
	"fmt"
	"github.com/Nutnoobly/NutzMotorsportCalendar/internal/db"
)

templ Home(activeTab string, events []db.ListUpcomingEventsRow) {
	@Layout("Home") {
		<div class="space-y-8">
			<!-- Hero Section & Telemetry Countdown -->
			<div class="relative overflow-hidden rounded-2xl border border-slate-800 bg-gradient-to-br from-slate-900 via-slate-950 to-slate-900 p-6 sm:p-8 shadow-2xl">
				<div class="absolute -right-12 -top-12 h-64 w-64 rounded-full bg-red-600/10 blur-3xl pointer-events-none"></div>
				<div class="relative z-10 flex flex-col lg:flex-row lg:items-center justify-between gap-8">
					<div class="max-w-xl">
						<div class="inline-flex items-center gap-2 px-3 py-1 rounded-full text-xs font-semibold bg-red-500/10 text-red-400 border border-red-500/20 mb-3">
							<span class="w-2 h-2 rounded-full bg-red-500 animate-ping"></span>
							Live Hub
						</div>
						<h1 class="text-3xl sm:text-4xl font-black tracking-tight text-white">
							Race Weekend Calendar
						</h1>
						<p class="mt-2 text-sm sm:text-base text-slate-400">
							Track upcoming Formula 1 and MotoGP Grands Prix, weekend timetables, and official podium results.
						</p>
					</div>

					<!-- Telemetry Countdown Widget -->
					if len(events) > 0 {
						@CountdownWidget(events[0])
					} else {
						<div class="rounded-xl border border-slate-800/80 bg-slate-900/60 p-5 text-center backdrop-blur-sm min-w-[280px]">
							<span class="text-xs uppercase tracking-wider text-slate-500 font-bold">Season Status</span>
							<p class="text-sm font-semibold text-slate-300 mt-1">No upcoming races scheduled</p>
						</div>
					}
				</div>
			</div>

			<!-- Dynamic HTMX Section: Tabs + Race List -->
			@EventsSection(activeTab, events)
		</div>
	}
}

templ CountdownWidget(ev db.ListUpcomingEventsRow) {
	<div
		id="countdown-widget"
		data-target-utc={ ISOTimestamp(ev.EventStartsAt) }
		class="rounded-xl border border-slate-800/80 bg-slate-900/80 p-5 backdrop-blur-sm shadow-xl min-w-[320px] sm:min-w-[360px]"
	>
		<div class="flex items-center justify-between gap-2 mb-3">
			<span id="cd-status-badge" class="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[11px] font-bold bg-slate-800 text-slate-300 border border-slate-700">
				<span class="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse-dot"></span> NEXT GRAND PRIX
			</span>
			<span class={ "px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider", SeriesBadgeClass(ev.SerieID) }>
				{ ev.SerieID }
			</span>
		</div>

		<h2 class="text-base font-bold text-white truncate max-w-[280px]" title={ ev.EventName }>
			{ ev.EventName }
		</h2>
		<p id="cd-status-text" class="text-xs text-slate-400 mt-0.5 truncate">
			{ FormatDate(ev.EventStartsAt) } • { ev.CircuitName }
		</p>

		<!-- 4-Digit Segmented Counter -->
		<div class="mt-4 grid grid-cols-4 gap-2 text-center font-mono">
			<div class="bg-slate-950/70 border border-slate-800 rounded-lg py-2 px-1">
				<span id="cd-days" class="block text-xl sm:text-2xl font-black text-white tabular-nums">--</span>
				<span class="block text-[10px] uppercase font-sans tracking-wider text-slate-400 font-semibold mt-0.5">Days</span>
			</div>
			<div class="bg-slate-950/70 border border-slate-800 rounded-lg py-2 px-1">
				<span id="cd-hours" class="block text-xl sm:text-2xl font-black text-white tabular-nums">--</span>
				<span class="block text-[10px] uppercase font-sans tracking-wider text-slate-400 font-semibold mt-0.5">Hours</span>
			</div>
			<div class="bg-slate-950/70 border border-slate-800 rounded-lg py-2 px-1">
				<span id="cd-mins" class="block text-xl sm:text-2xl font-black text-white tabular-nums">--</span>
				<span class="block text-[10px] uppercase font-sans tracking-wider text-slate-400 font-semibold mt-0.5">Mins</span>
			</div>
			<div class="bg-slate-950/70 border border-slate-800 rounded-lg py-2 px-1">
				<span id="cd-secs" class="block text-xl sm:text-2xl font-black text-amber-400 tabular-nums">--</span>
				<span class="block text-[10px] uppercase font-sans tracking-wider text-slate-400 font-semibold mt-0.5">Secs</span>
			</div>
		</div>
	</div>
}

templ EventsSection(activeTab string, events []db.ListUpcomingEventsRow) {
	<section id="calendar-section" class="space-y-6">
		<!-- Filter Tabs & Quick Actions -->
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-slate-800 pb-4">
			<div class="flex items-center gap-2">
				<a
					href="/"
					hx-get="/"
					hx-target="#calendar-section"
					hx-swap="outerHTML"
					hx-push-url="true"
					class={ "px-4 py-2 rounded-lg text-sm font-semibold transition-all cursor-pointer",
						templ.KV("bg-red-600 text-white shadow-lg shadow-red-600/30", activeTab == "" || activeTab == "all"),
						templ.KV("bg-slate-900/80 text-slate-400 hover:text-white hover:bg-slate-800", activeTab != "" && activeTab != "all") }
				>
					All Series
				</a>
				<a
					href="/?series=f1"
					hx-get="/?series=f1"
					hx-target="#calendar-section"
					hx-swap="outerHTML"
					hx-push-url="true"
					class={ "px-4 py-2 rounded-lg text-sm font-semibold transition-all cursor-pointer",
						templ.KV("bg-red-600 text-white shadow-lg shadow-red-600/30", activeTab == "f1"),
						templ.KV("bg-slate-900/80 text-slate-400 hover:text-white hover:bg-slate-800", activeTab != "f1") }
				>
					🏎️ Formula 1
				</a>
				<a
					href="/?series=motogp"
					hx-get="/?series=motogp"
					hx-target="#calendar-section"
					hx-swap="outerHTML"
					hx-push-url="true"
					class={ "px-4 py-2 rounded-lg text-sm font-semibold transition-all cursor-pointer",
						templ.KV("bg-sky-600 text-white shadow-lg shadow-sky-600/30", activeTab == "motogp"),
						templ.KV("bg-slate-900/80 text-slate-400 hover:text-white hover:bg-slate-800", activeTab != "motogp") }
				>
					🏍️ MotoGP
				</a>
			</div>

			<!-- Calendar Sync Button -->
			<div class="flex items-center gap-2">
				if activeTab == "f1" {
					<a
						href="/series/f1/calendar.ics"
						download="f1-calendar.ics"
						class="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-900 text-slate-300 border border-slate-800 hover:border-slate-700 hover:text-white transition-colors"
					>
						📅 Export F1 .ics
					</a>
				} else if activeTab == "motogp" {
					<a
						href="/series/motogp/calendar.ics"
						download="motogp-calendar.ics"
						class="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-900 text-slate-300 border border-slate-800 hover:border-slate-700 hover:text-white transition-colors"
					>
						📅 Export MotoGP .ics
					</a>
				} else {
					<div class="flex items-center gap-1.5">
						<a
							href="/series/f1/calendar.ics"
							class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-slate-900 text-slate-300 border border-slate-800 hover:border-slate-700 hover:text-white transition-colors"
						>
							F1 .ics
						</a>
						<a
							href="/series/motogp/calendar.ics"
							class="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-slate-900 text-slate-300 border border-slate-800 hover:border-slate-700 hover:text-white transition-colors"
						>
							MotoGP .ics
						</a>
					</div>
				}
			</div>
		</div>

		<!-- Race Calendar List -->
		if len(events) == 0 {
			<div class="rounded-2xl border border-dashed border-slate-800 p-12 text-center">
				<div class="text-4xl mb-3">🏁</div>
				<h3 class="text-lg font-bold text-white">No upcoming events scheduled</h3>
				<p class="text-sm text-slate-400 mt-1 max-w-md mx-auto">
					The current season calendar has concluded or upcoming events are awaiting upstream data sync.
				</p>
			</div>
		} else {
			<div class="space-y-4">
				for i, ev := range events {
					if i == 0 || FormatMonthYear(ev.EventStartsAt) != FormatMonthYear(events[i-1].EventStartsAt) {
						<div class="pt-4 pb-2">
							<h2 class="text-sm font-bold uppercase tracking-wider text-slate-400 flex items-center gap-2">
								<span class="w-1.5 h-1.5 rounded-full bg-red-500"></span>
								{ FormatMonthYear(ev.EventStartsAt) }
							</h2>
						</div>
					}
					@EventCard(ev)
				}
			</div>
		}
	</section>
}

templ EventCard(ev db.ListUpcomingEventsRow) {
	<div class={ "group relative rounded-xl border border-slate-800/80 bg-slate-900/40 p-5 transition-all duration-200 hover:border-slate-700 hover:bg-slate-900/80 hover:shadow-xl",
		templ.KV("opacity-60", ev.EventStatus == "canceled" || ev.EventStatus == "cancelled") }>
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
			<!-- Event Info -->
			<div class="space-y-1.5">
				<div class="flex items-center gap-2.5 flex-wrap">
					<!-- Series Badge -->
					<span class={ "px-2 py-0.5 rounded text-[11px] font-bold uppercase tracking-wider", SeriesBadgeClass(ev.SerieID) }>
						{ ev.SerieID }
					</span>

					<!-- Round Number -->
					<span class="text-xs font-semibold text-slate-400">
						Round { fmt.Sprintf("%d", ev.EventRound) }
					</span>

					<!-- Status Badge -->
					<span class={ "px-2 py-0.5 rounded-full text-[10px] font-semibold uppercase tracking-wider", StatusBadgeClass(ev.EventStatus) }>
						{ ev.EventStatus }
					</span>
				</div>

				<!-- Event Name -->
				<h3 class="text-lg font-bold text-white group-hover:text-red-400 transition-colors">
					<a href={ templ.SafeURL("/events/" + ev.EventSlug) } class="focus:outline-none">
						<span class="absolute inset-0" aria-hidden="true"></span>
						{ ev.EventName }
					</a>
				</h3>

				<!-- Circuit and Country -->
				<p class="text-xs sm:text-sm text-slate-400 flex items-center gap-1.5">
					<span>📍</span>
					<span>{ ev.CircuitName }</span>
					if TextString(ev.CircuitLocality, "") != "" {
						<span class="text-slate-500">•</span>
						<span>{ TextString(ev.CircuitLocality, "") }</span>
					}
					if TextString(ev.CircuitCountry, "") != "" {
						<span class="text-slate-500">•</span>
						<span class="font-medium text-slate-300">{ TextString(ev.CircuitCountry, "") }</span>
					}
				</p>
			</div>

			<!-- Date & Action -->
			<div class="flex sm:flex-col sm:items-end justify-between items-center shrink-0 pt-2 sm:pt-0 border-t border-slate-800/60 sm:border-0">
				<div class="text-left sm:text-right">
					<div class="text-sm font-bold text-white font-mono">
						{ FormatDate(ev.EventStartsAt) }
					</div>
					<div class="text-xs text-slate-400" data-utc={ ISOTimestamp(ev.EventStartsAt) }>
						{ FormatTimeUTC(ev.EventStartsAt) }
					</div>
				</div>

				<span class="mt-2 text-xs font-medium text-red-400 group-hover:translate-x-0.5 transition-transform flex items-center gap-1">
					Details <span>&rarr;</span>
				</span>
			</div>
		</div>
	</div>
}
```

---

### Task 4: Load `countdown.js` in `internal/views/layout.templ`
**File**: `internal/views/layout.templ`  
**Description**: Add `<script src="/static/countdown.js" defer></script>` in the `<head>` of `layout.templ`.

```templ
<!-- Line 13 of internal/views/layout.templ -->
<link rel="stylesheet" href="/static/style.css"/>
<script src="/static/countdown.js" defer></script>
```

---

### Task 5: Execute `templ generate`
**Command**:
```bash
templ generate
```
**Expected Output**: `Generated internal/views/home_templ.go` and `internal/views/layout_templ.go` with exit code 0.

---

### Task 6: Backend Handler Integration (Contract-First)
**File**: `internal/web/handler.go`  
**Description**: Update `handleHome` to check for `r.Header.Get("HX-Request") == "true"`.
- If true: render `views.EventsSection(seriesFilter, events)` (the partial).
- If false: render `views.Home(seriesFilter, events)` (the full page).

```go
// Inside handleHome in internal/web/handler.go:
w.Header().Set("Content-Type", "text/html; charset=utf-8")

if r.Header.Get("HX-Request") == "true" {
    // Partial render for HTMX tab navigation
    if err := views.EventsSection(seriesFilter, events).Render(r.Context(), w); err != nil {
        http.Error(w, "Failed to render partial", http.StatusInternalServerError)
    }
    return
}

// Full page render for initial / direct browser requests
if err := views.Home(seriesFilter, events).Render(r.Context(), w); err != nil {
    http.Error(w, "Failed to render template", http.StatusInternalServerError)
}
```

---

## Tests & Validation

### 1. Build Verification
```bash
templ generate && go build ./... && go vet ./...
```
*Expected Result*: Exit code 0, no errors, no warnings.

### 2. HTMX Partial vs Full Page Test
Run server: `go run ./cmd/server/main.go`

- **Full Page Request**:
  ```bash
  curl -s http://localhost:8081/ | grep "<!DOCTYPE html>"
  ```
  *Expected*: Contains `<!DOCTYPE html>` and `<html lang="en" class="h-full dark">`.

- **HTMX Partial Request**:
  ```bash
  curl -s -H "HX-Request: true" "http://localhost:8081/?series=f1" | grep "<!DOCTYPE html>"
  ```
  *Expected*: Empty (does not contain `<!DOCTYPE html>`).
  ```bash
  curl -s -H "HX-Request: true" "http://localhost:8081/?series=f1" | grep 'id="calendar-section"'
  ```
  *Expected*: Match found (`<section id="calendar-section"...`).

### 3. Browser End-to-End Visual Verification
1. Open `http://localhost:8081/` in the browser.
2. Verify Countdown Widget:
   - Digit blocks (`Days`, `Hours`, `Mins`, `Secs`) display numbers.
   - `Secs` decrements every 1 second in amber text.
3. Click "🏎️ Formula 1" tab:
   - URL updates to `http://localhost:8081/?series=f1` without full page refresh.
   - Tab background changes to red.
   - Event list smoothly crossfades to F1 events only.
4. Click browser back button:
   - Navigates back to "All Series" seamlessly (`hx-push-url="true"`).

---

## Risks, Tradeoffs, and Open Questions

1. **Client Clock Skew**:
   - *Risk*: If a user's system clock is wrong, countdown values could be offset.
   - *Mitigation*: The countdown is cosmetic for the frontend; all authoritative event timetables display explicit UTC and date strings.
2. **Tab Swap Granularity vs OOB Swaps**:
   - *Decision*: Swapping `#calendar-section` (which bundles both tabs and cards) avoids needing multiple `hx-swap-oob` fragments or client-side class toggling JS. It keeps the Templ view and Go handler simple and clean.
3. **Countdown Re-Initialization**:
   - *Detail*: Since `#countdown-widget` lives in the Hero section (outside `#calendar-section`), tab clicks will NOT destroy or reset the running ticker, preserving timer continuity.
