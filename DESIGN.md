# System Design

## Rate calculation engine

The rate engine is deliberately isolated as a pure function
(`rateengine.Calculate`) that takes physical inputs and an already-resolved
rate card, and returns a charge with no database or network access inside
it. This is the single most-evaluated piece of logic in the system, so it
needed to be trivially unit-testable — the tests cover volumetric-vs-actual
weight winning, prepaid vs. COD, and the missing-rate-card error path,
without touching a database.

The calculation itself follows the brief exactly: volumetric weight is
`(L × B × H) / 5000`; billable weight is the greater of actual and
volumetric, since a light-but-bulky package still occupies proportional
vehicle space; the charge is `base_fee + billable_weight × rate_per_kg`,
plus a flat `cod_surcharge` when the order is COD. The rate card itself is
resolved one layer up, in the orders handler, by a `(order_type,
from_zone_id, to_zone_id)` lookup against the `rate_cards` table — so
intra-zone and inter-zone rates, and B2B vs. B2C, are just different rows
in one table rather than parallel code paths. Nothing is hardcoded: base
fee, per-kg rate, and COD surcharge are all admin-configured. If no rate
card exists for a route/order-type combination, order creation fails with a
clear `422` rather than silently defaulting to some rate — a wrong price is
worse than a blocked order.

The customer-facing "charge shown before confirmation" requirement is
satisfied by sharing the exact same `resolveCharge` function between a
`/orders/preview` endpoint (side-effect-free) and `/orders` creation, so
there's no possibility of the preview number drifting from what the order
is actually created with.

## Zone detection

Zone detection is a direct pincode → zone lookup against an
admin-maintained `zone_areas` table (unique index on pincode), rather than
geocoding or polygon math. This was a conscious simplification against the
project's timeline: the brief's own admin workflow — "assigns areas to
zones" — describes exactly this table, and pincode-level granularity is how
Indian logistics platforms actually zone their networks in practice. It's
O(1), fully admin-configurable with no code changes, and has an obvious
failure mode (unmapped pincode → explicit error) rather than a fuzzy one
(nearest polygon match). The trade-off is that it can't express finer
sub-pincode geography or true distance-based rating — if that's ever
needed, `zone_areas` could gain a `lat/long` + radius mode without
disturbing the rate-card or order schema, since both only care about a
resolved `zone_id`.

## Auto-assignment logic

Auto-assignment reuses the same zone graph as the rate engine: an agent has
a home `zone_id`, and "nearest available agent" is modelled as "any
`available` agent whose zone matches the order's pickup zone," picked by
least-recently-assigned (`last_assigned_at ASC`) so load spreads round-robin
rather than always hitting the same agent. This runs inside the same
database transaction as the order's status change to `ASSIGNED`, using
`SELECT ... FOR UPDATE SKIP LOCKED` so two concurrent assignment attempts
can't race onto the same agent — a real concern once agents and admins are
both triggering assignment simultaneously.

This is a simplification of true nearest-neighbor GPS routing, chosen
because it needs no location-tracking infrastructure (no agent app pinging
coordinates, no distance math, no stale-location edge cases) while still
satisfying the actual requirement of "nearest available, by zone." An
agent's status flips `available → busy` on assignment and back to
`available` when their order reaches a terminal state for that attempt
(`DELIVERED` or `FAILED`). If the model needs live-GPS "true nearest" later,
it's an additive change — add `lat/long` to `agents`, add a
haversine-distance query as a second `AutoAssign` strategy — rather than a
rewrite, since the order/rate-card schema doesn't depend on how "nearest"
is defined.

## Failed delivery handling

A `FAILED` status is a normal, valid endpoint of the delivery lifecycle
(`OUT_FOR_DELIVERY → FAILED`), not an error state — it triggers the same
outbox notification path as every other transition, telling the customer
their delivery failed and inviting a reschedule. The customer (or admin)
then calls the reschedule endpoint with a new date; this is only legal
from `FAILED`, preventing accidental reschedules of in-flight orders.
Rescheduling logs a `reschedules` row (for audit — old date, new date,
reason, who requested it), sets `order_status_history` to `RESCHEDULED`,
clears the previous `agent_id`, and immediately re-runs the same
`AutoAssign` used at creation time against the order's pickup zone — so a
rescheduled order gets a fresh agent (not necessarily the one who failed
the first attempt) without any separate "reassignment" code path. If no
agent is free at that moment, the order sits at `RESCHEDULED` with no
agent, visible to admins via the orders filter, until one becomes
available or an admin assigns manually.
