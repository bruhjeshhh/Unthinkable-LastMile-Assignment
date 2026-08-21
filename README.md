# Last-Mile Delivery Tracker

A delivery management platform: customers create orders with auto-calculated
charges, orders are assigned to delivery agents (manually or automatically),
and customers get notified at every step — including a reschedule flow for
failed deliveries.

## Stack

- **Backend:** Go (standard `net/http`, Go 1.22+ pattern-based routing — no
  web framework), `pgx/v5` for Postgres, hand-rolled HMAC-signed auth tokens
  (no external JWT dependency), `bcrypt` for passwords.
- **Database:** PostgreSQL.
- **Frontend:** React + Vite, plain CSS (no UI framework).
- **Notifications:** outbox table + background polling worker. Email via
  SMTP (or logged to stdout if unconfigured); SMS is mocked (logged) by
  default since paid SMS providers need phone verification — swap in a real
  provider by implementing the `SMSSender` interface.

No microservices, Kafka, or Redis — see [DESIGN.md](DESIGN.md) for why, given
the scope and timeline of this build.

## Project structure

```
backend/
  cmd/api/main.go            entrypoint
  internal/
    config/                  env-driven config, no hardcoded values
    db/                      pgx pool + SQL migrations
    models/                  shared domain types
    auth/                    bcrypt, signed tokens, middleware
    rateengine/               pure charge-calculation function + unit tests
    zones/                   pincode->zone lookup, rate card CRUD
    assignment/              auto-assignment (nearest available agent)
    notifications/           outbox, email/SMS senders, background worker
    httpapi/                 HTTP handlers + router
frontend/
  src/
    api/client.js            fetch wrapper
    pages/                   Login, Register, Customer/Agent/Admin dashboards
    components/               Shell, StatusPill, Timeline
```

## Setup guide

### 1. Database

```bash
createdb delivery
psql delivery -f backend/internal/db/migrations/0001_init.sql
```

(Or point `DATABASE_URL` at a hosted Postgres instance — Railway/Render both
provision one for free.)

### 2. Backend

```bash
cd backend
cp .env.example .env    # edit DATABASE_URL / JWT_SECRET at minimum
go mod tidy              # fetches pgx, bcrypt, etc.
go run ./cmd/api
```

Server starts on `:8080` (or `$PORT`). `go test ./...` runs the rate-engine
unit tests.

**First admin account:** the public register endpoint only allows
`customer`/`agent` roles. Register a normal account, then promote it:

```sql
UPDATE users SET role = 'admin' WHERE email = 'you@example.com';
```

Then, as admin, configure at least one zone, map a few pincodes to it, and
add rate cards — orders can't be created for a route with no zone mapping or
rate card (by design, so nothing is silently free or guessed).

### 3. Frontend

```bash
cd frontend
cp .env.example .env    # VITE_API_URL, defaults to http://localhost:8080
npm install
npm run dev
```

### 4. Deploying

- **Backend:** Railway/Render — set the four core env vars
  (`DATABASE_URL`, `JWT_SECRET`, plus SMTP vars if you want real emails),
  point the build at `backend/`, run `go run ./cmd/api` or build a binary.
- **DB:** Railway/Render managed Postgres, run the migration once via `psql`.
- **Frontend:** Vercel/Netlify/Railway static build (`npm run build`, serve
  `dist/`), with `VITE_API_URL` pointing at the deployed backend.

## Environment variables

| Variable | Where | Purpose |
|---|---|---|
| `PORT` | backend | HTTP port (default 8080) |
| `DATABASE_URL` | backend | Postgres connection string |
| `JWT_SECRET` | backend | HMAC signing key for auth tokens |
| `SMTP_HOST/PORT/USER/PASS`, `FROM_EMAIL` | backend | Email delivery; blank `SMTP_HOST` logs emails to stdout instead |
| `SMS_MOCK` | backend | `true` logs SMS instead of sending (default) |
| `VITE_API_URL` | frontend | Backend base URL |

## API docs

All authenticated routes take `Authorization: Bearer <token>`.

### Auth
| Method | Path | Access | Body |
|---|---|---|---|
| POST | `/api/auth/register` | public | `{name, email, phone, password, role: customer\|agent, zone_id?}` |
| POST | `/api/auth/login` | public | `{email, password}` |

### Orders
| Method | Path | Access | Notes |
|---|---|---|---|
| POST | `/api/orders/preview` | customer/admin | Returns computed charge without creating an order |
| POST | `/api/orders` | customer/admin | Creates the order; admin may pass `customer_id` to place on a customer's behalf |
| GET | `/api/orders` | any | Customers see their own; agents see their assigned orders; admin can filter with `?status=&zone_id=&agent_id=` |
| GET | `/api/orders/{id}` | owner/assigned agent/admin | Returns `{order, history}` — `history` is the full immutable timeline |
| PATCH | `/api/orders/{id}/status` | assigned agent | `{status, note?}` — validated against the allowed lifecycle transitions |
| POST | `/api/orders/{id}/reschedule` | owning customer/admin | `{new_date, reason?}` — only valid from `FAILED`; re-triggers auto-assignment |

Order creation body:
```json
{
  "pickup_address": "...", "pickup_pincode": "110001",
  "drop_address": "...", "drop_pincode": "400001",
  "length_cm": 30, "breadth_cm": 20, "height_cm": 10,
  "actual_weight_kg": 2,
  "order_type": "B2C", "payment_type": "COD"
}
```

### Admin
| Method | Path | Body |
|---|---|---|
| GET | `/api/zones` | — |
| POST | `/api/admin/zones` | `{name}` |
| POST | `/api/admin/zones/map-pincode` | `{zone_id, pincode, label?}` |
| GET / POST | `/api/admin/rate-cards` | `{order_type, from_zone_id, to_zone_id, base_fee, rate_per_kg, cod_surcharge}` |
| GET | `/api/admin/agents` | — |
| PATCH | `/api/agents/{id}/availability` | `{status?, zone_id?}` (agent can update their own; admin any) |
| POST | `/api/admin/orders/{id}/assign` | `{agent_id?}` — omit for auto-assignment |
| PATCH | `/api/admin/orders/{id}/override` | `{status, note?}` — bypasses lifecycle validation |

## Database schema

See `backend/internal/db/migrations/0001_init.sql` for the full DDL. Key
design points:

- **`zone_areas`** is the admin-configurable pincode → zone map that powers
  zone detection — a unique index on `pincode` makes lookup O(1).
- **`rate_cards`** is unique on `(order_type, from_zone_id, to_zone_id)`,
  so intra-zone (`from = to`) and inter-zone rates are just different rows
  in the same table, and B2B/B2C never share a row.
- **`order_status_history`** is append-only and is the source of truth for
  the tracking timeline; `orders.status` is a denormalized "current status"
  column kept in sync for fast filtering/listing.
- **`notifications`** is an outbox table — every status change enqueues a
  row in the same DB transaction as the status update, so a notification is
  never lost even if the process crashes right after committing.

## Rate calculation logic

Implemented as a pure function in `internal/rateengine/rateengine.go`
(fully unit-tested in `rateengine_test.go`), called from the order-creation
and preview handlers:

1. **Zone detection:** look up `pickup_pincode` and `drop_pincode` in
   `zone_areas` → `from_zone_id`, `to_zone_id`.
2. **Volumetric weight:** `(L × B × H) / 5000`.
3. **Billable weight:** `max(actual_weight, volumetric_weight)`.
4. **Rate card lookup:** fetch the `rate_cards` row for
   `(order_type, from_zone_id, to_zone_id)` — a missing row is a hard
   error (`422`), never a silent fallback.
5. **Charge:** `base_fee + billable_weight × rate_per_kg`, plus
   `cod_surcharge` if `payment_type = COD`.

Nothing here is hardcoded — base fee, per-kg rate, and COD surcharge are all
admin-configured per `(order_type, from_zone, to_zone)` via the rate-card
endpoints/UI.
