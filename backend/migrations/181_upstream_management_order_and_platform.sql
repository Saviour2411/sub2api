-- 二开功能：支持上游站点持久化排序，并修复 New API 历史分组的平台类型。
ALTER TABLE upstream_sites
    ADD COLUMN IF NOT EXISTS sort_order INTEGER NOT NULL DEFAULT 0;

-- 既有站点按创建主键初始化，避免升级后所有站点拥有相同顺序。
UPDATE upstream_sites
SET sort_order = id
WHERE sort_order = 0;

CREATE INDEX IF NOT EXISTS idx_upstream_sites_sort_order
    ON upstream_sites (sort_order, id);

-- New API 可能只返回分组名、描述和倍率，旧同步逻辑因此把站点类型误存为分组平台。
-- 先识别 Antigravity，避免名称中的 Google 被归入普通 Gemini。
UPDATE upstream_groups
SET platform = 'Antigravity', updated_at = NOW()
WHERE LOWER(platform) IN ('', 'newapi', 'new api')
  AND LOWER(name || ' ' || description) LIKE '%antigravity%';

UPDATE upstream_groups
SET platform = 'Anthropic', updated_at = NOW()
WHERE LOWER(platform) IN ('', 'newapi', 'new api')
  AND (
    LOWER(name || ' ' || description) LIKE '%claude%'
    OR LOWER(name || ' ' || description) LIKE '%anthropic%'
    OR LOWER(name || ' ' || description) LIKE '%kiro%'
    OR LOWER(name || ' ' || description) LIKE '%sonnet%'
    OR LOWER(name || ' ' || description) LIKE '%opus%'
    OR LOWER(name || ' ' || description) LIKE '%haiku%'
  );

UPDATE upstream_groups
SET platform = 'OpenAI', updated_at = NOW()
WHERE LOWER(platform) IN ('', 'newapi', 'new api')
  AND (
    LOWER(name || ' ' || description) LIKE '%gpt%'
    OR LOWER(name || ' ' || description) LIKE '%openai%'
  );

UPDATE upstream_groups
SET platform = 'Gemini', updated_at = NOW()
WHERE LOWER(platform) IN ('', 'newapi', 'new api')
  AND (
    LOWER(name || ' ' || description) LIKE '%gemini%'
    OR LOWER(name || ' ' || description) LIKE '%google ai%'
  );

UPDATE upstream_groups
SET platform = 'Grok', updated_at = NOW()
WHERE LOWER(platform) IN ('', 'newapi', 'new api')
  AND (
    LOWER(name || ' ' || description) LIKE '%grok%'
    OR LOWER(name || ' ' || description) LIKE '%x.ai%'
  );

-- 倍率历史沿用同一平台归类，避免详情弹窗继续显示旧的 New API 标签。
UPDATE upstream_group_multiplier_history
SET platform = 'Antigravity'
WHERE LOWER(platform) IN ('', 'newapi', 'new api')
  AND LOWER(name || ' ' || description) LIKE '%antigravity%';

UPDATE upstream_group_multiplier_history
SET platform = 'Anthropic'
WHERE LOWER(platform) IN ('', 'newapi', 'new api')
  AND (
    LOWER(name || ' ' || description) LIKE '%claude%'
    OR LOWER(name || ' ' || description) LIKE '%anthropic%'
    OR LOWER(name || ' ' || description) LIKE '%kiro%'
    OR LOWER(name || ' ' || description) LIKE '%sonnet%'
    OR LOWER(name || ' ' || description) LIKE '%opus%'
    OR LOWER(name || ' ' || description) LIKE '%haiku%'
  );

UPDATE upstream_group_multiplier_history
SET platform = 'OpenAI'
WHERE LOWER(platform) IN ('', 'newapi', 'new api')
  AND (
    LOWER(name || ' ' || description) LIKE '%gpt%'
    OR LOWER(name || ' ' || description) LIKE '%openai%'
  );

UPDATE upstream_group_multiplier_history
SET platform = 'Gemini'
WHERE LOWER(platform) IN ('', 'newapi', 'new api')
  AND (
    LOWER(name || ' ' || description) LIKE '%gemini%'
    OR LOWER(name || ' ' || description) LIKE '%google ai%'
  );

UPDATE upstream_group_multiplier_history
SET platform = 'Grok'
WHERE LOWER(platform) IN ('', 'newapi', 'new api')
  AND (
    LOWER(name || ' ' || description) LIKE '%grok%'
    OR LOWER(name || ' ' || description) LIKE '%x.ai%'
  );
