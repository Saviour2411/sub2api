-- 删除已失效的分组级 Claude Code 上游模拟配置。
-- 当前运行时仅保留二开网关配置中的全局开关，不再读取此列。
DROP INDEX IF EXISTS idx_groups_claude_code_upstream_mimicry;

ALTER TABLE groups
DROP COLUMN IF EXISTS claude_code_upstream_mimicry;
