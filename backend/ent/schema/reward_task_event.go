package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type RewardTaskEvent struct{ ent.Schema }

func (RewardTaskEvent) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "reward_task_events"}}
}
func (RewardTaskEvent) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("partner_id"),
		field.String("event_id").MaxRuneLen(128).SchemaType(map[string]string{dialect.Postgres: "varchar(128)"}),
		field.String("event_type").MaxRuneLen(20).SchemaType(map[string]string{dialect.Postgres: "varchar(20)"}).Default(""),
		field.String("ticket").MaxRuneLen(64).SchemaType(map[string]string{dialect.Postgres: "varchar(64)"}).Default(""),
		field.Int64("task_id").Optional().Nillable(),
		field.Int64("record_id").Optional().Nillable(),
		field.Int64("user_id").Optional().Nillable(),
		field.String("external_user_id").MaxRuneLen(128).SchemaType(map[string]string{dialect.Postgres: "varchar(128)"}).Default(""),
		field.Float("amount").SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}).Default(0),
		field.String("order_id").MaxRuneLen(128).SchemaType(map[string]string{dialect.Postgres: "varchar(128)"}).Default(""),
		field.String("result").MaxRuneLen(20).SchemaType(map[string]string{dialect.Postgres: "varchar(20)"}),
		field.String("error_code").MaxRuneLen(64).SchemaType(map[string]string{dialect.Postgres: "varchar(64)"}).Default(""),
		field.String("remote_ip").MaxRuneLen(64).SchemaType(map[string]string{dialect.Postgres: "varchar(64)"}).Default(""),
		field.JSON("payload", map[string]any{}).Default(map[string]any{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("created_at").SchemaType(map[string]string{dialect.Postgres: "timestamptz"}).Default(time.Now).Immutable(),
	}
}
func (RewardTaskEvent) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("partner_id", "event_id").Unique().Annotations(entsql.IndexWhere("result <> 'rejected'")),
		index.Fields("partner_id", "created_at").Annotations(entsql.DescColumns("created_at")),
		index.Fields("created_at"),
	}
}
