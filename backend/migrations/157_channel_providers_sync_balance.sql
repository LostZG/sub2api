-- 157_channel_providers_sync_balance.sql
-- 渠道号商新增 sync_balance 字段：是否参与"刷新全部"的余额同步。
-- 关闭后刷新全部时跳过该渠道商；单行刷新不受影响。默认 TRUE。幂等。
ALTER TABLE channel_providers ADD COLUMN IF NOT EXISTS sync_balance BOOLEAN NOT NULL DEFAULT TRUE;
