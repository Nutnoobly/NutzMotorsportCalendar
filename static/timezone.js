// Nutz Motorsport Calendar — Client-side Timezone Auto-detection & Conversion
(function () {
    const STORAGE_KEY = "nutz_user_timezone";

    // Detect system IANA timezone
    function getSystemTimezone() {
        try {
            return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
        } catch (e) {
            return "UTC";
        }
    }

    // Get current active timezone (user override or auto system)
    function getActiveTimezone() {
        const stored = localStorage.getItem(STORAGE_KEY);
        if (stored && stored !== "auto") {
            return stored;
        }
        return getSystemTimezone();
    }

    // Get short timezone abbreviation (e.g. 'ICT', 'EDT', 'CET', 'UTC')
    function getTimezoneAbbr(date, timeZone) {
        try {
            const parts = new Intl.DateTimeFormat("en-US", {
                timeZone,
                timeZoneName: "short"
            }).formatToParts(date);
            const tzPart = parts.find(p => p.type === "timeZoneName");
            return tzPart ? tzPart.value : timeZone;
        } catch (e) {
            return timeZone;
        }
    }

    // Format ISO UTC string according to target timezone and format specifier
    function formatUtc(utcString, timeZone, formatType) {
        if (!utcString) return "";
        const date = new Date(utcString);
        if (isNaN(date.getTime())) return "";

        const tzAbbr = getTimezoneAbbr(date, timeZone);

        try {
            if (formatType === "time") {
                // "15:04 ICT"
                const timeStr = new Intl.DateTimeFormat("en-GB", {
                    timeZone,
                    hour: "2-digit",
                    minute: "2-digit",
                    hour12: false
                }).format(date);
                return `${timeStr} ${tzAbbr}`;
            }

            if (formatType === "date") {
                // "06 Sep 2026"
                return new Intl.DateTimeFormat("en-GB", {
                    timeZone,
                    day: "2-digit",
                    month: "short",
                    year: "numeric"
                }).format(date);
            }

            if (formatType === "datetime") {
                // "06 Sep 2026, 15:04 ICT"
                const dateStr = new Intl.DateTimeFormat("en-GB", {
                    timeZone,
                    day: "2-digit",
                    month: "short",
                    year: "numeric"
                }).format(date);
                const timeStr = new Intl.DateTimeFormat("en-GB", {
                    timeZone,
                    hour: "2-digit",
                    minute: "2-digit",
                    hour12: false
                }).format(date);
                return `${dateStr}, ${timeStr} ${tzAbbr}`;
            }

            if (formatType === "short-datetime") {
                // "06 Sep, 15:04"
                return new Intl.DateTimeFormat("en-GB", {
                    timeZone,
                    day: "2-digit",
                    month: "short",
                    hour: "2-digit",
                    minute: "2-digit",
                    hour12: false
                }).format(date);
            }

            // Default: "15:04 ICT"
            const timeStr = new Intl.DateTimeFormat("en-GB", {
                timeZone,
                hour: "2-digit",
                minute: "2-digit",
                hour12: false
            }).format(date);
            return `${timeStr} ${tzAbbr}`;
        } catch (e) {
            console.error("Timezone formatting error:", e);
            return date.toUTCString();
        }
    }

    // Update all DOM elements with timezone data
    function applyTimezones(root = document) {
        const timeZone = getActiveTimezone();
        const now = new Date();
        const currentAbbr = getTimezoneAbbr(now, timeZone);

        // 1. Update all [data-utc] elements
        const elements = root.querySelectorAll("[data-utc]");
        elements.forEach(el => {
            const utcString = el.getAttribute("data-utc");
            if (!utcString) return;

            const formatType = el.getAttribute("data-format") || "time";
            const formatted = formatUtc(utcString, timeZone, formatType);
            if (formatted) {
                el.textContent = formatted;
            }
        });

        // 2. Update timezone abbreviations in labels (e.g. "Times in ICT")
        const abbrElements = root.querySelectorAll(".tz-abbr");
        abbrElements.forEach(el => {
            el.textContent = currentAbbr;
        });

        // 3. Update timezone names in labels (e.g. "Asia/Bangkok")
        const nameElements = root.querySelectorAll(".tz-name");
        nameElements.forEach(el => {
            el.textContent = timeZone;
        });

        // 4. Synchronize timezone selector dropdown if present
        const selector = document.getElementById("tz-selector");
        if (selector) {
            const stored = localStorage.getItem(STORAGE_KEY) || "auto";
            selector.value = stored;
        }
    }

    // Set user timezone override
    function setTimezone(tz) {
        if (tz === "auto") {
            localStorage.removeItem(STORAGE_KEY);
        } else {
            localStorage.setItem(STORAGE_KEY, tz);
        }
        applyTimezones();
    }

    // Expose API on window
    window.NutzTimezone = {
        getActive: getActiveTimezone,
        getSystem: getSystemTimezone,
        set: setTimezone,
        format: formatUtc,
        refresh: applyTimezones
    };

    // Initialize on DOM load
    if (document.readyState === "loading") {
        document.addEventListener("DOMContentLoaded", function () {
            applyTimezones();
            setupSelector();
        });
    } else {
        applyTimezones();
        setupSelector();
    }

    // Attach selector change handler
    function setupSelector() {
        const selector = document.getElementById("tz-selector");
        if (!selector) return;

        // Populate auto option with detected timezone name
        const autoOpt = selector.querySelector('option[value="auto"]');
        if (autoOpt) {
            autoOpt.textContent = `Auto (${getSystemTimezone()})`;
        }

        const stored = localStorage.getItem(STORAGE_KEY) || "auto";
        selector.value = stored;

        selector.addEventListener("change", function (e) {
            setTimezone(e.target.value);
        });
    }

    // Re-apply on HTMX partial content swaps
    document.body.addEventListener("htmx:afterSettle", function (evt) {
        applyTimezones(evt.target || document);
        setupSelector();
    });
})();
