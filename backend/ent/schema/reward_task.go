package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
)

type RewardTask struct{ ent.Schema }

func (RewardTask) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "reward_tasks"}}
}
func (RewardTask) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}, mixins.SoftDeleteMixin{}}
}
func (RewardTask) Fields() []ent.Field {
	return []ent.Field{
		field.String("title").MaxRuneLen(100).SchemaType(map[string]string{dialect.Postgres: "varchar(100)"}),
		field.String("description").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""),
		field.String("task_type").MaxRuneLen(20).SchemaType(map[string]string{dialect.Postgres: "varchar(20)"}),
		field.String("verify_mode").MaxRuneLen(20).SchemaType(map[string]string{dialect.Postgres: "varchar(20)"}),
		field.Int64("partner_id").Optional().Nillable(),
		field.String("target_url").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("icon_url").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""),
		field.Float("reward_amount").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.String("period").MaxRuneLen(20).SchemaType(map[string]string{dialect.Postgres: "varchar(20)"}).Default("once"),
		field.Int("browse_seconds").Default(0),
		field.Int("idle_timeout_seconds").Default(15),
		field.Float("min_recharge_amount").SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}).Default(0),
		field.Int("max_completions").Default(0),
		field.Int("completed_count").Default(0),
		field.Time("starts_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("ends_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Bool("enabled").Default(true),
		field.Int("sort_order").Default(0),
	}
}
func (RewardTask) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled", "deleted_at", "sort_order"),
	}
}
