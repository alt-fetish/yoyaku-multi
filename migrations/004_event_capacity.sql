-- events: 定員をsession_type固定値ではなくイベントごとに持つ
ALTER TABLE events
    ADD COLUMN IF NOT EXISTS capacity INTEGER NOT NULL DEFAULT 2;

ALTER TABLE events
    DROP CONSTRAINT IF EXISTS events_capacity_positive;

ALTER TABLE events
    ADD CONSTRAINT events_capacity_positive CHECK (capacity > 0);
