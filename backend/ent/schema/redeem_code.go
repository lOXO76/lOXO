package schema

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// RedeemCode holds the schema definition for the RedeemCode entity.
//
// 删除策略：硬删除
// RedeemCode 使用硬删除而非软删除，原因如下：
//   - 兑换码具有一次性使用特性，删除后无需保留历史记录
//   - 已使用的兑换码通过 status 和 used_at 字段追踪，无需依赖软删除
//   - 减少数据库存储压力和查询复杂度
//
// 如需审计已删除的兑换码，建议在删除前将关键信息写入审计日志表。
//
// 多次兑换码（type = multi_use）：
//   - 码本身（父码）不记录 used_by，而是用 max_users / max_uses_per_user 限制
//     可兑换的用户数与每个用户的可兑换次数，used_count / used_user_count 为已用计数
//   - benefit_type 决定每次兑换发放的权益（balance / concurrency / subscription），
//     面值、分组与有效天数沿用 value / group_id / validity_days
//   - 每次兑换都会落一条 type = benefit_type、status = used 的流水行，parent_id 指向父码；
//     兑换历史、余额流水与充值统计因此无需额外的数据来源
//   - 父码被删除时流水行保留（parent_id 置空）
type RedeemCode struct {
	ent.Schema
}

func (RedeemCode) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "redeem_codes"},
	}
}

func (RedeemCode) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").
			MaxLen(32).
			NotEmpty().
			Unique(),
		field.String("type").
			MaxLen(20).
			Default(domain.RedeemTypeBalance),
		field.Float("value").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.String("status").
			MaxLen(20).
			Default(domain.StatusUnused),
		field.Int64("used_by").
			Optional().
			Nillable(),
		field.Time("used_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("notes").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("expires_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int64("group_id").
			Optional().
			Nillable(),
		field.Int("validity_days").
			Default(30),
		field.String("benefit_type").
			MaxLen(20).
			Default("").
			Comment("多次兑换码：每次兑换发放的权益类型（balance/concurrency/subscription）"),
		field.Int("max_users").
			Default(1).
			Comment("多次兑换码：允许兑换的用户数"),
		field.Int("max_uses_per_user").
			Default(1).
			Comment("多次兑换码：每个用户可兑换的次数"),
		field.Int("used_count").
			Default(0).
			Comment("多次兑换码：已兑换总次数"),
		field.Int("used_user_count").
			Default(0).
			Comment("多次兑换码：已兑换的用户数"),
		field.Int64("parent_id").
			Optional().
			Nillable().
			Comment("多次兑换码的兑换流水：指向被兑换的父码"),
	}
}

func (RedeemCode) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("redeem_codes").
			Field("used_by").
			Unique(),
		edge.From("group", Group.Type).
			Ref("redeem_codes").
			Field("group_id").
			Unique(),
		// 多次兑换码（父码）与其兑换流水行的自引用关系。
		edge.To("redemptions", RedeemCode.Type).
			From("parent").
			Field("parent_id").
			Unique(),
	}
}

func (RedeemCode) Indexes() []ent.Index {
	return []ent.Index{
		// code 字段已在 Fields() 中声明 Unique()，无需重复索引
		index.Fields("status"),
		index.Fields("used_by"),
		index.Fields("group_id"),
		index.Fields("expires_at"),
		// 多次兑换码按「父码 + 用户」统计该用户已兑换次数。
		index.Fields("parent_id", "used_by").
			Annotations(entsql.IndexWhere("parent_id IS NOT NULL")),
	}
}
