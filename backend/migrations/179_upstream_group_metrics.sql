-- 上游监控改用实际扣费口径，并记录分组描述与倍率变化历史。
ALTER TABLE upstream_groups
	ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';

ALTER TABLE upstream_daily_stats
    ADD COLUMN IF NOT EXISTS cost_basis_version INTEGER NOT NULL DEFAULT 1;

COMMENT ON COLUMN upstream_daily_stats.cost_basis_version IS
    '成本口径版本：1=历史标准成本或未知口径，2=上游实际扣费';

-- NewAPI 的既有 cost_usd 一直来自 quota / quota_per_unit，已是实际扣费口径。
UPDATE upstream_daily_stats AS stats
SET cost_basis_version = 2
FROM upstream_sites AS sites
WHERE sites.id = stats.site_id
  AND sites.platform = 'newapi'
  AND stats.cost_basis_version < 2;

-- Sub2API 旧汇总可能来自标准成本；实际扣费回填完成前宁可显示 0，也不能继续冒充实际消耗。
UPDATE upstream_sites
SET today_cost_usd = 0,
    total_cost_usd = 0,
    updated_at = NOW()
WHERE platform = 'sub2api';

CREATE TABLE IF NOT EXISTS upstream_group_multiplier_history (
    id            BIGSERIAL PRIMARY KEY,
    site_id       BIGINT NOT NULL REFERENCES upstream_sites(id) ON DELETE CASCADE,
    remote_id     VARCHAR(100) NOT NULL,
    name          VARCHAR(100) NOT NULL,
    platform      VARCHAR(50) NOT NULL DEFAULT '',
	description   TEXT NOT NULL DEFAULT '',
    multiplier    DOUBLE PRECISION,
    recorded_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_upstream_group_multiplier_history_site_remote_recorded
    ON upstream_group_multiplier_history (site_id, remote_id, recorded_at);

-- 升级时用当前分组生成第一条倍率基线；迁移重复执行也不会追加重复基线。
INSERT INTO upstream_group_multiplier_history (
    site_id, remote_id, name, platform, description, multiplier, recorded_at
)
SELECT
    groups.site_id,
    groups.remote_id,
    groups.name,
    groups.platform,
    groups.description,
    groups.multiplier,
    NOW()
FROM upstream_groups AS groups
WHERE NOT EXISTS (
    SELECT 1
    FROM upstream_group_multiplier_history AS history
    WHERE history.site_id = groups.site_id
      AND history.remote_id = groups.remote_id
);

-- Sub2API 旧历史口径未知，启用站点在升级后立即进入实际扣费回填队列。
UPDATE upstream_sites
SET status = 'pending',
    error_message = NULL,
    next_sync_at = NOW(),
    updated_at = NOW()
WHERE platform = 'sub2api'
  AND enabled = TRUE;
