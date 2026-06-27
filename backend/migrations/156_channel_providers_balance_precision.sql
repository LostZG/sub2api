-- 156_channel_providers_balance_precision.sql
-- 扩大 balance 列精度：NewAPI 类上游 quota 值可能很大，按系数换算后的余额
-- 超过 DECIMAL(10,4) 的上限（整数部分仅 6 位，最大 999999.9999）会触发
-- "numeric field overflow"，导致刷新余额失败。
-- DECIMAL(20,4) 整数部分 16 位，足够容纳大额余额。幂等：相同类型重复 ALTER 不报错。
ALTER TABLE channel_providers ALTER COLUMN balance TYPE DECIMAL(20,4);
