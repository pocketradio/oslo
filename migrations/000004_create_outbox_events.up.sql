-- saved with the ride so queue jobs are not lost
CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,
    ride_id UUID NOT NULL REFERENCES rides (id),
    event_type TEXT NOT NULL CHECK (btrim(event_type) <> ''),
    payload JSONB NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at TIMESTAMPTZ
);

CREATE INDEX outbox_events_unpublished
    ON outbox_events (created_at)
    WHERE published_at IS NULL;
