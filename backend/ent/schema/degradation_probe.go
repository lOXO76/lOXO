package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// DegradationProbe is a prompt and matching rule used by degradation checks.
type DegradationProbe struct {
	ent.Schema
}

func (DegradationProbe) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "degradation_probes"},
	}
}

func (DegradationProbe) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (DegradationProbe) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty().
			MaxRuneLen(100).
			SchemaType(map[string]string{dialect.Postgres: "varchar(100)"}),
		field.String("prompt").
			NotEmpty().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("expect_regex").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("regex_flags").
			Default("").
			MaxLen(8),
		field.Bool("negate").
			Default(false),
		field.Bool("enabled").
			Default(true),
		field.Int("sort_order").
			Default(0),
		field.String("probe_type").
			Default("regex").
			MaxLen(16).
			SchemaType(map[string]string{dialect.Postgres: "varchar(16)"}),
		field.Int("max_output_tokens").
			Default(0),
		field.Int("timeout_seconds").
			Default(0),
		field.Int("run_interval_seconds").
			Default(0),
	}
}
