-- 针对公开（非专属）分组的用户级访问控制。
--
-- 公开分组原本允许所有用户绑定。为用户开启此开关后，可绑定范围收窄为
-- user_allowed_groups 中列出的公开分组；该表此前只承载专属分组。
-- 默认值保持所有既有用户不受限制。
ALTER TABLE users ADD COLUMN IF NOT EXISTS restrict_public_groups BOOLEAN NOT NULL DEFAULT false;
