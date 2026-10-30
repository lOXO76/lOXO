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

// DegradationRun records the result for one prompt execution.
type DegradationRun struct {
	ent.Schema
}

func (DegradationRun) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "degradation_runs"},
	}
}

func (DegradationRun) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("target_id"),
		field.Int64("group_id"),
		field.String("model").
			NotEmpty().
			MaxRuneLen(200).
			SchemaType(map[string]string{dialect.Postgres: "varchar(200)"}),
		field.Int64("probe_id"),
		field.String("probe_name").
			NotEmpty().
			MaxRuneLen(100).
			SchemaType(map[string]string{dialect.Postgres: "varchar(100)"}),
		field.String("status").
			NotEmpty().
			MaxLen(16),
		field.String("answer_text").
			Default("").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.String("error_message").
			Default("").
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int("latency_ms").
			Default(0),
		field.Int64("account_id").
			Default(0),
		field.String("probe_type").
			Default("regex").
			MaxLen(16).
			SchemaType(map[string]string{dialect.Postgres: "varchar(16)"}),
		field.String("artifact_kind").
			Default("").
			MaxLen(16).
			SchemaType(map[string]string{dialect.Postgres: "varchar(16)"}),
		field.String("artifact_content").
			Default("").
			Sensitive().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (DegradationRun) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id", "created_at").
			StorageKey("idx_degradation_runs_group_created").
			Annotations(entsql.DescColumns("created_at")),
		index.Fields("target_id", "created_at").
			StorageKey("idx_degradation_runs_target_created").
			Annotations(entsql.DescColumns("created_at")),
		index.Fields("created_at").
			StorageKey("idx_degradation_runs_created"),
		index.Fields("group_id", "model", "probe_id", "created_at").
			StorageKey("idx_degradation_runs_visual_latest").
			Annotations(entsql.DescColumns("created_at"), entsql.IndexWhere("probe_type = 'visual' AND status = 'rendered'")),
		index.Fields("target_id", "probe_id", "created_at").
			StorageKey("idx_degradation_runs_target_probe_created").
			Annotations(entsql.DescColumns("created_at")),
	}
}
