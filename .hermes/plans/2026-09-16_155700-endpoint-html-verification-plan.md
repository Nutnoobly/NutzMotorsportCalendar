# Endpoint HTML & Content-Type Verification Plan

## Goal
Verify all application routes and response types across **NutzMotorsportCalendar** — differentiating between full-page HTML5 documents, HTMX partial fragments, iCalendar feeds, JSON APIs, and static assets.

---

## 1. What Each Test Verifies

| Endpoint | Method | Header | Target Content-Type | What It Verifies |
| :--- | :--- | :--- | :--- | :--- |
| `GET /` | `GET` | *(Standard Browser)* | `text/html; charset=utf-8` | **Full Home Page**: Returns complete HTML document with `<!doctype html>`, `<head>` assets, navigation, countdown hero, and calendar list. |
| `GET /?series=f1` | `GET` | `HX-Request: true` | `text/html; charset=utf-8` | **HTMX Partial Swap**: Returns ONLY `<div id="calendar-section"...>` without `<!doctype html>` or outer layout, allowing instant DOM swapping without full reload. |
| `GET /?series=motogp` | `GET` | `HX-Request: true` | `text/html; charset=utf-8` | **HTMX Series Filter**: Returns only the MotoGP series partial tab and card section. |
| `GET /events/{slug}` | `GET` | *(Standard Browser)* | `text/html; charset=utf-8` | **Full Event Detail Page**: Returns complete HTML page with circuit info, weekend timetable sessions, and podium results. |
| `GET /series/f1/calendar.ics` | `GET` | *(Standard Browser)* | `text/calendar; charset=utf-8` | **iCalendar Feed**: Returns RFC 5545 `.ics` file attachment for syncing with Apple Calendar, Google Calendar, or Outlook. |
| `POST /admin/refresh` | `POST` | *(No Auth)* | `text/plain` | **Auth Guard**: Returns `401 Unauthorized` when `ADMIN_SECRET` is missing or invalid. |
| `POST /admin/refresh` | `POST` | `Authorization: Bearer <secret>` | `application/json` | **Admin Sync API**: Triggers upstream data ingestion and returns `{"status":"ok",...}`. |
| `GET /static/style.css` | `GET` | *(Standard Browser)* | `text/css` | **Static File Server**: Verifies `http.FileServer` serves CSS without path errors. |
| `GET /static/countdown.js`| `GET` | *(Standard Browser)* | `text/javascript` or `application/javascript` | **Client Scripts**: Verifies countdown timer script is served. |

---

## 2. Step-by-Step Verification Script

Run the following commands against the local server (default port `8081`):

### Test 1: Full Home Page (Expects DOCTYPE)
```bash
curl -s -i http://localhost:8081/ | grep -E "HTTP/|Content-Type:|<\\!doctype"
```
- **Expected Status**: `HTTP/1.1 200 OK`
- **Expected Content-Type**: `Content-Type: text/html; charset=utf-8`
- **Expected Body**: Starts with `<!doctype html>`

### Test 2: HTMX Partial Swap (Expects NO DOCTYPE, only `#calendar-section`)
```bash
curl -s -i -H "HX-Request: true" "http://localhost:8081/?series=f1" | grep -E "HTTP/|Content-Type:|<\\!doctype|<div id=\"calendar-section\""
```
- **Expected Status**: `HTTP/1.1 200 OK`
- **Expected Content-Type**: `Content-Type: text/html; charset=utf-8`
- **Expected Match**: `<div id="calendar-section"` is present; `<!doctype` is NOT present.

### Test 3: Event Detail Page (Expects DOCTYPE and Circuit / Timetable)
```bash
curl -s -i http://localhost:8081/events/monza-2026 | grep -E "HTTP/|Content-Type:|<\\!doctype"
```
- **Expected Status**: `HTTP/1.1 200 OK` (or `404 Not Found` if test slug does not exist in DB yet)
- **Expected Content-Type**: `Content-Type: text/html; charset=utf-8`

### Test 4: iCalendar Export
```bash
curl -s -i http://localhost:8081/series/f1/calendar.ics | grep -E "HTTP/|Content-Type:|BEGIN:VCALENDAR"
```
- **Expected Status**: `HTTP/1.1 200 OK`
- **Expected Content-Type**: `Content-Type: text/calendar; charset=utf-8`
- **Expected Body**: `BEGIN:VCALENDAR`

### Test 5: Static Asset Serving
```bash
curl -s -i http://localhost:8081/static/style.css | head -n 5
```
- **Expected Status**: `HTTP/1.1 200 OK`
- **Expected Content-Type**: `Content-Type: text/css; charset=utf-8`

---

## 3. Automated Bash Verification One-Liner

Save or run this one-liner to verify all endpoints at once:

```bash
python3 -c "
import urllib.request

def check(name, url, headers={}, expect_doctype=True):
    try:
        req = urllib.request.Request(url, headers=headers)
        with urllib.request.urlopen(req) as resp:
            body = resp.read().decode('utf-8')
            ct = resp.headers.get('Content-Type', '')
            has_dt = '<!doctype html>' in body.lower()
            ok = (has_dt == expect_doctype)
            status = 'PASS' if ok else 'FAIL'
            print(f'[{status}] {name:30} | Status: {resp.status} | Content-Type: {ct} | DOCTYPE: {has_dt}')
    except Exception as e:
        print(f'[ERR ] {name:30} | {e}')

base = 'http://localhost:8081'
check('Home (Full HTML)', base + '/', {}, expect_doctype=True)
check('HTMX Partial (F1)', base + '/?series=f1', {'HX-Request': 'true'}, expect_doctype=False)
check('HTMX Partial (MotoGP)', base + '/?series=motogp', {'HX-Request': 'true'}, expect_doctype=False)
check('ICS Export', base + '/series/f1/calendar.ics', {}, expect_doctype=False)
check('Static CSS', base + '/static/style.css', {}, expect_doctype=False)
"
```
