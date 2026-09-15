CREATE TYPE ride_status AS ENUM (
    'requested',
    'matching',
    'offered',
    'assigned',
    'driver_arriving',
    'in_progress',
    'completed',
    'cancelled',
    'failed'
);

CREATE TABLE rides (
    id UUID PRIMARY KEY,
    rider_id UUID NOT NULL REFERENCES users(id),
    driver_id UUID REFERENCES users(id),
    status ride_status NOT NULL DEFAULT 'requested',
    pickup_latitude DOUBLE PRECISION NOT NULL CHECK (pickup_latitude BETWEEN -90 AND 90),
    pickup_longitude DOUBLE PRECISION NOT NULL CHECK (pickup_longitude BETWEEN -180 AND 180),
    destination_latitude DOUBLE PRECISION NOT NULL CHECK (destination_latitude BETWEEN -90 AND 90),
    destination_longitude DOUBLE PRECISION NOT NULL CHECK (destination_longitude BETWEEN -180 AND 180),
    fare_cents BIGINT NOT NULL CHECK (fare_cents >= 0),
    idempotency_key TEXT NOT NULL CHECK (btrim(idempotency_key) <> ''),
    matching_deadline TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (driver_id IS NULL OR driver_id <> rider_id) -- prevents someone from being both rider and driver for same ride
);

CREATE UNIQUE INDEX rides_idempotency_unique
    ON rides (rider_id, idempotency_key);

CREATE UNIQUE INDEX rides_one_active_ride_per_rider
    ON rides (rider_id)
    WHERE status IN ('requested', 'matching', 'offered', 'assigned', 'driver_arriving', 'in_progress');

CREATE UNIQUE INDEX rides_one_active_ride_per_driver
    ON rides (driver_id)
    WHERE status IN ('assigned', 'driver_arriving', 'in_progress');
