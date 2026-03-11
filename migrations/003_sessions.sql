-- sessions: セッション履歴
CREATE TABLE IF NOT EXISTS sessions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_type  TEXT NOT NULL CHECK (session_type IN ('solo', 'group')),
    session_date  DATE NOT NULL,
    session_time  TIME NOT NULL,
    completed_at  TIMESTAMPTZ,
    status        TEXT NOT NULL DEFAULT 'confirmed',
    -- confirmed / done / cancelled
    notes         TEXT,
    event_id      UUID REFERENCES events(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_date ON sessions(session_date DESC);

-- session_participants: セッション参加者（多対多）
CREATE TABLE IF NOT EXISTS session_participants (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id    UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    applicant_id  UUID NOT NULL REFERENCES applicants(id),
    UNIQUE (session_id, applicant_id)
);

CREATE INDEX IF NOT EXISTS idx_session_participants_session_id ON session_participants(session_id);
CREATE INDEX IF NOT EXISTS idx_session_participants_applicant_id ON session_participants(applicant_id);
