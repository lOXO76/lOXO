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

type TicketMessage struct{ ent.Schema }

func (TicketMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "ticket_messages"}}
}

func (TicketMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("ticket_id"),
		field.String("author_type").MaxRuneLen(20),
		field.Int64("author_id").Optional().Nillable(),
		field.String("via").MaxRuneLen(20).Default("web"),
		field.Int64("agent_key_id").Optional().Nillable(),
		field.Bool("internal").Default(false),
		field.String("body").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("created_at").Immutable().Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (TicketMessage) Indexes() []ent.Index {
	return []ent.Index{index.Fields("ticket_id", "id").StorageKey("ticket_messages_ticket_id")}
}
