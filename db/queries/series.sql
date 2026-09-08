-- name: ListSeries :many
SELECT serie_id, serie_name, serie_slug
FROM SERIES
ORDER BY serie_id ASC;

-- name: GetSeries :one
SELECT serie_id, serie_name, serie_slug
FROM SERIES
WHERE serie_id = $1;

-- name: GetSeriesBySlug :one
SELECT serie_id, serie_name, serie_slug
FROM SERIES
WHERE serie_slug = $1;
