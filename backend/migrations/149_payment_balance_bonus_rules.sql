ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS base_amount DECIMAL(20,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS bonus_amount DECIMAL(20,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS bonus_rate DECIMAL(10,4) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS bonus_rule_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE payment_orders
SET base_amount = CASE
        WHEN order_type = 'balance' AND fee_rate > 0 THEN ROUND((pay_amount / (1 + fee_rate / 100))::numeric, 2)
        WHEN order_type = 'balance' THEN pay_amount
        ELSE amount
    END,
    bonus_amount = CASE
        WHEN order_type = 'balance' THEN ROUND((amount - CASE
            WHEN fee_rate > 0 THEN ROUND((pay_amount / (1 + fee_rate / 100))::numeric, 2)
            ELSE pay_amount
        END)::numeric, 2)
        ELSE 0
    END,
    bonus_rate = CASE
        WHEN order_type = 'balance' AND (
            CASE
                WHEN fee_rate > 0 THEN ROUND((pay_amount / (1 + fee_rate / 100))::numeric, 2)
                ELSE pay_amount
            END
        ) > 0 THEN ROUND(((amount / (
            CASE
                WHEN fee_rate > 0 THEN ROUND((pay_amount / (1 + fee_rate / 100))::numeric, 2)
                ELSE pay_amount
            END
        ) - 1) * 100)::numeric, 4)
        ELSE 0
    END
WHERE base_amount = 0;

INSERT INTO settings (key, value)
VALUES ('BALANCE_RECHARGE_BONUS_RULES', '')
ON CONFLICT (key) DO NOTHING;
