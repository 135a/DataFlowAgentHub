-- 007_role_system_v2.sql
-- 数据存储与权限管理重构：4级角色体系 + 数据集管理 + 权限升级

-- 1. 将 users.role CHECK 约束从 3 角色扩展到 4 角色
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
  CHECK (role IN ('super_admin', 'data_admin', 'normal_user', 'read_only_visitor'));

-- 2. 创建数据集表（映射到 MySQL 数据库）
CREATE TABLE IF NOT EXISTS datasets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE,
    mysql_database TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'deleted')),
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 3. 创建数据集授权表
CREATE TABLE IF NOT EXISTS dataset_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dataset_id UUID NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    permission_level TEXT NOT NULL CHECK (permission_level IN ('read', 'write', 'admin')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (dataset_id, user_id)
);

-- 4. 创建数据表定义表
CREATE TABLE IF NOT EXISTS dataset_tables (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dataset_id UUID NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    display_name TEXT,
    mysql_table_name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (dataset_id, name),
    UNIQUE (dataset_id, mysql_table_name)
);

-- 5. 创建字段定义表
CREATE TABLE IF NOT EXISTS table_fields (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    table_id UUID NOT NULL REFERENCES dataset_tables(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    display_name TEXT,
    field_type TEXT NOT NULL,
    field_length INT,
    is_nullable BOOLEAN NOT NULL DEFAULT true,
    ordinal_position INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (table_id, name)
);

-- 6. 创建权限升级请求表
CREATE TABLE IF NOT EXISTS permission_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    requested_role TEXT NOT NULL CHECK (requested_role IN ('data_admin', 'read_only_visitor')),
    reason TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewed_by UUID REFERENCES users(id),
    review_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at TIMESTAMPTZ
);

-- 7. sessions 表增加 dataset_id 和 dataset_table_id 可选引用
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS dataset_id UUID REFERENCES datasets(id) ON DELETE SET NULL;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS dataset_table_id UUID REFERENCES dataset_tables(id) ON DELETE SET NULL;

-- 8. 创建索引
CREATE INDEX IF NOT EXISTS idx_dataset_permissions_user ON dataset_permissions(user_id);
CREATE INDEX IF NOT EXISTS idx_dataset_permissions_dataset ON dataset_permissions(dataset_id);
CREATE INDEX IF NOT EXISTS idx_dataset_tables_dataset ON dataset_tables(dataset_id);
CREATE INDEX IF NOT EXISTS idx_table_fields_table ON table_fields(table_id);
CREATE INDEX IF NOT EXISTS idx_permission_requests_user ON permission_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_permission_requests_status ON permission_requests(status);
