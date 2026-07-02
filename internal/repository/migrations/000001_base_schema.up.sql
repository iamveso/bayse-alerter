CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS rules (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id text NOT NULL,
    event_slug text NOT NULL,
    market_id text NOT NULL,
    outcome text NOT NULL,
    outcome_id text NOT NULL,
    type text NOT NULL CHECK (type IN ('threshold_cross', 'percent_move')),
    direction text NOT NULL DEFAULT '',
    target double precision NOT NULL DEFAULT 0,
    percent double precision NOT NULL DEFAULT 0,
    window_seconds integer NOT NULL DEFAULT 0,
    enabled boolean NOT NULL DEFAULT true,
    state jsonb NOT NULL DEFAULT '{"triggered": false}'::jsonb,
    trigger_sequence bigint NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS rules_enabled_idx ON rules (enabled);
CREATE INDEX IF NOT EXISTS rules_ticker_idx ON rules (market_id, outcome_id) WHERE enabled;

CREATE TABLE IF NOT EXISTS price_samples (
    id bigserial PRIMARY KEY,
    market_id text NOT NULL,
    outcome_id text NOT NULL,
    price double precision NOT NULL,
    observed_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS price_samples_lookup_idx
    ON price_samples (market_id, outcome_id, observed_at);

CREATE TABLE IF NOT EXISTS alerts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    rule_id uuid NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    market_id text NOT NULL,
    outcome text NOT NULL,
    outcome_id text NOT NULL,
    observed_price double precision NOT NULL,
    trigger_value double precision NOT NULL,
    trigger_sequence bigint NOT NULL,
    triggered_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (rule_id, trigger_sequence)
);
