package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
)

type Ticket struct{ ent.Schema }

func (Ticket) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "tickets"}}
}

func (Ticket) Mixin() []ent.Mixin { return []ent.Mixin{mixins.TimeMixin{}} }

func (Ticket) Fields() []ent.Field {
	return []ent.Field{
		field.String("ticket_no").MaxRuneLen(16).SchemaType(map[string]string{dialect.Postgres: "varchar(16)"}),
		field.Int64("user_id"),
		field.String("subject").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("category").MaxRuneLen(20).Default("general"),
		field.String("priority").MaxRuneLen(20).Default("normal"),
		field.String("status").MaxRuneLen(20).Default("pending_admin"),
		field.Int64("assignee_id").Optional().Nillable(),
		field.Bool("user_unread").Default(false),
		field.Bool("admin_unread").Default(true),
		field.Int("message_count").Default(0),
		field.Time("last_message_at").Default(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.String("last_message_by").MaxRuneLen(20).Default("user"),
		field.Time("closed_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (Ticket) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("ticket_no").Unique(),
		index.Fields("user_id", "updated_at").StorageKey("tickets_user_updated").Annotations(entsql.DescColumns("updated_at")),
		index.Fields("status", "updated_at").StorageKey("tickets_status_updated").Annotations(entsql.DescColumns("updated_at")),
		index.Fields("assignee_id").StorageKey("tickets_assignee"),
	}
}
