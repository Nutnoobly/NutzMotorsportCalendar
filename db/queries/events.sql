-- name: ListUpcomingEvents :many
SELECT 
    e.event_id,
    e.serie_id,
    e.circuit_id,
    e.event_season,
    e.event_round,
    e.event_slug,
    e.event_name,
    e.event_starts_at,
    e.event_status,
    e.event_official_url,
    e.event_ticket_url,
    c.circuit_slug,
    c.circuit_name,
    c.circuit_locality,
    c.circuit_country
FROM EVENTS e
JOIN CIRCUITS c ON e.circuit_id = c.circuit_id
WHERE e.event_starts_at >= $1
ORDER BY e.event_starts_at ASC;

-- name: ListUpcomingEventsBySeries :many
SELECT 
    e.event_id,
    e.serie_id,
    e.circuit_id,
    e.event_season,
    e.event_round,
    e.event_slug,
    e.event_name,
    e.event_starts_at,
    e.event_status,
    e.event_official_url,
    e.event_ticket_url,
    c.circuit_slug,
    c.circuit_name,
    c.circuit_locality,
    c.circuit_country
FROM EVENTS e
JOIN CIRCUITS c ON e.circuit_id = c.circuit_id
WHERE e.serie_id = $1 AND e.event_starts_at >= $2
ORDER BY e.event_starts_at ASC;

-- name: GetLastCompletedEventBySeries :one
SELECT 
    e.event_id,
    e.serie_id,
    e.circuit_id,
    e.event_season,
    e.event_round,
    e.event_slug,
    e.event_name,
    e.event_starts_at,
    e.event_status,
    e.event_official_url,
    e.event_ticket_url,
    c.circuit_slug,
    c.circuit_name,
    c.circuit_locality,
    c.circuit_country
FROM EVENTS e
JOIN CIRCUITS c ON e.circuit_id = c.circuit_id
WHERE e.serie_id = $1 AND e.event_status = 'completed'
ORDER BY e.event_starts_at DESC
LIMIT 1;

-- name: GetEventBySlug :one
SELECT 
    e.event_id,
    e.serie_id,
    e.circuit_id,
    e.event_season,
    e.event_round,
    e.event_slug,
    e.event_name,
    e.event_starts_at,
    e.event_status,
    e.event_official_url,
    e.event_ticket_url,
    c.circuit_slug,
    c.circuit_name,
    c.circuit_locality,
    c.circuit_country,
    c.circuit_latitude,
    c.circuit_longitude
FROM EVENTS e
JOIN CIRCUITS c ON e.circuit_id = c.circuit_id
WHERE e.event_slug = $1;

-- name: ListSessionsByEventID :many
SELECT 
    session_id,
    event_id,
    session_kind,
    session_name,
    session_starts_at
FROM SESSIONS
WHERE event_id = $1
ORDER BY session_starts_at ASC;
