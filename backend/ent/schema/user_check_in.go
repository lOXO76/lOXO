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

// UserCheckIn records one user's daily check-in reward.
type UserCheckIn struct {
	ent.Schema
}

func (UserCheckIn) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Table: "user_check_ins"}}
}

func (UserCheckIn) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.TimeMixin{}}
}

func (UserCheckIn) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Time("check_in_date").SchemaType(map[string]string{dialect.Postgres: "date"}),
		field.Float("reward").SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).Default(0),
		field.Int("tier_index").Default(0),
	}
}

func (UserCheckIn) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id", "check_in_date").Unique(),
	}
}
