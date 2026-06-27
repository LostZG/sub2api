-- 155_channel_providers_quota_per_unit.sql
-- 渠道号商新增 quota_per_unit 字段：NewAPI 类上游 quota→USD 换算系数（1 USD = N quota 点）。
-- 默认 500000（NewAPI 标准）；不同部署可能不同（如 codexapis 用 5000000）。
-- 仅对 /api/user/self 余额查询生效；sub2api 类直接返回 USD，不读此字段。幂等。
ALTER TABLE channel_providers ADD COLUMN IF NOT EXISTS quota_per_unit BIGINT NOT NULL DEFAULT 500000;
