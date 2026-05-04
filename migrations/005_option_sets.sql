CREATE TABLE IF NOT EXISTS option_sets (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT NOT NULL UNIQUE,
    description   TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS option_items (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    option_set_id UUID NOT NULL REFERENCES option_sets(id) ON DELETE CASCADE,
    label         TEXT NOT NULL,
    sort_order    INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_option_items_set_label ON option_items(option_set_id, label);

CREATE TABLE IF NOT EXISTS event_entry_option_selections (
    event_entry_id UUID NOT NULL REFERENCES event_entries(id) ON DELETE CASCADE,
    option_item_id UUID NOT NULL REFERENCES option_items(id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_entry_id, option_item_id)
);

ALTER TABLE events
    ADD COLUMN IF NOT EXISTS option_set_id UUID;

DO $$
DECLARE
    default_set_id UUID;
BEGIN
    INSERT INTO option_sets (name, description)
    VALUES ('デフォルトオプション', '既存のNG設定から移行した初期セット')
    ON CONFLICT (name) DO NOTHING;

    SELECT id INTO default_set_id
    FROM option_sets
    WHERE name = 'デフォルトオプション';

    INSERT INTO option_items (option_set_id, label, sort_order)
    VALUES
        (default_set_id, '指を口に入れる', 0),
        (default_set_id, 'アナルに触れる', 1),
        (default_set_id, 'シース（陰茎部ラバー）を外す', 2),
        (default_set_id, 'キス', 3),
        (default_set_id, 'フェラチオ', 4)
    ON CONFLICT (option_set_id, label) DO NOTHING;

    UPDATE events
    SET option_set_id = default_set_id
    WHERE option_set_id IS NULL;
END $$;

ALTER TABLE events
    ALTER COLUMN option_set_id SET NOT NULL;

ALTER TABLE events
    DROP CONSTRAINT IF EXISTS events_option_set_id_fkey;

ALTER TABLE events
    ADD CONSTRAINT events_option_set_id_fkey
    FOREIGN KEY (option_set_id) REFERENCES option_sets(id);

INSERT INTO event_entry_option_selections (event_entry_id, option_item_id)
SELECT ee.id, oi.id
FROM event_entries ee
JOIN events e ON e.id = ee.event_id
JOIN option_sets os ON os.id = e.option_set_id
JOIN applicants a ON a.id = ee.applicant_id
JOIN ng_settings ns ON ns.applicant_id = a.id AND ns.is_ok = TRUE
JOIN option_items oi ON oi.option_set_id = os.id
WHERE os.name = 'デフォルトオプション'
  AND (
        (ns.action_key = 'finger_mouth' AND oi.label = '指を口に入れる') OR
        (ns.action_key = 'anal' AND oi.label = 'アナルに触れる') OR
        (ns.action_key = 'sheath' AND oi.label = 'シース（陰茎部ラバー）を外す') OR
        (ns.action_key = 'kiss' AND oi.label = 'キス') OR
        (ns.action_key = 'fellatio' AND oi.label = 'フェラチオ')
      )
ON CONFLICT DO NOTHING;
