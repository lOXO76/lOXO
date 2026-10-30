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

// DegradationTarget configures checks for a model in a group.
type DegradationTarget struct {
	ent.Schema
}

func (DegradationTarget) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "degradation_targets"},
	}
}

func (DegradationTarget) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (DegradationTarget) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("group_id"),
		field.String("model").
			NotEmpty().
			MaxRuneLen(200).
			SchemaType(map[string]string{dialect.Postgres: "varchar(200)"}),
		field.JSON("probe_ids", []int64{}).
			Default([]int64{}).
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Int("interval_seconds").
			Default(1800),
		field.Bool("enabled").
			Default(true),
		field.Time("last_run_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (DegradationTarget) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id", "model").
			Unique().
			StorageKey("idx_degradation_targets_group_model").
			Annotations(entsql.IndexWhere("deleted_at IS NULL")),
	}
}
