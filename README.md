# bayse-alerter

`bayse-alerter` is a small Go service that registers Bayse prediction-market price alert rules, polls Bayse ticker prices, evaluates enabled rules, and persists alerts to Postgres.

The service supports two rule types:

- `threshold_cross`: fires when an outcome price crosses above or below a target probability.
- `percent_move`: fires when the absolute percentage move over a rolling window reaches a configured threshold.

## Requirements

- Go 1.25+
- Podman with Compose support (Docker would also work but the makefile would need to be updated if used)
- A Bayse public API key

## Configuration

Create a local `.env` from `.env.example`:

```bash
cp .env.example .env
```

Set at least:

```env
BAYSE_PUBLIC_KEY=pk_live_...
BAYSE_BASE_URL=https://relay.bayse.markets
HTTP_PORT=8080
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=bayse
DB_PORT=5432
POLL_INTERVAL_SECONDS=20
POLL_TICK_TIMEOUT_SECONDS=10
```

For local host tools, `DATABASE_URL` can use `localhost`:

```env
DATABASE_URL=postgres://postgres:postgres@localhost:5432/bayse?sslmode=disable
```

Inside Podman Compose, the app is given a container-network database URL automatically using the `postgres` service name.

## Running The Project

Start Postgres, run migrations, and start the app:

```bash
make compose-up
```

Equivalent direct command:

```bash
podman compose up --build
```

The app container runs `golang-migrate` before starting the HTTP server. If migration fails, the app process does not start.

Useful commands:

```bash
make compose-logs
make compose-down
```

Check alerts in Postgres:

```bash
podman compose exec postgres psql -U postgres -d bayse -c "SELECT * FROM alerts ORDER BY created_at DESC;"
```

## Running Tests

Run all tests:

```bash
make test
```

Equivalent:

```bash
go test -v ./...
```

The current tests focus on rule evaluation and edge-triggering behavior.

## Creating Rules

Endpoint:

```text
POST /rules
```

The request must include an `eventSlug` and one or more rules. The service resolves the event from Bayse, validates that each `marketId` and `outcome` exists on that event, then persists all rules atomically.

Threshold-cross example:

```bash
curl -X POST http://localhost:8080/rules \
  -H "Content-Type: application/json" \
  -d '{
    "eventSlug": "crypto-btc-1h-feb-24-11am",
    "rules": [
      {
        "marketId": "b2c3d4e5-...",
        "outcome": "YES",
        "type": "threshold_cross",
        "direction": "above",
        "target": 0.60
      }
    ]
  }'
```

Percent-move example:

```bash
curl -X POST http://localhost:8080/rules \
  -H "Content-Type: application/json" \
  -d '{
    "eventSlug": "crypto-btc-1h-feb-24-11am",
    "rules": [
      {
        "marketId": "b2c3d4e5-...",
        "outcome": "YES",
        "type": "percent_move",
        "percent": 10,
        "windowSeconds": 900
      }
    ]
  }'
```

Successful response:

```json
{
  "created": [
    {
      "ruleId": "rule-uuid",
      "type": "threshold_cross"
    }
  ]
}
```

Validation failures return `422` with per-rule error details. Unknown event slugs return `404`.

## Design Decisions

The service uses the standard `net/http` mux rather than a router framework because the API surface is intentionally small.

Postgres access uses `pgx`, and SQL is generated with `sqlc`. The schema is managed with plain SQL migrations run by `golang-migrate`.

The poller loads enabled rules on each tick and groups them by `(marketId, outcomeId)`. This avoids fetching the same ticker repeatedly when multiple rules watch the same outcome.

The canonical price is Bayse ticker `midPrice`. It is used because it is engine-agnostic and less noisy than `lastPrice`.

Rule state is persisted in Postgres, not held only in memory. Each evaluation reloads the rule row with `FOR UPDATE`, updates the rule state, and inserts an alert inside one transaction. Alerts also use `(rule_id, trigger_sequence)` as a uniqueness guard, so a rule cannot create two alerts for the same firing sequence.

Threshold rules are edge-triggered. A rule fires only when the condition moves from not triggered to triggered. It re-arms when the price no longer satisfies the condition.

Percent-move rules compare the current price against the oldest stored sample in the configured window. They fire once when the absolute move reaches the threshold and re-arm once the move falls below the threshold.

Creating the same rule twice currently creates two separate rules. The endpoint is not idempotent, and there is no uniqueness constraint across rule definition fields.

## Assumptions

Prices are probabilities, so threshold targets must be greater than `0` and less than `1`.

`threshold_cross.direction` must be either `above` or `below`.

`percent_move.percent` must be greater than `0` and no more than `500`. The upper bound is a pragmatic guardrail against accidental or nonsensical inputs.

`percent_move.windowSeconds` must be between `15` and `86400`. This rejects very noisy windows and very large windows that would be expensive for a polling service without additional retention controls.

The service records alerts in Postgres only. It does not send email, webhook, SMS, or push notifications.

No hysteresis buffer is implemented for threshold rules. A price hovering around the exact target can re-arm and fire again if it crosses back and forth across the target.

The rolling percent-move window is built from samples collected by this service. It is not backfilled from Bayse price history on startup.

## Possible Expansions

- Add notification delivery through webhooks, email, Slack, or queues.
- Add idempotency keys or a unique rule-definition constraint for duplicate rule creation.
- Add threshold hysteresis or cooldown periods to reduce alert flapping.
- Backfill percent-move windows from Bayse price history on startup.
- Use Bayse order books as a CLOB midpoint fallback when ticker data is unavailable.
- Add pagination and filtering endpoints for rules and alerts.
- Add rule disabling/deletion endpoints.
- Add integration tests against a temporary Postgres instance.
- Add WebSocket-based price updates if Bayse exposes a suitable stream.
