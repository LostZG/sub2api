-- 154_channel_providers.sql
-- 渠道号商（Channel Providers）：按 base_url 维度聚合上游渠道商的衍生数据表。
-- 仅保存可手动编辑的充值金额与定期刷新得到的余额快照；账号数据仍以 accounts 表为准，
-- 通过运行时聚合查询（credentials->>'base_url'）关联，不建立外键。
-- 幂等：全部使用 IF NOT EXISTS。
CREATE TABLE IF NOT EXISTS channel_providers (
    id BIGSERIAL PRIMARY KEY,
    base_url VARCHAR(500) NOT NULL,
    display_name VARCHAR(200),
    recharge_amount DECIMAL(10,2) NOT NULL DEFAULT 0,
    balance DECIMAL(10,4),
    balance_unit VARCHAR(20) NOT NULL DEFAULT 'USD',
    balance_checked_at TIMESTAMPTZ,
    is_valid BOOLEAN NOT NULL DEFAULT TRUE,
    last_refresh_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS channel_providers_base_url_key ON channel_providers (base_url);
