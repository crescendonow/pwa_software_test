CREATE TABLE IF NOT EXISTS ut_logs.freetext_query_audit (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    uid TEXT NOT NULL DEFAULT '',
    uname TEXT NOT NULL DEFAULT '',
    prompt TEXT NOT NULL,
    generated_sql TEXT,
    status TEXT NOT NULL,
    error_message TEXT NOT NULL DEFAULT '',
    duration_ms BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_freetext_query_audit_status
        CHECK (status IN ('success', 'rejected', 'error'))
);

CREATE INDEX IF NOT EXISTS idx_freetext_query_audit_created_at
    ON ut_logs.freetext_query_audit(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_freetext_query_audit_uid
    ON ut_logs.freetext_query_audit(uid, created_at DESC);
