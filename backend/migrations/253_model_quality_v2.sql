-- CUST-OBS-003：新版检测独立保存；升级暂停旧调度，并拒绝旧实例重新启用。
UPDATE model_quality_config SET enabled=FALSE,version=version+1 WHERE id=1;
ALTER TABLE model_quality_config ADD CONSTRAINT model_quality_v2_legacy_disabled CHECK (NOT enabled);
UPDATE settings SET value='false' WHERE key='model_quality_enabled';
CREATE TABLE model_quality_v2_config (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    version INTEGER NOT NULL DEFAULT 1,
    definition JSONB NOT NULL DEFAULT '{}'::jsonb
);
INSERT INTO model_quality_v2_config (id) VALUES (1);
CREATE TABLE model_quality_v2_rounds (
    id TEXT PRIMARY KEY,
    group_id BIGINT NOT NULL,
    group_name TEXT NOT NULL,
    interval_minutes INTEGER NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX model_quality_v2_round_active ON model_quality_v2_rounds(group_id) WHERE finished_at IS NULL;
CREATE INDEX model_quality_v2_round_retention ON model_quality_v2_rounds(started_at);
CREATE TABLE model_quality_v2_schedules (
    group_id BIGINT PRIMARY KEY,
    next_run_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE model_quality_v2_runs (
    id TEXT PRIMARY KEY,
    round_id TEXT NOT NULL REFERENCES model_quality_v2_rounds(id) ON DELETE CASCADE,
    target_id TEXT NOT NULL,
    model TEXT NOT NULL,
    protocol TEXT NOT NULL,
    reasoning_effort TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    outcome TEXT NOT NULL DEFAULT 'running'
);
CREATE INDEX model_quality_v2_run_order ON model_quality_v2_runs(started_at DESC,id DESC);
CREATE INDEX model_quality_v2_run_round ON model_quality_v2_runs(round_id);
CREATE TABLE model_quality_v2_tasks (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES model_quality_v2_runs(id) ON DELETE CASCADE,
    question_id TEXT NOT NULL,
    specification JSONB NOT NULL,
    result JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    verdict TEXT NOT NULL DEFAULT 'unknown',
    owner TEXT NOT NULL DEFAULT '',
    lease_until TIMESTAMPTZ,
    deadline_at TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    UNIQUE(run_id,question_id)
);
CREATE INDEX model_quality_v2_task_dispatch ON model_quality_v2_tasks(next_attempt_at,started_at) WHERE finished_at IS NULL;
