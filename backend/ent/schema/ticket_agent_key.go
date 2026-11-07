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

type TicketAgentKey struct{ ent.Schema }

func (TicketAgentKey) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ticket_agent_keys"}}
}

func (TicketAgentKey) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}, mixins.SoftDeleteMixin{}}
}

func (TicketAgentKey) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.String("name").MaxRuneLen(100),
		field.String("key_prefix").MaxRuneLen(12).Sensitive(),
		field.String("key_hash").MaxRuneLen(64).Sensitive(),
		field.String("scope").MaxRuneLen(20).Default("user"),
		field.Int("qps_limit").Default(2),
		field.String("status").MaxRuneLen(20).Default("active"),
		field.Time("last_used_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("last_used_ip").MaxRuneLen(64).Optional().Nillable(),
		field.Time("expires_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (TicketAgentKey) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("key_hash").Unique(),
		index.Fields("user_id", "deleted_at").StorageKey("ticket_agent_keys_owner"),
	}
}
