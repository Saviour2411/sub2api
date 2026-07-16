-- 将同步上游分组绑定到本地分组账号，用于按上游倍率维护账号全局优先级。
CREATE TABLE IF NOT EXISTS upstream_group_account_bindings (
    id                 BIGSERIAL PRIMARY KEY,
    upstream_group_id  BIGINT NOT NULL REFERENCES upstream_groups(id) ON DELETE CASCADE,
    local_group_id     BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    account_id         BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT upstream_group_account_bindings_account_unique UNIQUE (account_id)
);

CREATE INDEX IF NOT EXISTS idx_upstream_group_account_bindings_upstream_group_id
    ON upstream_group_account_bindings (upstream_group_id);

CREATE INDEX IF NOT EXISTS idx_upstream_group_account_bindings_local_group_id
    ON upstream_group_account_bindings (local_group_id);
