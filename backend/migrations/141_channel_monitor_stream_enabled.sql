-- 141: 为渠道监控和请求模板增加显式流式请求开关。
-- 默认 false，保持现有非流式监控行为不变。

ALTER TABLE channel_monitors
    ADD COLUMN IF NOT EXISTS stream_enabled BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE channel_monitor_request_templates
    ADD COLUMN IF NOT EXISTS stream_enabled BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN channel_monitors.stream_enabled IS
    '此监控是否发送流式请求并解析 SSE 响应。';

COMMENT ON COLUMN channel_monitor_request_templates.stream_enabled IS
    '应用此模板的监控是否发送流式请求并解析 SSE 响应。';
