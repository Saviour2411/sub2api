-- 为兑换码增加可使用次数，并记录每个用户的使用明细。

ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS max_uses INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS used_count INT NOT NULL DEFAULT 0;

UPDATE redeem_codes
SET used_count = 1
WHERE status = 'used' AND used_count = 0;

CREATE TABLE IF NOT EXISTS redeem_code_usages (
    id BIGSERIAL PRIMARY KEY,
    redeem_code_id BIGINT NOT NULL REFERENCES redeem_codes(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    value DECIMAL(20,8) NOT NULL,
    group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    validity_days INT NOT NULL DEFAULT 30,
    used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(redeem_code_id, user_id)
);

ALTER TABLE redeem_code_usages
    ALTER COLUMN type TYPE VARCHAR(50);

INSERT INTO redeem_code_usages (
    redeem_code_id,
    user_id,
    type,
    value,
    group_id,
    validity_days,
    used_at
)
SELECT
    id,
    used_by,
    type,
    value,
    group_id,
    validity_days,
    COALESCE(used_at, created_at)
FROM redeem_codes
WHERE used_by IS NOT NULL
ON CONFLICT (redeem_code_id, user_id) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_redeem_codes_used_count ON redeem_codes(used_count);
CREATE INDEX IF NOT EXISTS idx_redeem_code_usages_redeem_code_id ON redeem_code_usages(redeem_code_id);
CREATE INDEX IF NOT EXISTS idx_redeem_code_usages_user_id ON redeem_code_usages(user_id);
CREATE INDEX IF NOT EXISTS idx_redeem_code_usages_used_at ON redeem_code_usages(used_at);

COMMENT ON COLUMN redeem_codes.max_uses IS '最大可使用次数，必须为正整数';
COMMENT ON COLUMN redeem_codes.used_count IS '已使用次数';
COMMENT ON TABLE redeem_code_usages IS '兑换码使用记录';
