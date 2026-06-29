-- 156_add_group_claude_code_upstream_mimicry.sql
-- Add group-level switch for simulating Claude Code when forwarding to Anthropic upstream.

ALTER TABLE groups
ADD COLUMN IF NOT EXISTS claude_code_upstream_mimicry BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_groups_claude_code_upstream_mimicry
ON groups(claude_code_upstream_mimicry)
WHERE deleted_at IS NULL;

COMMENT ON COLUMN groups.claude_code_upstream_mimicry IS '是否将非 Claude Code 下游请求模拟为 Claude Code 客户端转发到 Anthropic 上游';
