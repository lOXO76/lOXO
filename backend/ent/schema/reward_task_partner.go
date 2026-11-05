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

type RewardTaskPartner struct{ ent.Schema }

func (RewardTaskPartner) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "reward_task_partners"}}
}
func (RewardTaskPartner) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}, mixins.SoftDeleteMixin{}}
}
func (RewardTaskPartner) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxRuneLen(100).SchemaType(map[string]string{dialect.Postgres: "varchar(100)"}),
		field.String("app_key").MaxRuneLen(64).SchemaType(map[string]string{dialect.Postgres: "varchar(64)"}),
		field.String("app_secret_encrypted").SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("secret_hint").MaxRuneLen(8).SchemaType(map[string]string{dialect.Postgres: "varchar(8)"}).Default(""),
		field.JSON("allowed_domains", []string{}).Default([]string{}).SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("contact").MaxRuneLen(200).SchemaType(map[string]string{dialect.Postgres: "varchar(200)"}).Default(""),
		field.String("notes").SchemaType(map[string]string{dialect.Postgres: "text"}).Default(""),
		field.Bool("enabled").Default(true),
		field.Time("last_event_at").Optional().Nillable().SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}
func (RewardTaskPartner) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("app_key").Unique(),
	}
}
