-- chat_messages: 管理者 ↔ 参加者 チャット
CREATE TABLE IF NOT EXISTS chat_messages (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    applicant_id  UUID NOT NULL REFERENCES applicants(id) ON DELETE CASCADE,
    sender        TEXT NOT NULL CHECK (sender IN ('admin', 'applicant')),
    body          TEXT NOT NULL,
    is_read       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_applicant_id ON chat_messages(applicant_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_created_at ON chat_messages(created_at);
