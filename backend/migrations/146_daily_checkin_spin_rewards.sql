-- 每日签到转盘奖项与衰减配置。

ALTER TABLE daily_checkins
    ADD COLUMN IF NOT EXISTS prize_id VARCHAR(64) NOT NULL DEFAULT 'legacy_balance';

ALTER TABLE daily_checkins
    ADD COLUMN IF NOT EXISTS prize_name VARCHAR(120) NOT NULL DEFAULT '余额奖励';

ALTER TABLE daily_checkins
    ADD COLUMN IF NOT EXISTS reward_type VARCHAR(20) NOT NULL DEFAULT 'balance';

ALTER TABLE daily_checkins
    ADD COLUMN IF NOT EXISTS probability_bps INT NOT NULL DEFAULT 10000;

ALTER TABLE daily_checkins
    ADD COLUMN IF NOT EXISTS effective_probability_bps INT NOT NULL DEFAULT 10000;

ALTER TABLE daily_checkins
    ADD COLUMN IF NOT EXISTS decay_factor_bps INT NOT NULL DEFAULT 10000;

ALTER TABLE daily_checkins
    ADD COLUMN IF NOT EXISTS concurrency_before INT;

ALTER TABLE daily_checkins
    ADD COLUMN IF NOT EXISTS concurrency_after INT;

ALTER TABLE daily_checkins
    ADD COLUMN IF NOT EXISTS subscription_group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL;

ALTER TABLE daily_checkins
    ADD COLUMN IF NOT EXISTS subscription_validity_days INT;

ALTER TABLE daily_checkins
    ADD COLUMN IF NOT EXISTS subscription_expires_at TIMESTAMPTZ;

ALTER TABLE daily_checkins
    ADD COLUMN IF NOT EXISTS reward_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_daily_checkins_user_type_created_at
    ON daily_checkins (user_id, reward_type, created_at DESC);

INSERT INTO settings (key, value)
VALUES
    ('daily_checkin_prizes', '[{"id":"legacy_balance","name":"余额奖励","type":"balance","probability_bps":10000,"enabled":true,"sort_order":0,"balance_mode":"fixed","amount":1}]'),
    ('daily_checkin_unpaid_full_days', '7'),
    ('daily_checkin_unpaid_decay_rules', '[{"after_days":7,"factor_bps":5000},{"after_days":14,"factor_bps":2000},{"after_days":30,"factor_bps":0}]'),
    ('daily_checkin_linuxdo_exempt_enabled', 'false')
ON CONFLICT (key) DO NOTHING;
