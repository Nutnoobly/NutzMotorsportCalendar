-- name: CreateSyncRun :one
INSERT INTO SYNC_RUN (serie_id, started_at, ok, message)
VALUES ($1, now(), false, $2)
RETURNING sync_id, serie_id, started_at, finished_at, ok, message;

-- name: FinishSyncRun :exec
UPDATE SYNC_RUN
SET finished_at = now(), ok = $2, message = $3
WHERE sync_id = $1;

-- name: GetLastSyncRunBySeries :one
SELECT sync_id, serie_id, started_at, finished_at, ok, message
FROM SYNC_RUN
WHERE serie_id = $1
ORDER BY started_at DESC
LIMIT 1;

-- name: UpsertCircuit :one
INSERT INTO CIRCUITS (
    circuit_slug,
    circuit_name,
    circuit_locality,
    circuit_country,
    circuit_latitude,
    circuit_longitude,
    circuit_external_id
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (circuit_slug) DO UPDATE SET
    circuit_name = EXCLUDED.circuit_name,
    circuit_locality = EXCLUDED.circuit_locality,
    circuit_country = EXCLUDED.circuit_country,
    circuit_latitude = EXCLUDED.circuit_latitude,
    circuit_longitude = EXCLUDED.circuit_longitude,
    circuit_external_id = COALESCE(EXCLUDED.circuit_external_id, CIRCUITS.circuit_external_id)
RETURNING circuit_id;

-- name: GetCircuitBySlug :one
SELECT circuit_id, circuit_slug, circuit_name, circuit_locality, circuit_country, circuit_latitude, circuit_longitude, circuit_external_id
FROM CIRCUITS
WHERE circuit_slug = $1;

-- name: InsertTeam :one
INSERT INTO TEAMS (serie_id, team_name, team_external_id)
VALUES ($1, $2, $3)
RETURNING team_id;

-- name: GetTeamByExternalID :one
SELECT team_id, serie_id, team_name, team_external_id
FROM TEAMS
WHERE serie_id = $1 AND team_external_id = $2;

-- name: GetTeamByName :one
SELECT team_id, serie_id, team_name, team_external_id
FROM TEAMS
WHERE serie_id = $1 AND team_name = $2;

-- name: InsertDriver :one
INSERT INTO DRIVERS (
    serie_id, team_id, driver_first_name, driver_last_name,
    driver_code, driver_number, driver_external_id
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING driver_id;

-- name: UpdateDriverTeam :exec
UPDATE DRIVERS
SET team_id = $2
WHERE driver_id = $1;

-- name: GetDriverByExternalID :one
SELECT driver_id, serie_id, team_id, driver_first_name, driver_last_name, driver_code, driver_number, driver_external_id
FROM DRIVERS
WHERE serie_id = $1 AND driver_external_id = $2;

-- name: UpsertEvent :one
INSERT INTO EVENTS (
    serie_id, circuit_id, event_season, event_round, event_slug,
    event_name, event_starts_at, event_status, event_official_url,
    event_ticket_url, event_external_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (serie_id, event_season, event_round) DO UPDATE SET
    circuit_id = EXCLUDED.circuit_id,
    event_slug = EXCLUDED.event_slug,
    event_name = EXCLUDED.event_name,
    event_starts_at = EXCLUDED.event_starts_at,
    event_status = EXCLUDED.event_status,
    event_official_url = EXCLUDED.event_official_url,
    event_ticket_url = EXCLUDED.event_ticket_url,
    event_external_id = EXCLUDED.event_external_id
RETURNING event_id;

-- name: DeleteSessionsByEventID :exec
DELETE FROM SESSIONS
WHERE event_id = $1;

-- name: InsertSession :one
INSERT INTO SESSIONS (event_id, session_kind, session_name, session_starts_at)
VALUES ($1, $2, $3, $4)
RETURNING session_id;

-- name: UpsertResult :exec
INSERT INTO RESULTS (
    event_id, session_type, result_position, driver_id, team_id,
    result_time_or_gap, result_points
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (event_id, session_type, result_position) DO UPDATE SET
    driver_id = EXCLUDED.driver_id,
    team_id = EXCLUDED.team_id,
    result_time_or_gap = EXCLUDED.result_time_or_gap,
    result_points = EXCLUDED.result_points;
