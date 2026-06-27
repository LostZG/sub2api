// Package schema 定义 Ent ORM 的数据库 schema。
// 每个文件对应一个数据库实体（表），定义其字段、边（关联）和索引。
package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ChannelProvider 定义上游渠道商实体的 schema。
//
// 渠道商是按 baseUrl 维度聚合的衍生数据：一个 baseUrl 可能对应多条账号，
// 但充值金额、余额等是渠道商维度的。该表仅保存可手动编辑的充值金额与
// 定期刷新得到的余额快照，账号数据仍以 accounts 表为准。
//
// 主要功能：
//   - 按 base_url 唯一标识一个上游渠道商
//   - 保存手动录入的充值金额
//   - 保存定期刷新得到的余额、单位、检查时间与有效性
type ChannelProvider struct {
	ent.Schema
}

// Annotations 返回 schema 的注解配置。
// 这里指定数据库表名为 "channel_providers"。
func (ChannelProvider) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "channel_providers"},
	}
}

// Mixin 返回该 schema 使用的混入组件。
// - TimeMixin: 自动管理 created_at 和 updated_at 时间戳
// 注意：这是衍生数据，不需要软删除。
func (ChannelProvider) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

// Fields 定义渠道商实体的所有字段。
func (ChannelProvider) Fields() []ent.Field {
	return []ent.Field{
		// base_url: 上游渠道商的 baseUrl，标准化后唯一（小写 + 去尾斜杠）
		field.String("base_url").
			MaxLen(500).
			NotEmpty(),

		// display_name: 可选的展示名称（可为空）
		field.String("display_name").
			Optional().
			Nillable().
			MaxLen(200),

		// recharge_amount: 手动录入的充值金额
		field.Float("recharge_amount").
			SchemaType(map[string]string{dialect.Postgres: "decimal(10,2)"}).
			Default(0),

		// quota_per_unit: NewAPI 类上游 quota→USD 换算系数（1 USD = N 个 quota 点）。
		// 默认 500000（NewAPI 标准），不同部署可能不同（如 codexapis 用 5000000）。
		// 仅对 /api/user/self 查询生效；sub2api 类直接返回 USD，不读此字段。
		field.Int64("quota_per_unit").
			Default(500000),

		// balance: 最近一次刷新得到的余额（可为空，表示从未刷新过）。
		// 精度 20,4：NewAPI 类 quota 值可能很大，换算后余额需大整数部分。
		field.Float("balance").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,4)"}),

		// balance_unit: 余额单位，默认 USD
		field.String("balance_unit").
			MaxLen(20).
			Default("USD"),

		// balance_checked_at: 最近一次余额检查时间
		field.Time("balance_checked_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),

		// is_valid: 最近一次刷新是否成功
		field.Bool("is_valid").
			Default(true),

		// sync_balance: 是否参与"刷新全部"的余额同步。关闭后刷新全部时跳过该渠道商；
		// 单行刷新不受影响（用户主动点单行刷新仍会执行）。
		field.Bool("sync_balance").
			Default(true),

		// last_refresh_error: 最近一次刷新失败的原因（可为空）
		field.String("last_refresh_error").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
	}
}

// Edges 定义渠道商实体的关联关系。
// 渠道商不与账号建立外键关联（账号的 base_url 存在 JSONB 中，
// 关联通过运行时聚合查询实现，避免数据冗余与一致性维护成本）。
func (ChannelProvider) Edges() []ent.Edge {
	return nil
}

// Indexes 定义数据库索引，优化查询性能。
func (ChannelProvider) Indexes() []ent.Index {
	return []ent.Index{
		// base_url 唯一索引：保证渠道商按 baseUrl 去重，Upsert 依赖该约束
		index.Fields("base_url").Unique(),
	}
}
