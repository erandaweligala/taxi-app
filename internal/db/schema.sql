-- Durable trip records. The Kafka trip-event stream is the source of truth;
-- this table is the materialized, queryable projection of it.
CREATE TABLE IF NOT EXISTS trips (
    id         TEXT PRIMARY KEY,
    rider_id   TEXT NOT NULL,
    driver_id  TEXT NOT NULL DEFAULT '',
    region     TEXT NOT NULL,
    state      TEXT NOT NULL,
    surge      DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS trips_rider_idx  ON trips (rider_id);
CREATE INDEX IF NOT EXISTS trips_driver_idx ON trips (driver_id);
CREATE INDEX IF NOT EXISTS trips_state_idx  ON trips (state);

-- Append-only audit log of every state transition, for replay and analytics.
CREATE TABLE IF NOT EXISTS trip_events (
    id        BIGSERIAL PRIMARY KEY,
    trip_id   TEXT NOT NULL,
    state     TEXT NOT NULL,
    driver_id TEXT NOT NULL DEFAULT '',
    at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS trip_events_trip_idx ON trip_events (trip_id);
