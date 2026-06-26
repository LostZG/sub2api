package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// cpSQLExecutor 是 channel_provider_repo 依赖的最小 SQL 执行接口。
// 与 group_repo / account_repo 的 sqlExecutor 保持一致，便于注入 *sql.DB 或事务。
type cpSQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// channelProviderRepository 实现 service.ChannelProviderRepository。
// channel_providers 是衍生数据表，全部用原生 SQL 访问（聚合 / upsert 比 ent 更直观可控）。
type channelProviderRepository struct {
	sql cpSQLExecutor
}

// NewChannelProviderRepository 构造渠道商仓储。
// 仅依赖 *sql.DB：该表是衍生数据，不参与 ent 关联，无需 *ent.Client。
func NewChannelProviderRepository(sqlDB *sql.DB) service.ChannelProviderRepository {
	return &channelProviderRepository{sql: sqlDB}
}

// ListAll 返回所有渠道商，按 base_url 升序。
func (r *channelProviderRepository) ListAll(ctx context.Context) ([]*service.ChannelProvider, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, base_url, display_name, recharge_amount, balance, balance_unit,
		       balance_checked_at, is_valid, last_refresh_error, created_at, updated_at
		FROM channel_providers
		ORDER BY base_url ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("channel_provider list all: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*service.ChannelProvider
	for rows.Next() {
		p, err := scanChannelProvider(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetByBaseURL 按标准化 baseURL 查询单个渠道商。不存在返回 (nil, nil)。
func (r *channelProviderRepository) GetByBaseURL(ctx context.Context, baseURL string) (*service.ChannelProvider, error) {
	row := r.sql.QueryRowContext(ctx, `
		SELECT id, base_url, display_name, recharge_amount, balance, balance_unit,
		       balance_checked_at, is_valid, last_refresh_error, created_at, updated_at
		FROM channel_providers
		WHERE base_url = $1
	`, baseURL)

	p, err := scanChannelProviderRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("channel_provider get by base_url: %w", err)
	}
	return p, nil
}

// Upsert 按 base_url 唯一约束插入或更新充值金额。
// 存在时仅更新 recharge_amount（保留余额等字段），不存在时插入默认记录。
func (r *channelProviderRepository) Upsert(ctx context.Context, p *service.ChannelProvider) error {
	_, err := r.sql.ExecContext(ctx, `
		INSERT INTO channel_providers (base_url, recharge_amount, balance_unit, is_valid, created_at, updated_at)
		VALUES ($1, $2, 'USD', TRUE, NOW(), NOW())
		ON CONFLICT (base_url) DO UPDATE
			SET recharge_amount = EXCLUDED.recharge_amount,
			    updated_at = NOW()
	`, p.BaseURL, p.RechargeAmount)
	if err != nil {
		return fmt.Errorf("channel_provider upsert: %w", err)
	}
	return nil
}

// UpdateBalance 更新余额相关字段。base_url 不存在时自动插入（余额刷新可能先于充值金额编辑）。
func (r *channelProviderRepository) UpdateBalance(ctx context.Context, baseURL string, balance *float64, unit string, isValid bool, errMsg string, checkedAt time.Time) error {
	_, err := r.sql.ExecContext(ctx, `
		INSERT INTO channel_providers (base_url, recharge_amount, balance, balance_unit, balance_checked_at, is_valid, last_refresh_error, created_at, updated_at)
		VALUES ($1, 0, $2, $3, $4, $5, NULLIF($6, ''), NOW(), NOW())
		ON CONFLICT (base_url) DO UPDATE
			SET balance = EXCLUDED.balance,
			    balance_unit = EXCLUDED.balance_unit,
			    balance_checked_at = EXCLUDED.balance_checked_at,
			    is_valid = EXCLUDED.is_valid,
			    last_refresh_error = EXCLUDED.last_refresh_error,
			    updated_at = NOW()
	`,
		baseURL,
		balance, // *float64 → pg 驱动接受 nil
		defaultStr(unit, "USD"),
		checkedAt,
		isValid,
		errMsg,
	)
	if err != nil {
		return fmt.Errorf("channel_provider update balance: %w", err)
	}
	return nil
}

// ListAggregated 聚合 accounts 表按 base_url 去重，LEFT JOIN channel_providers，
// 返回每行 base_url + account_count + 充值/余额信息。
//
// 标准化规则：LOWER(TRIM(TRAILING '/' FROM TRIM(credentials->>'base_url')))
// 必须与 service.NormalizeBaseURL 保持一致，否则去重 / 关联会错乱。
func (r *channelProviderRepository) ListAggregated(ctx context.Context) ([]service.ChannelProviderAggregated, error) {
	rows, err := r.sql.QueryContext(ctx, `
		WITH normalized AS (
			SELECT LOWER(TRIM(TRAILING '/' FROM TRIM(a.credentials->>'base_url'))) AS base_url,
			       COUNT(*)::BIGINT AS account_count
			FROM accounts a
			WHERE a.deleted_at IS NULL
			  AND a.type = 'api_key'
			  AND btrim(a.credentials->>'base_url') <> ''
			GROUP BY LOWER(TRIM(TRAILING '/' FROM TRIM(a.credentials->>'base_url')))
		)
		SELECT n.base_url,
		       n.account_count,
		       cp.id, cp.display_name, cp.recharge_amount, cp.balance, cp.balance_unit,
		       cp.balance_checked_at, cp.is_valid, cp.last_refresh_error,
		       COALESCE(cp.created_at, '1970-01-01T00:00:00Z'::timestamptz) AS created_at,
		       COALESCE(cp.updated_at, '1970-01-01T00:00:00Z'::timestamptz) AS updated_at
		FROM normalized n
		LEFT JOIN channel_providers cp ON cp.base_url = n.base_url
		ORDER BY n.base_url ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("channel_provider list aggregated: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []service.ChannelProviderAggregated
	for rows.Next() {
		var (
			baseURL       string
			accountCount  int64
			id            sql.NullInt64
			displayName   sql.NullString
			rechargeAmt   sql.NullFloat64
			balance       sql.NullFloat64
			balanceUnit   sql.NullString
			balanceChk    sql.NullTime
			isValid       sql.NullBool
			lastErr       sql.NullString
			createdAt     time.Time
			updatedAt     time.Time
		)
		if err := rows.Scan(
			&baseURL, &accountCount,
			&id, &displayName, &rechargeAmt, &balance, &balanceUnit,
			&balanceChk, &isValid, &lastErr,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("channel_provider scan aggregated: %w", err)
		}

		agg := service.ChannelProviderAggregated{
			AccountCount: accountCount,
			ChannelProvider: service.ChannelProvider{
				BaseURL:       baseURL,
				BalanceUnit:   "USD",
				RechargeAmount: 0,
				IsValid:       true,
				CreatedAt:     createdAt,
				UpdatedAt:     updatedAt,
			},
		}
		if id.Valid {
			agg.ID = id.Int64
		}
		if displayName.Valid {
			s := displayName.String
			agg.DisplayName = &s
		}
		if rechargeAmt.Valid {
			agg.RechargeAmount = rechargeAmt.Float64
		}
		if balance.Valid {
			v := balance.Float64
			agg.Balance = &v
		}
		if balanceUnit.Valid && balanceUnit.String != "" {
			agg.BalanceUnit = balanceUnit.String
		}
		if balanceChk.Valid {
			t := balanceChk.Time
			agg.BalanceCheckedAt = &t
		}
		if isValid.Valid {
			agg.IsValid = isValid.Bool
		}
		if lastErr.Valid {
			s := lastErr.String
			agg.LastRefreshError = &s
		}
		out = append(out, agg)
	}
	return out, rows.Err()
}

// FindFirstActiveAPIKeyAccountByBaseURL 取该标准化 baseUrl 下第一个
// status='active' AND type='api_key' 的账号的上游调用凭据（apiKey/baseURL/proxy）。
// 未找到返回 (nil, nil)。
func (r *channelProviderRepository) FindFirstActiveAPIKeyAccountByBaseURL(ctx context.Context, normalizedBaseURL string) (*service.ProviderRefreshSource, error) {
	rows, err := r.sql.QueryContext(ctx, `
		SELECT a.credentials->>'api_key' AS api_key,
		       a.credentials->>'base_url' AS base_url,
		       p.protocol, p.host, p.port, p.username, p.password
		FROM accounts a
		LEFT JOIN proxies p ON p.id = a.proxy_id AND p.deleted_at IS NULL
		WHERE a.deleted_at IS NULL
		  AND a.type = 'api_key'
		  AND a.status = 'active'
		  AND LOWER(TRIM(TRAILING '/' FROM TRIM(a.credentials->>'base_url'))) = $1
		  AND btrim(a.credentials->>'api_key') <> ''
		ORDER BY a.priority ASC, a.id ASC
		LIMIT 1
	`, normalizedBaseURL)
	if err != nil {
		return nil, fmt.Errorf("channel_provider find account: %w", err)
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		return nil, rows.Err()
	}

	var (
		apiKey   sql.NullString
		baseURL  sql.NullString
		protocol sql.NullString
		host     sql.NullString
		port     sql.NullInt64
		username sql.NullString
		password sql.NullString
	)
	if err := rows.Scan(&apiKey, &baseURL, &protocol, &host, &port, &username, &password); err != nil {
		return nil, fmt.Errorf("channel_provider scan account: %w", err)
	}

	source := &service.ProviderRefreshSource{
		APIKey:  apiKey.String,
		BaseURL: baseURL.String,
	}
	if protocol.Valid {
		source.Proxy = &service.Proxy{
			Protocol: protocol.String,
			Host:     host.String,
			Port:     int(port.Int64),
			Username: username.String,
			Password: password.String,
		}
	}
	return source, rows.Err()
}

// scanChannelProvider 从 *sql.Rows 扫描一行到 ChannelProvider。
func scanChannelProvider(rows *sql.Rows) (*service.ChannelProvider, error) {
	var (
		p              service.ChannelProvider
		displayName    sql.NullString
		balance        sql.NullFloat64
		balanceUnit    sql.NullString
		balanceChecked sql.NullTime
		lastErr        sql.NullString
	)
	if err := rows.Scan(
		&p.ID, &p.BaseURL, &displayName, &p.RechargeAmount, &balance, &balanceUnit,
		&balanceChecked, &p.IsValid, &lastErr, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("channel_provider scan: %w", err)
	}
	p.BalanceUnit = "USD"
	if balanceUnit.Valid && balanceUnit.String != "" {
		p.BalanceUnit = balanceUnit.String
	}
	if displayName.Valid {
		s := displayName.String
		p.DisplayName = &s
	}
	if balance.Valid {
		v := balance.Float64
		p.Balance = &v
	}
	if balanceChecked.Valid {
		t := balanceChecked.Time
		p.BalanceCheckedAt = &t
	}
	if lastErr.Valid {
		s := lastErr.String
		p.LastRefreshError = &s
	}
	return &p, nil
}

// scanChannelProviderRow 从 *sql.Row（单行）扫描到 ChannelProvider。
func scanChannelProviderRow(row *sql.Row) (*service.ChannelProvider, error) {
	var (
		p              service.ChannelProvider
		displayName    sql.NullString
		balance        sql.NullFloat64
		balanceUnit    sql.NullString
		balanceChecked sql.NullTime
		lastErr        sql.NullString
	)
	if err := row.Scan(
		&p.ID, &p.BaseURL, &displayName, &p.RechargeAmount, &balance, &balanceUnit,
		&balanceChecked, &p.IsValid, &lastErr, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	p.BalanceUnit = "USD"
	if balanceUnit.Valid && balanceUnit.String != "" {
		p.BalanceUnit = balanceUnit.String
	}
	if displayName.Valid {
		s := displayName.String
		p.DisplayName = &s
	}
	if balance.Valid {
		v := balance.Float64
		p.Balance = &v
	}
	if balanceChecked.Valid {
		t := balanceChecked.Time
		p.BalanceCheckedAt = &t
	}
	if lastErr.Valid {
		s := lastErr.String
		p.LastRefreshError = &s
	}
	return &p, nil
}

// defaultStr 在 v 为空时返回 fallback。
func defaultStr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
