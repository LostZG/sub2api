-- 158_provider_group_ratio.sql
-- 账号新增 upstream_group：上游 NewAPI 部署里该 sk- key 绑定的分组名，
-- 结合 channel_providers.group_ratio 缓存查"最新倍率"。用户手动填一次。幂等。
--
-- 渠道号商新增 group_ratio（/api/pricing 返回的分组→倍率映射）与
-- group_ratio_checked_at（最近刷新时间）。幂等。

ALTER TABLE IF NOT EXISTS accounts
  ADD COLUMN IF NOT EXISTS upstream_group VARCHAR(100);

ALTER TABLE IF NOT EXISTS channel_providers
  ADD COLUMN IF NOT EXISTS group_ratio JSONB,
  ADD COLUMN IF NOT EXISTS group_ratio_checked_at TIMESTAMPTZ;
