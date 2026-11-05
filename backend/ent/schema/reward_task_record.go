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

type RewardTaskRecord struct{ ent.Schema }

func (RewardTaskRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "reward_task_records"}}
}
func (RewardTaskRecord) Mixin() []ent.Mixin { return []ent.Mixin{mixins.TimeMixin{}} }
func (RewardTaskRecord) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("task_id"),
		field.Int64("user_id"),
		field.String("period_key").MaxRuneLen(20).SchemaType(map[string]string{dialect.Postgres: "varchar(20)"}),
		field.String("ticket").MaxRuneLen(64).SchemaType(map[string]string{dialect.Postgres: "varchar(64)"}),
		field.String("status").MaxRuneLen(20).SchemaType(map[string]string{dialect.Postgres: "varchar(20)"}),
		field.Int64("progress_ms").Default(0),
		field.Float("recharge_total").SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}).Default(0),
		field.String("external_user_id").MaxRuneLen(128).SchemaType(map[string]string{dialect.Postgres: "varchar(128)"}).Default(""),
		field.Time("started_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("last_heartbeat_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("ticket_expires_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("completed_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Float("reward_amount").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).Default(0),
		field.String("task_title").MaxRuneLen(100).SchemaType(map[string]string{dialect.Postgres: "varchar(100)"}).Default(""),
	}
}
func (RewardTaskRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "task_id", "period_key").Unique(),
		index.Fields("ticket").Unique(),
		index.Fields("task_id", "period_key", "external_user_id").Unique().Annotations(entsql.IndexWhere("external_user_id <> ''")),
		index.Fields("task_id", "status"),
		index.Fields("user_id", "completed_at").Annotations(entsql.DescColumns("completed_at")),
		index.Fields("completed_at"),
	}
}
