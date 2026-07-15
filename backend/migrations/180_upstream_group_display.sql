-- 保存管理员选择展示的上游分组，并保留暂时从上游消失的已展示分组。
ALTER TABLE upstream_groups
    ADD COLUMN IF NOT EXISTS displayed BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE upstream_groups
    ADD COLUMN IF NOT EXISTS available BOOLEAN NOT NULL DEFAULT TRUE;

CREATE INDEX IF NOT EXISTS idx_upstream_groups_site_displayed
    ON upstream_groups (site_id, displayed);

CREATE INDEX IF NOT EXISTS idx_upstream_groups_site_available_name
    ON upstream_groups (site_id, available, name);
