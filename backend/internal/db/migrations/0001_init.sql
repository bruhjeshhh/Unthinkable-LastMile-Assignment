-- Last-Mile Delivery Tracker: initial schema

CREATE TYPE user_role AS ENUM ('customer', 'agent', 'admin');
CREATE TYPE order_type AS ENUM ('B2B', 'B2C');
CREATE TYPE payment_type AS ENUM ('PREPAID', 'COD');
CREATE TYPE agent_status AS ENUM ('available', 'busy', 'offline');
CREATE TYPE order_status AS ENUM (
  'CREATED', 'ASSIGNED', 'PICKED_UP', 'IN_TRANSIT',
  'OUT_FOR_DELIVERY', 'DELIVERED', 'FAILED', 'RESCHEDULED', 'CANCELLED'
);
CREATE TYPE notification_channel AS ENUM ('EMAIL', 'SMS');
CREATE TYPE notification_status AS ENUM ('PENDING', 'SENT', 'FAILED');

CREATE TABLE users (
  id            BIGSERIAL PRIMARY KEY,
  role          user_role NOT NULL,
  name          TEXT NOT NULL,
  email         TEXT NOT NULL UNIQUE,
  phone         TEXT,
  password_hash TEXT NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Zones are admin-defined logical regions (e.g. "North Delhi", "Central Mumbai")
CREATE TABLE zones (
  id         BIGSERIAL PRIMARY KEY,
  name       TEXT NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Admin maps pincodes/areas to zones. A pincode maps to exactly one zone.
CREATE TABLE zone_areas (
  id       BIGSERIAL PRIMARY KEY,
  zone_id  BIGINT NOT NULL REFERENCES zones(id) ON DELETE CASCADE,
  pincode  TEXT NOT NULL UNIQUE,
  label    TEXT -- optional human-readable area name
);

-- Rate cards: one row per (order_type, from_zone, to_zone). from = to means intra-zone.
CREATE TABLE rate_cards (
  id             BIGSERIAL PRIMARY KEY,
  order_type     order_type NOT NULL,
  from_zone_id   BIGINT NOT NULL REFERENCES zones(id) ON DELETE CASCADE,
  to_zone_id     BIGINT NOT NULL REFERENCES zones(id) ON DELETE CASCADE,
  base_fee       NUMERIC(10,2) NOT NULL DEFAULT 0,
  rate_per_kg    NUMERIC(10,2) NOT NULL,
  cod_surcharge  NUMERIC(10,2) NOT NULL DEFAULT 0, -- flat surcharge applied when payment_type = COD
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (order_type, from_zone_id, to_zone_id)
);

-- Delivery agents. One row per agent user, plus their assignment state.
CREATE TABLE agents (
  user_id       BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  zone_id       BIGINT REFERENCES zones(id),
  status        agent_status NOT NULL DEFAULT 'offline',
  last_assigned_at TIMESTAMPTZ, -- used for round-robin selection among available agents
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE orders (
  id                  BIGSERIAL PRIMARY KEY,
  customer_id         BIGINT NOT NULL REFERENCES users(id),
  created_by          BIGINT NOT NULL REFERENCES users(id), -- customer themself, or admin creating on their behalf
  pickup_address      TEXT NOT NULL,
  pickup_pincode      TEXT NOT NULL,
  drop_address        TEXT NOT NULL,
  drop_pincode        TEXT NOT NULL,
  length_cm           NUMERIC(10,2) NOT NULL,
  breadth_cm          NUMERIC(10,2) NOT NULL,
  height_cm           NUMERIC(10,2) NOT NULL,
  actual_weight_kg     NUMERIC(10,3) NOT NULL,
  volumetric_weight_kg NUMERIC(10,3) NOT NULL,
  billable_weight_kg   NUMERIC(10,3) NOT NULL,
  order_type          order_type NOT NULL,
  payment_type        payment_type NOT NULL,
  from_zone_id        BIGINT NOT NULL REFERENCES zones(id),
  to_zone_id          BIGINT NOT NULL REFERENCES zones(id),
  rate_card_id        BIGINT NOT NULL REFERENCES rate_cards(id),
  charge              NUMERIC(10,2) NOT NULL,
  status              order_status NOT NULL DEFAULT 'CREATED',
  agent_id            BIGINT REFERENCES agents(user_id),
  scheduled_date      DATE, -- current delivery attempt date (set/reset on reschedule)
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_from_zone ON orders(from_zone_id);
CREATE INDEX idx_orders_agent ON orders(agent_id);
CREATE INDEX idx_orders_customer ON orders(customer_id);

-- Append-only. Every status transition (and assignment change) is logged here.
-- orders.status is a derived/denormalized "current" pointer; this table is the source of truth.
CREATE TABLE order_status_history (
  id          BIGSERIAL PRIMARY KEY,
  order_id    BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  status      order_status NOT NULL,
  actor_id    BIGINT REFERENCES users(id),
  actor_role  user_role NOT NULL,
  note        TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_history_order ON order_status_history(order_id, created_at);

-- Failed-delivery reschedule requests
CREATE TABLE reschedules (
  id            BIGSERIAL PRIMARY KEY,
  order_id      BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  old_date      DATE,
  new_date      DATE NOT NULL,
  reason        TEXT,
  requested_by  BIGINT NOT NULL REFERENCES users(id),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Outbox pattern: every customer-facing event enqueues a notification row here.
-- A background worker polls PENDING rows and dispatches via the given channel.
CREATE TABLE notifications (
  id          BIGSERIAL PRIMARY KEY,
  order_id    BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  user_id     BIGINT NOT NULL REFERENCES users(id),
  channel     notification_channel NOT NULL,
  subject     TEXT,
  body        TEXT NOT NULL,
  status      notification_status NOT NULL DEFAULT 'PENDING',
  attempts    INT NOT NULL DEFAULT 0,
  last_error  TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  sent_at     TIMESTAMPTZ
);
CREATE INDEX idx_notifications_status ON notifications(status);
