
CREATE TABLE queries (
    id BIGSERIAL PRIMARY KEY,
    query_id TEXT NOT NULL,
    db_user TEXT NOT NULL,
    normalized_query TEXT NOT NULL,
    executions_count BIGINT NOT NULL DEFAULT 0,
    mean_time_ms DOUBLE PRECISION NOT NULL DEFAULT 0,
    m2 DOUBLE PRECISION NOT NULL DEFAULT 0,
    last_execution_at TIMESTAMPTZ,
    last_anomaly_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (query_id, db_user)
);