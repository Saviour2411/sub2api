-- 用户定制查询与一次性授信独立存储，不改变现有余额扣费路径。
CREATE TABLE user_customizations (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    link_hash CHAR(64) UNIQUE,
    link_encrypted TEXT,
    link_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    link_version BIGINT NOT NULL DEFAULT 0 CHECK (link_version >= 0),
    auto_credit_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    credit_threshold NUMERIC(20,8) NOT NULL DEFAULT 1000 CHECK (credit_threshold >= 0),
    credit_amount NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (credit_amount >= 0),
    credit_generation BIGINT NOT NULL DEFAULT 1 CHECK (credit_generation > 0),
    credit_used_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (NOT link_enabled OR (link_hash IS NOT NULL AND link_encrypted IS NOT NULL)),
    CHECK (NOT auto_credit_enabled OR credit_amount > 0)
);
CREATE INDEX idx_user_customizations_credit_ready ON user_customizations(user_id)
    WHERE auto_credit_enabled AND credit_used_at IS NULL;

CREATE TABLE temporary_credit_grants (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    generation BIGINT NOT NULL,
    threshold NUMERIC(20,8) NOT NULL,
    amount NUMERIC(20,8) NOT NULL CHECK (amount > 0),
    balance_before NUMERIC(20,8) NOT NULL,
    balance_after NUMERIC(20,8) NOT NULL,
    redeem_code_id BIGINT REFERENCES redeem_codes(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cache_pass SMALLINT NOT NULL DEFAULT 0 CHECK (cache_pass IN (0, 1)),
    cache_next_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cache_cleared_at TIMESTAMPTZ,
    UNIQUE(user_id, generation)
);
CREATE INDEX idx_temporary_credit_grants_cache_pending ON temporary_credit_grants(cache_next_at, id)
    WHERE cache_cleared_at IS NULL;
COMMENT ON TABLE user_customizations IS '用户免登录查询链接和按代次消费的一次性授信配置';
COMMENT ON TABLE temporary_credit_grants IS '临时授信审计流水，不属于真实充值，不自动扣回';
