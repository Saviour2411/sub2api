-- 将网络安全策略阻断记录为 request_type=4，使其在用量审计中保持可见，且不与旧版 request_type=0 混淆。
ALTER TABLE usage_logs
    DROP CONSTRAINT IF EXISTS usage_logs_request_type_check;

ALTER TABLE usage_logs
    ADD CONSTRAINT usage_logs_request_type_check
    CHECK (request_type IN (0, 1, 2, 3, 4)) NOT VALID;
