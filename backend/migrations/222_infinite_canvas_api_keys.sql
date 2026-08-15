-- 为无限画布创建可识别、可并发复用的托管 API Key。
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS purpose VARCHAR(32) NOT NULL DEFAULT 'general';

ALTER TABLE api_keys
    DROP CONSTRAINT IF EXISTS api_keys_purpose_check;

ALTER TABLE api_keys
    ADD CONSTRAINT api_keys_purpose_check
    CHECK (purpose IN ('general', 'infinite_canvas'));

CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_infinite_canvas_user_group
    ON api_keys(user_id, group_id)
    WHERE deleted_at IS NULL AND purpose = 'infinite_canvas';

COMMENT ON COLUMN api_keys.purpose IS 'API key purpose: general or infinite_canvas';
