-- 在 251 的过期下界与重建脏标记之上保留 226 的当日 INSERT 非阻塞行为。
-- 仅替换 INSERT 函数；不修改任何已发布迁移，也不放宽历史写入的失效锁。
CREATE OR REPLACE FUNCTION invalidate_group_usage_rollup_state_after_insert()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    configured_timezone TEXT := current_setting('TimeZone');
    affected_date DATE;
    state_closed_before DATE;
    state_building_until DATE;
    state_build_dirty_from DATE;
    state_expired_before TIMESTAMPTZ;
BEGIN
    SELECT MIN((created_at AT TIME ZONE configured_timezone)::date)
    INTO affected_date
    FROM inserted_usage_logs
    WHERE group_id IS NOT NULL;

    IF affected_date IS NULL THEN
        RETURN NULL;
    END IF;

    BEGIN
        SELECT closed_before, building_until, build_dirty_from, expired_before
        INTO state_closed_before, state_building_until, state_build_dirty_from, state_expired_before
        FROM usage_group_rollup_state
        WHERE id = 1
        FOR KEY SHARE NOWAIT;
    EXCEPTION
        WHEN lock_not_available THEN
            -- 使用墙上时钟识别跨午夜事务；当日与未来写入不参与已关闭日桶。
            IF affected_date >=
                (clock_timestamp() AT TIME ZONE configured_timezone)::date THEN
                RETURN NULL;
            END IF;

            -- 历史写入等待登记/发布结束，再按最新过期下界和重建状态失效。
            SELECT closed_before, building_until, build_dirty_from, expired_before
            INTO state_closed_before, state_building_until, state_build_dirty_from, state_expired_before
            FROM usage_group_rollup_state
            WHERE id = 1
            FOR KEY SHARE;
    END;

    IF state_closed_before IS NULL THEN
        RETURN NULL;
    END IF;

    SELECT MIN((created_at AT TIME ZONE configured_timezone)::date)
    INTO affected_date
    FROM inserted_usage_logs
    WHERE group_id IS NOT NULL
        AND created_at >= state_expired_before;

    IF state_closed_before > affected_date
        OR (
            state_building_until > affected_date
            AND (state_build_dirty_from IS NULL OR state_build_dirty_from > affected_date)
        ) THEN
        PERFORM apply_group_usage_rollup_invalidation(affected_date);
    END IF;

    RETURN NULL;
END;
$$;
