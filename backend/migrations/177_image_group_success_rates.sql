-- Image 分组成功率使用“统计代次”隔离清零与并发写入。
CREATE TABLE IF NOT EXISTS image_group_success_rate_state (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    generation BIGINT NOT NULL DEFAULT 1 CHECK (generation > 0),
    reset_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO image_group_success_rate_state (id, generation, reset_at)
VALUES (1, 1, NOW())
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS image_group_success_rate_stats (
    generation BIGINT NOT NULL CHECK (generation > 0),
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    request_count BIGINT NOT NULL DEFAULT 0 CHECK (request_count >= 0),
    failure_count BIGINT NOT NULL DEFAULT 0 CHECK (failure_count >= 0 AND failure_count <= request_count),
    last_success_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (generation, group_id)
);

CREATE INDEX IF NOT EXISTS image_group_success_rate_stats_group_idx
    ON image_group_success_rate_stats (group_id, generation DESC);

-- 批量任务可能因 Worker 重试重复进入终态，事件键用于保证每批只统计一次。
CREATE TABLE IF NOT EXISTS image_group_success_rate_events (
    event_key VARCHAR(160) PRIMARY KEY,
    generation BIGINT NOT NULL CHECK (generation > 0),
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS image_group_success_rate_events_created_idx
    ON image_group_success_rate_events (created_at);

CREATE INDEX IF NOT EXISTS image_group_success_rate_events_generation_idx
    ON image_group_success_rate_events (generation);

-- 批量任务保存提交时命中的真实分组，避免任务完成前 API Key 改组导致统计串组。
ALTER TABLE batch_image_jobs
    ADD COLUMN IF NOT EXISTS group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL;

UPDATE batch_image_jobs AS jobs
SET group_id = keys.group_id
FROM api_keys AS keys
WHERE jobs.group_id IS NULL
  AND jobs.api_key_id = keys.id
  AND keys.group_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS batch_image_jobs_group_id_idx
    ON batch_image_jobs (group_id)
    WHERE group_id IS NOT NULL;
