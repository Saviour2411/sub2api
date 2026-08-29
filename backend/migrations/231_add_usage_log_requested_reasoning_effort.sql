-- 在分组策略改写和模型族映射（例如 max -> xhigh）前保存客户端请求的推理强度。
-- NULL 表示双写启用前的历史记录，或请求未声明推理强度。
--
-- 字段可空且无默认值：PostgreSQL 11+ 只修改元数据，不会重写可能很大且已分区的
-- usage_logs 表。
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS requested_reasoning_effort VARCHAR(20);
