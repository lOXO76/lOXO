package schema

import (
	"fmt"
	"strings"
	"testing"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/field"
	"github.com/stretchr/testify/require"
)

func TestDegradationSchemaCharacterLengthBoundaries(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		fields []ent.Field
		field  string
		limit  int
	}{
		{name: "probe name", fields: DegradationProbe{}.Fields(), field: "name", limit: 100},
		{name: "target model", fields: DegradationTarget{}.Fields(), field: "model", limit: 200},
		{name: "run model", fields: DegradationRun{}.Fields(), field: "model", limit: 200},
		{name: "run probe name", fields: DegradationRun{}.Fields(), field: "probe_name", limit: 100},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var descriptor *field.Descriptor
			for _, entField := range tt.fields {
				if entField.Descriptor().Name == tt.field {
					descriptor = entField.Descriptor()
					break
				}
			}
			require.NotNil(t, descriptor)
			require.Equal(t, fmt.Sprintf("varchar(%d)", tt.limit), descriptor.SchemaType[dialect.Postgres])
			require.NotEmpty(t, descriptor.Validators)

			validate := func(value string) error {
				for _, rawValidator := range descriptor.Validators {
					validator, ok := rawValidator.(func(string) error)
					require.True(t, ok)
					if err := validator(value); err != nil {
						return err
					}
				}
				return nil
			}

			require.Error(t, validate(""))
			for _, character := range []string{"a", "界"} {
				require.NoError(t, validate(strings.Repeat(character, tt.limit-1)))
				require.NoError(t, validate(strings.Repeat(character, tt.limit)))
				require.Error(t, validate(strings.Repeat(character, tt.limit+1)))
			}
		})
	}
}

func TestDegradationVisualSchemaFieldsAndIndexes(t *testing.T) {
	t.Parallel()

	probeFields := DegradationProbe{}.Fields()
	probeType := schemaFieldDescriptor(t, probeFields, "probe_type")
	require.Equal(t, "regex", probeType.Default)
	require.Equal(t, "varchar(16)", probeType.SchemaType[dialect.Postgres])
	for _, name := range []string{"max_output_tokens", "timeout_seconds", "run_interval_seconds"} {
		require.Equal(t, 0, schemaFieldDescriptor(t, probeFields, name).Default)
	}

	runFields := DegradationRun{}.Fields()
	for _, name := range []string{"probe_type", "artifact_kind"} {
		descriptor := schemaFieldDescriptor(t, runFields, name)
		require.Equal(t, "varchar(16)", descriptor.SchemaType[dialect.Postgres])
	}
	artifact := schemaFieldDescriptor(t, runFields, "artifact_content")
	require.Equal(t, "text", artifact.SchemaType[dialect.Postgres])
	require.True(t, artifact.Sensitive)

	indexes := DegradationRun{}.Indexes()
	var visualIndex, targetProbeIndex bool
	for _, index := range indexes {
		descriptor := index.Descriptor()
		if descriptor.StorageKey == "idx_degradation_runs_visual_latest" {
			visualIndex = true
			require.Equal(t, []string{"group_id", "model", "probe_id", "created_at"}, descriptor.Fields)
			var predicate string
			var descending bool
			for _, rawAnnotation := range descriptor.Annotations {
				annotation, ok := rawAnnotation.(*entsql.IndexAnnotation)
				require.True(t, ok)
				if annotation.Where != "" {
					predicate = annotation.Where
				}
				descending = descending || annotation.DescColumns["created_at"]
			}
			require.Equal(t, "probe_type = 'visual' AND status = 'rendered'", predicate)
			require.True(t, descending)
		}
		if descriptor.StorageKey == "idx_degradation_runs_target_probe_created" {
			targetProbeIndex = true
			require.Equal(t, []string{"target_id", "probe_id", "created_at"}, descriptor.Fields)
		}
	}
	require.True(t, visualIndex)
	require.True(t, targetProbeIndex)
}

func schemaFieldDescriptor(t *testing.T, fields []ent.Field, name string) *field.Descriptor {
	t.Helper()
	for _, entField := range fields {
		if entField.Descriptor().Name == name {
			return entField.Descriptor()
		}
	}
	require.Failf(t, "missing schema field", "field %s not found", name)
	return nil
}
