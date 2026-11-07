package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

type TicketNotifyPreference struct{ ent.Schema }

func (TicketNotifyPreference) Annotations() []schema.Annotation {
	incremental := false
	return []schema.Annotation{entsql.Annotation{Table: "ticket_notify_preferences", Incremental: &incremental}}
}

func (TicketNotifyPreference) Fields() []ent.Field {
	incremental := false
	return []ent.Field{
		field.Int64("id").StorageKey("user_id").Annotations(entsql.Annotation{Incremental: &incremental}),
		field.Bool("email_enabled").Default(true),
		field.Bool("feishu_enabled").Default(false),
		field.String("feishu_webhook_url_encrypted").Sensitive().Default("").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("feishu_secret_encrypted").Sensitive().Default("").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}
