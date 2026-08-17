-- 高频 usage INSERT 的非阻塞失效适配。
-- 222/223 已发布，不能修改其校验和；本迁移只替换 INSERT 触发器函数。
-- 当前开放日不参与已关闭日桶发布，回填锁竞争时可以立即放行；历史日期以及
-- 跨午夜事务仍必须等待水位锁，以保证历史汇总失效不会静默丢失。

CREATE OR REPLACE FUNCTION invalidate_group_usage_rollup_state_after_insert()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    affected_date DATE;
    published_before DATE;
    configured_timezone TEXT := current_setting('TimeZone');
    current_date_in_timezone DATE;
BEGIN
    SELECT MIN((created_at AT TIME ZONE configured_timezone)::date)
    INTO affected_date
    FROM inserted_usage_logs
    WHERE group_id IS NOT NULL;

    IF affected_date IS NULL THEN
        RETURN NULL;
    END IF;

    BEGIN
        SELECT closed_before
        INTO published_before
        FROM usage_group_rollup_state
        WHERE id = 1
        FOR KEY SHARE NOWAIT;
    EXCEPTION
        WHEN lock_not_available THEN
            -- CURRENT_TIMESTAMP 固定在事务开始时；这里必须使用 clock_timestamp，
            -- 才能识别已经跨过自然日边界但仍在执行的长事务。
            current_date_in_timezone := (clock_timestamp() AT TIME ZONE configured_timezone)::date;
            IF affected_date >= current_date_in_timezone THEN
                RETURN NULL;
            END IF;

            -- 历史日期或跨午夜写入不能绕过发布锁，等待后再按最新水位失效。
            SELECT closed_before
            INTO published_before
            FROM usage_group_rollup_state
            WHERE id = 1
            FOR KEY SHARE;
    END;

    IF published_before > affected_date THEN
        UPDATE usage_group_rollup_state
        SET closed_before = LEAST(closed_before, affected_date),
            updated_at = NOW()
        WHERE id = 1;
    END IF;

    RETURN NULL;
END;
$$;
