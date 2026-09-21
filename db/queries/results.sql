-- name: ListResultsByEventID :many
SELECT 
    r.event_id,
    r.session_type,
    r.result_position,
    r.driver_id,
    r.team_id,
    r.result_time_or_gap,
    r.result_points,
    d.driver_first_name,
    d.driver_last_name,
    d.driver_code,
    d.driver_number,
    t.team_name
FROM RESULTS r
JOIN DRIVERS d ON r.driver_id = d.driver_id
JOIN TEAMS t ON r.team_id = t.team_id
WHERE r.event_id = $1
ORDER BY r.session_type ASC, r.result_position ASC;

-- name: GetSessionResultP1 :one
SELECT 
    r.event_id,
    r.session_type,
    r.result_position,
    r.driver_id,
    r.team_id,
    r.result_time_or_gap,
    r.result_points,
    d.driver_first_name,
    d.driver_last_name,
    d.driver_code,
    d.driver_number,
    t.team_name
FROM RESULTS r
JOIN DRIVERS d ON r.driver_id = d.driver_id
JOIN TEAMS t ON r.team_id = t.team_id
WHERE r.event_id = $1 AND r.session_type = $2 AND r.result_position = 1
LIMIT 1;

