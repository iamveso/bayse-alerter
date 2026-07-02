-- name: CreateRule :one
INSERT INTO rules (
    event_id,
    event_slug,
    market_id,
    outcome,
    outcome_id,
    type,
    direction,
    target,
    percent,
    window_seconds,
    state
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, event_id, event_slug, market_id, outcome, outcome_id, type,
    direction, target, percent, window_seconds, enabled, state, trigger_sequence,
    created_at, updated_at;

-- name: ListEnabledRules :many
SELECT id, event_id, event_slug, market_id, outcome, outcome_id, type,
    direction, target, percent, window_seconds, enabled, state, trigger_sequence,
    created_at, updated_at
FROM rules
WHERE enabled = true
ORDER BY created_at ASC;

-- name: GetRuleForUpdate :one
SELECT id, event_id, event_slug, market_id, outcome, outcome_id, type,
    direction, target, percent, window_seconds, enabled, state, trigger_sequence,
    created_at, updated_at
FROM rules
WHERE id = $1
FOR UPDATE;

-- name: UpdateRuleState :exec
UPDATE rules
SET state = $1,
    trigger_sequence = $2,
    updated_at = now()
WHERE id = $3;
