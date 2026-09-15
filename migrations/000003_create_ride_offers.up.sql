CREATE TYPE offer_status AS ENUM (
    'pending',
    'accepted',
    'rejected',
    'expired'
);

CREATE TABLE ride_offers (
    id UUID PRIMARY KEY,
    ride_id UUID NOT NULL REFERENCES rides (id),
    driver_id UUID NOT NULL REFERENCES users (id),
    status offer_status NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMPTZ NOT NULL,
    responded_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (expires_at > created_at),
    CHECK (
        (
            status = 'pending'
            AND responded_at IS NULL
        )
        OR (
            status <> 'pending'
            AND responded_at IS NOT NULL
        )
    )
);

CREATE UNIQUE INDEX ride_offers_driver_attempt_unique ON ride_offers (ride_id, driver_id);
-- to not offer same ride to the same driver twice

-- keeps a ride from having multiple open offers
CREATE UNIQUE INDEX ride_offers_one_open_per_ride ON ride_offers (ride_id)
WHERE
    status IN ('pending', 'accepted');

-- keeps a driver from receiving multiple offers at once
CREATE UNIQUE INDEX ride_offers_one_pending_per_driver ON ride_offers (driver_id)
WHERE
    status = 'pending';
