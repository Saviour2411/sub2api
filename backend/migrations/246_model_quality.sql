-- CUST-OBS-003：降智检测配置、轮次及独立测试结果。
CREATE TABLE IF NOT EXISTS model_quality_config (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    version INTEGER NOT NULL DEFAULT 1,
    groups JSONB NOT NULL DEFAULT '[]'::jsonb
);
INSERT INTO model_quality_config (id) VALUES (1) ON CONFLICT DO NOTHING;
INSERT INTO settings (key, value) VALUES ('model_quality_enabled', 'false') ON CONFLICT (key) DO NOTHING;

CREATE TABLE IF NOT EXISTS model_quality_rounds (
    id TEXT PRIMARY KEY,
    group_id BIGINT NOT NULL,
    group_name TEXT NOT NULL,
    parameters JSONB NOT NULL,
    endpoint TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS model_quality_round_active ON model_quality_rounds (group_id) WHERE finished_at IS NULL;
CREATE INDEX IF NOT EXISTS model_quality_round_retention ON model_quality_rounds (started_at);
CREATE TABLE IF NOT EXISTS model_quality_schedules (
    group_id BIGINT PRIMARY KEY,
    next_run_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE IF NOT EXISTS model_quality_runs (
    id BIGSERIAL PRIMARY KEY,
    round_id TEXT NOT NULL REFERENCES model_quality_rounds(id) ON DELETE CASCADE,
    model TEXT NOT NULL,
    reasoning_effort TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    outcome TEXT NOT NULL DEFAULT 'running'
);
CREATE INDEX IF NOT EXISTS model_quality_run_order ON model_quality_runs (started_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS model_quality_run_round ON model_quality_runs (round_id);
CREATE INDEX IF NOT EXISTS model_quality_run_active ON model_quality_runs (id) WHERE finished_at IS NULL;
CREATE TABLE IF NOT EXISTS model_quality_tasks (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES model_quality_runs(id) ON DELETE CASCADE,
    benchmark TEXT NOT NULL CHECK (benchmark IN ('candy', 'pelican')),
    status TEXT NOT NULL DEFAULT 'pending',
    verdict TEXT NOT NULL DEFAULT 'unknown',
    remote_id TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL UNIQUE,
    next_poll_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    submitted_at TIMESTAMPTZ,
    deadline_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    answer TEXT NOT NULL DEFAULT '',
    html TEXT NOT NULL DEFAULT '',
    duration_ms BIGINT NOT NULL DEFAULT 0,
    input_tokens BIGINT,
    output_tokens BIGINT,
    reasoning_tokens BIGINT,
    UNIQUE (run_id, benchmark)
);
CREATE INDEX IF NOT EXISTS model_quality_task_pending ON model_quality_tasks (next_poll_at) WHERE status IN ('pending', 'submitting', 'running', 'uncertain');
