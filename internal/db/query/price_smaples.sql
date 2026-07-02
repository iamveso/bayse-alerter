-- name: InsertPriceSample :exec
INSERT INTO price_samples (market_id, outcome_id, price, observed_at)
VALUES ($1, $2, $3, $4);

-- name: ListPriceSamplesForWindow :many
SELECT price, observed_at
FROM price_samples
WHERE market_id = $1
  AND outcome_id = $2
  AND observed_at >= $3
  AND observed_at <= $4
ORDER BY observed_at ASC;
