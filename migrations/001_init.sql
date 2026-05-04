-- applicants: 参加希望者台帳
CREATE TABLE IF NOT EXISTS applicants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    handle      TEXT NOT NULL,
    email       TEXT NOT NULL UNIQUE,
    token       TEXT NOT NULL UNIQUE,
    status      TEXT NOT NULL DEFAULT 'active',  -- active / banned
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ng_settings: NG行為設定
CREATE TABLE IF NOT EXISTS ng_settings (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    applicant_id  UUID NOT NULL REFERENCES applicants(id) ON DELETE CASCADE,
    action_key    TEXT NOT NULL,
    -- 'finger_mouth' | 'anal' | 'sheath' | 'kiss' | 'fellatio'
    is_ok         BOOLEAN NOT NULL DEFAULT FALSE,
    UNIQUE (applicant_id, action_key)
);

-- option_sets: 開催イベントごとに割り当てるオプションセット
CREATE TABLE IF NOT EXISTS option_sets (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT NOT NULL UNIQUE,
    description   TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- option_items: 各オプションセット内の選択肢
CREATE TABLE IF NOT EXISTS option_items (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    option_set_id UUID NOT NULL REFERENCES option_sets(id) ON DELETE CASCADE,
    label         TEXT NOT NULL,
    sort_order    INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_option_items_set_label ON option_items(option_set_id, label);

-- events: 開催日
CREATE TABLE IF NOT EXISTS events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_type  TEXT NOT NULL CHECK (session_type IN ('solo', 'group')),
    event_date    DATE NOT NULL,
    event_time    TIME NOT NULL,
    end_time      TIME,
    capacity      INTEGER NOT NULL DEFAULT 2 CHECK (capacity > 0),
    option_set_id UUID REFERENCES option_sets(id),
    status        TEXT NOT NULL DEFAULT 'open',
    -- open / confirmed / done / cancelled
    notes         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (event_date, event_time)
);

CREATE INDEX IF NOT EXISTS idx_events_date ON events(event_date DESC);
CREATE INDEX IF NOT EXISTS idx_events_status ON events(status);

-- event_entries: 参加表明（中間テーブル）
CREATE TABLE IF NOT EXISTS event_entries (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id      UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    applicant_id  UUID NOT NULL REFERENCES applicants(id),
    status        TEXT NOT NULL DEFAULT 'pending',
    -- pending / confirmed / declined / cancelled
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (event_id, applicant_id)
);

CREATE INDEX IF NOT EXISTS idx_event_entries_event_id ON event_entries(event_id);
CREATE INDEX IF NOT EXISTS idx_event_entries_applicant_id ON event_entries(applicant_id);

-- event_entry_option_selections: イベント申込ごとのオプション選択
CREATE TABLE IF NOT EXISTS event_entry_option_selections (
    event_entry_id UUID NOT NULL REFERENCES event_entries(id) ON DELETE CASCADE,
    option_item_id UUID NOT NULL REFERENCES option_items(id) ON DELETE CASCADE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_entry_id, option_item_id)
);
