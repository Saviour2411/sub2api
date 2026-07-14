-- 二开功能：独立上游站点管理、当前分组指标与每日历史。
CREATE TABLE IF NOT EXISTS upstream_sites (
    id                    BIGSERIAL PRIMARY KEY,
    name                  VARCHAR(100) NOT NULL,
    base_url              VARCHAR(500) NOT NULL,
    platform              VARCHAR(20) NOT NULL,
    auth_mode             VARCHAR(20) NOT NULL,
    account               VARCHAR(255) NOT NULL DEFAULT '',
    credential_encrypted  TEXT NOT NULL,
    enabled               BOOLEAN NOT NULL DEFAULT TRUE,
    status                VARCHAR(20) NOT NULL DEFAULT 'pending',
    error_message         VARCHAR(500),
    balance_usd           DOUBLE PRECISION,
    today_tokens          BIGINT NOT NULL DEFAULT 0,
    today_cost_usd        DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_tokens          BIGINT NOT NULL DEFAULT 0,
    total_cost_usd        DOUBLE PRECISION NOT NULL DEFAULT 0,
    tracking_started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_synced_at        TIMESTAMPTZ,
    next_sync_at          TIMESTAMPTZ,
    created_by            BIGINT NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT upstream_sites_platform_check CHECK (platform IN ('sub2api', 'newapi')),
    CONSTRAINT upstream_sites_auth_mode_check CHECK (auth_mode IN ('password', 'token')),
    CONSTRAINT upstream_sites_status_check CHECK (status IN ('pending', 'syncing', 'healthy', 'error'))
);

CREATE INDEX IF NOT EXISTS idx_upstream_sites_enabled_next_sync
    ON upstream_sites (enabled, next_sync_at);
CREATE INDEX IF NOT EXISTS idx_upstream_sites_platform ON upstream_sites (platform);
CREATE INDEX IF NOT EXISTS idx_upstream_sites_status ON upstream_sites (status);
CREATE INDEX IF NOT EXISTS idx_upstream_sites_created_at ON upstream_sites (created_at DESC);

CREATE TABLE IF NOT EXISTS upstream_groups (
    id                BIGSERIAL PRIMARY KEY,
    site_id           BIGINT NOT NULL REFERENCES upstream_sites(id) ON DELETE CASCADE,
    remote_id         VARCHAR(100) NOT NULL,
    name              VARCHAR(100) NOT NULL,
    platform          VARCHAR(50) NOT NULL DEFAULT '',
    multiplier        DOUBLE PRECISION,
    today_tokens      BIGINT NOT NULL DEFAULT 0,
    today_cost_usd    DOUBLE PRECISION NOT NULL DEFAULT 0,
    last_synced_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT upstream_groups_site_remote_unique UNIQUE (site_id, remote_id)
);

CREATE INDEX IF NOT EXISTS idx_upstream_groups_site_name
    ON upstream_groups (site_id, name);

CREATE TABLE IF NOT EXISTS upstream_daily_stats (
    id            BIGSERIAL PRIMARY KEY,
    site_id       BIGINT NOT NULL REFERENCES upstream_sites(id) ON DELETE CASCADE,
    usage_date    DATE NOT NULL,
    balance_usd   DOUBLE PRECISION,
    tokens        BIGINT NOT NULL DEFAULT 0,
    cost_usd      DOUBLE PRECISION NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT upstream_daily_stats_site_date_unique UNIQUE (site_id, usage_date)
);

CREATE INDEX IF NOT EXISTS idx_upstream_daily_stats_usage_date
    ON upstream_daily_stats (usage_date DESC);
