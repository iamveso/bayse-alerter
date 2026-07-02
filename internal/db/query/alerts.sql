-- name: InsertAlert :exec
INSERT INTO alerts (
    rule_id,
    market_id,
    outcome,
    outcome_id,
    observed_price,
    trigger_value,
    trigger_sequence,
    triggered_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);
