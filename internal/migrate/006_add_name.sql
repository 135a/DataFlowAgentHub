-- migrate: up
-- 为 users 表新增 name 字段，支持姓名展示和注册
ALTER TABLE users ADD COLUMN IF NOT EXISTS name TEXT;

-- migrate: down
ALTER TABLE users DROP COLUMN IF EXISTS name;
