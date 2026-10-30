//go:build unit

package repository

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestDegradationRepositoryUserRunsFilterBeforeLimitAndRedact(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &degradationRepository{db: db}
	since := time.Now().Add(-24 * time.Hour)
	query := `SELECT ` + degradationRunColumns(false, false) + ` FROM degradation_runs WHERE group_id = ANY($1) AND created_at >= $2 AND model = $3 AND EXISTS (SELECT 1 FROM degradation_targets t WHERE t.group_id = degradation_runs.group_id AND t.model = degradation_runs.model AND t.enabled = TRUE AND t.deleted_at IS NULL) ORDER BY created_at DESC,id DESC LIMIT $4`
	rows := degradationRunMockRows().AddRow(1, 2, 12, "model", 3, "probe", "passed", "", "", 100, 0, time.Now(), "regex", "", 0, "")
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs("{12,13}", since, "model", 50).WillReturnRows(rows)
	runs, err := repo.ListRuns(context.Background(), []int64{12, 13}, "model", since, 50, false)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Empty(t, runs[0].AnswerText)
	require.Empty(t, runs[0].ErrorMessage)
	require.Zero(t, runs[0].AccountID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDegradationRepositoryEmptyVisibilityNeverQueriesRuns(t *testing.T) {
	repo := &degradationRepository{}
	runs, err := repo.ListRuns(context.Background(), nil, "", time.Now(), 50, false)
	require.NoError(t, err)
	require.NotNil(t, runs)
	require.Empty(t, runs)
}

func degradationRunMockRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "target_id", "group_id", "model", "probe_id", "probe_name", "status", "answer_text", "error_message", "latency_ms", "account_id", "created_at", "probe_type", "artifact_kind", "artifact_bytes", "artifact_content"})
}

func TestDegradationRepositoryRunProjectionOmitsArtifactContent(t *testing.T) {
	for _, includeDetails := range []bool{false, true} {
		columns := degradationRunColumns(includeDetails, false)
		require.Contains(t, columns, "octet_length(artifact_content) AS artifact_bytes")
		require.Contains(t, columns, "'' AS artifact_content")
		for _, column := range strings.Split(columns, ",") {
			require.NotEqual(t, "artifact_content", strings.TrimSpace(column))
		}
	}
	require.True(t, strings.HasSuffix(degradationRunColumns(true, true), ",artifact_content"))
}

func TestDegradationRepositoryTargetRunsDoNotReadArtifact(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &degradationRepository{db: db}
	at := time.Now()
	query := `SELECT ` + degradationRunColumns(true, false) + ` FROM degradation_runs WHERE target_id = $1 ORDER BY created_at DESC,id DESC LIMIT $2`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(int64(2), 50).
		WillReturnRows(degradationRunMockRows().AddRow(1, 2, 12, "model", 3, "probe", "rendered", "preview", "", 100, 4, at, "visual", "html", 12345, ""))

	runs, err := repo.ListRunsByTarget(context.Background(), 2, 50)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, "visual", runs[0].ProbeType)
	require.Equal(t, "html", runs[0].ArtifactKind)
	require.Equal(t, int64(12345), runs[0].ArtifactBytes)
	require.Empty(t, runs[0].ArtifactContent)
	require.Equal(t, "preview", runs[0].AnswerText)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDegradationRepositoryGetRunOnlyArtifactReaderLoadsContent(t *testing.T) {
	for _, includeArtifact := range []bool{false, true} {
		name := "metadata"
		if includeArtifact {
			name = "artifact"
		}
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			repo := &degradationRepository{db: db}
			content := ""
			if includeArtifact {
				content = "<html>content</html>"
			}
			query := `SELECT ` + degradationRunColumns(true, includeArtifact) + ` FROM degradation_runs WHERE id = $1`
			mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(int64(1)).
				WillReturnRows(degradationRunMockRows().AddRow(1, 2, 12, "model", 3, "probe", "rendered", "preview", "", 100, 4, time.Now(), "visual", "html", 20, content))
			var run *service.DegradationRun
			if includeArtifact {
				run, err = repo.GetRunArtifact(context.Background(), 1)
			} else {
				run, err = repo.GetRun(context.Background(), 1)
			}
			require.NoError(t, err)
			require.Equal(t, content, run.ArtifactContent)
			require.Equal(t, int64(20), run.ArtifactBytes)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDegradationRepositoryGetRunMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &degradationRepository{db: db}
	query := `SELECT ` + degradationRunColumns(true, false) + ` FROM degradation_runs WHERE id = $1`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(int64(99)).WillReturnRows(degradationRunMockRows())
	run, err := repo.GetRun(context.Background(), 99)
	require.Nil(t, run)
	require.ErrorIs(t, err, service.ErrDegradationRunNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDegradationRepositoryLatestProbeRunsUsesTargetAndRequestedProbes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &degradationRepository{db: db}
	at := time.Now()
	query := `SELECT probe_id, MAX(created_at) FROM degradation_runs WHERE target_id = $1 AND probe_id = ANY($2) GROUP BY probe_id`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(int64(2), "{3,4}").
		WillReturnRows(sqlmock.NewRows([]string{"probe_id", "max"}).AddRow(3, at).AddRow(4, at.Add(-time.Hour)))
	got, err := repo.LatestProbeRuns(context.Background(), 2, []int64{3, 4})
	require.NoError(t, err)
	require.Equal(t, map[int64]time.Time{3: at, 4: at.Add(-time.Hour)}, got)
	require.NoError(t, mock.ExpectationsWereMet())

	got, err = (&degradationRepository{}).LatestProbeRuns(context.Background(), 2, nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)
}

func TestDegradationRepositoryVisualListsUseRenderedThirtyDayHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &degradationRepository{db: db}
	at := time.Now()
	query := `SELECT ` + degradationRunColumns(false, false) + ` FROM degradation_runs WHERE group_id = $1 AND model = $2 AND probe_id = $3 AND probe_type = 'visual' AND status = 'rendered' AND created_at >= $4 ORDER BY created_at DESC,id DESC LIMIT $5`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(int64(12), "model", int64(3), sqlmock.AnyArg(), 20).
		WillReturnRows(degradationRunMockRows().AddRow(1, 2, 12, "model", 3, "probe", "rendered", "", "", 100, 0, at, "visual", "svg", 10000, ""))
	items, err := repo.ListVisualRuns(context.Background(), 12, "model", 3, 20)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "rendered", items[0].Status)
	require.Empty(t, items[0].ArtifactContent)

	query = `SELECT DISTINCT ON (group_id,model,probe_id) ` + degradationRunColumns(false, false) + ` FROM degradation_runs WHERE group_id = ANY($1) AND probe_type = 'visual' AND status = 'rendered' AND created_at >= $2 ORDER BY group_id,model,probe_id,created_at DESC,id DESC`
	mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs("{12,13}", sqlmock.AnyArg()).WillReturnRows(degradationRunMockRows())
	items, err = repo.ListLatestVisualRuns(context.Background(), []int64{12, 13})
	require.NoError(t, err)
	require.Empty(t, items)
	require.NoError(t, mock.ExpectationsWereMet())

	items, err = (&degradationRepository{}).ListLatestVisualRuns(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, items)
	require.Empty(t, items)
}

func TestDegradationRepositoryProbeMappingIncludesVisualConfiguration(t *testing.T) {
	probe := entToDegradationProbe(&dbent.DegradationProbe{
		ID: 3, ProbeType: "visual", MaxOutputTokens: 8192, TimeoutSeconds: 300, RunIntervalSeconds: 86400,
	})
	require.Equal(t, "visual", probe.ProbeType)
	require.Equal(t, 8192, probe.MaxOutputTokens)
	require.Equal(t, 300, probe.TimeoutSeconds)
	require.Equal(t, 86400, probe.RunIntervalSeconds)
}

func TestDegradationRepositoryArtifactContentIsNeverJSON(t *testing.T) {
	encoded, err := json.Marshal(service.DegradationRun{ID: 1, ArtifactKind: "html", ArtifactContent: "private-large-body", ArtifactBytes: 18})
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "artifact_content")
	require.NotContains(t, string(encoded), "private-large-body")
	require.Contains(t, string(encoded), `"artifact_bytes":18`)
}

func TestDegradationRepositoryCreateProbePersistsVisualConfiguration(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	repo := &degradationRepository{client: client, db: db}
	probe := &service.DegradationProbe{
		Name: "visual probe", Prompt: "generate html", Enabled: true, SortOrder: 2,
		ProbeType: "visual", MaxOutputTokens: 8192, TimeoutSeconds: 300, RunIntervalSeconds: 86400,
	}
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "degradation_probes"`)+`.*`+regexp.QuoteMeta(`"probe_type", "max_output_tokens", "timeout_seconds", "run_interval_seconds"`)).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), probe.Name, probe.Prompt, "", "", false, true, 2, "visual", 8192, 300, 86400).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3))

	require.NoError(t, repo.CreateProbe(context.Background(), probe))
	require.Equal(t, int64(3), probe.ID)
	require.False(t, probe.CreatedAt.IsZero())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDegradationRepositoryCreateRunPersistsSnapshotAndArtifact(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	repo := &degradationRepository{client: client, db: db}
	run := &service.DegradationRun{
		TargetID: 2, GroupID: 12, Model: "model", ProbeID: 3, ProbeName: "probe",
		ProbeType: "visual", Status: "rendered", AnswerText: "preview", LatencyMs: 100, AccountID: 4,
		ArtifactKind: "html", ArtifactContent: "<html>界</html>",
	}
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO "degradation_runs"`)+`.*`+regexp.QuoteMeta(`"probe_type", "artifact_kind", "artifact_content"`)).
		WithArgs(int64(2), int64(12), "model", int64(3), "probe", "rendered", "preview", "", 100, int64(4), "visual", "html", run.ArtifactContent, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))

	require.NoError(t, repo.CreateRun(context.Background(), run))
	require.Equal(t, int64(1), run.ID)
	require.Equal(t, int64(len(run.ArtifactContent)), run.ArtifactBytes)
	require.False(t, run.CreatedAt.IsZero())
	require.NoError(t, mock.ExpectationsWereMet())
}
