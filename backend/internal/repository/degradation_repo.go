package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/degradationprobe"
	"github.com/Wei-Shaw/sub2api/ent/degradationtarget"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type degradationRepository struct {
	client *dbent.Client
	db     *sql.DB
}

func NewDegradationRepository(client *dbent.Client, db *sql.DB) service.DegradationRepository {
	return &degradationRepository{client: client, db: db}
}

func (r *degradationRepository) CreateProbe(ctx context.Context, probe *service.DegradationProbe) error {
	created, err := clientFromContext(ctx, r.client).DegradationProbe.Create().
		SetName(probe.Name).
		SetPrompt(probe.Prompt).
		SetExpectRegex(probe.ExpectRegex).
		SetRegexFlags(probe.RegexFlags).
		SetNegate(probe.Negate).
		SetEnabled(probe.Enabled).
		SetSortOrder(probe.SortOrder).
		SetProbeType(probe.ProbeType).
		SetMaxOutputTokens(probe.MaxOutputTokens).
		SetTimeoutSeconds(probe.TimeoutSeconds).
		SetRunIntervalSeconds(probe.RunIntervalSeconds).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create degradation probe: %w", err)
	}
	probe.ID, probe.CreatedAt, probe.UpdatedAt = created.ID, created.CreatedAt, created.UpdatedAt
	return nil
}

func (r *degradationRepository) GetProbe(ctx context.Context, id int64) (*service.DegradationProbe, error) {
	row, err := clientFromContext(ctx, r.client).DegradationProbe.Query().Where(degradationprobe.IDEQ(id)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrDegradationProbeNotFound
		}
		return nil, fmt.Errorf("get degradation probe: %w", err)
	}
	return entToDegradationProbe(row), nil
}

func (r *degradationRepository) UpdateProbe(ctx context.Context, probe *service.DegradationProbe) error {
	updated, err := clientFromContext(ctx, r.client).DegradationProbe.UpdateOneID(probe.ID).
		SetName(probe.Name).
		SetPrompt(probe.Prompt).
		SetExpectRegex(probe.ExpectRegex).
		SetRegexFlags(probe.RegexFlags).
		SetNegate(probe.Negate).
		SetEnabled(probe.Enabled).
		SetSortOrder(probe.SortOrder).
		SetProbeType(probe.ProbeType).
		SetMaxOutputTokens(probe.MaxOutputTokens).
		SetTimeoutSeconds(probe.TimeoutSeconds).
		SetRunIntervalSeconds(probe.RunIntervalSeconds).
		Save(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrDegradationProbeNotFound
		}
		return fmt.Errorf("update degradation probe: %w", err)
	}
	probe.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *degradationRepository) DeleteProbe(ctx context.Context, id int64) error {
	err := clientFromContext(ctx, r.client).DegradationProbe.DeleteOneID(id).Exec(ctx)
	if dbent.IsNotFound(err) {
		return service.ErrDegradationProbeNotFound
	}
	if err != nil {
		return fmt.Errorf("delete degradation probe: %w", err)
	}
	return nil
}

func (r *degradationRepository) ListProbes(ctx context.Context, includeDisabled bool) ([]*service.DegradationProbe, error) {
	query := clientFromContext(ctx, r.client).DegradationProbe.Query()
	if !includeDisabled {
		query = query.Where(degradationprobe.Enabled(true))
	}
	rows, err := query.Order(degradationprobe.BySortOrder(), degradationprobe.ByID()).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list degradation probes: %w", err)
	}
	out := make([]*service.DegradationProbe, 0, len(rows))
	for _, row := range rows {
		out = append(out, entToDegradationProbe(row))
	}
	return out, nil
}

func (r *degradationRepository) ListProbesByIDs(ctx context.Context, ids []int64, enabledOnly bool) ([]*service.DegradationProbe, error) {
	if len(ids) == 0 {
		return []*service.DegradationProbe{}, nil
	}
	query := clientFromContext(ctx, r.client).DegradationProbe.Query().Where(degradationprobe.IDIn(ids...))
	if enabledOnly {
		query = query.Where(degradationprobe.Enabled(true))
	}
	rows, err := query.Order(degradationprobe.BySortOrder(), degradationprobe.ByID()).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list degradation probes by ids: %w", err)
	}
	out := make([]*service.DegradationProbe, 0, len(rows))
	for _, row := range rows {
		out = append(out, entToDegradationProbe(row))
	}
	return out, nil
}

func entToDegradationProbe(row *dbent.DegradationProbe) *service.DegradationProbe {
	return &service.DegradationProbe{
		ID: row.ID, Name: row.Name, Prompt: row.Prompt, ExpectRegex: row.ExpectRegex,
		RegexFlags: row.RegexFlags, Negate: row.Negate, Enabled: row.Enabled,
		SortOrder: row.SortOrder, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		ProbeType: row.ProbeType, MaxOutputTokens: row.MaxOutputTokens,
		TimeoutSeconds: row.TimeoutSeconds, RunIntervalSeconds: row.RunIntervalSeconds,
	}
}

func (r *degradationRepository) CreateTarget(ctx context.Context, target *service.DegradationTarget) error {
	return r.withLiveGroup(ctx, target.GroupID, func(client *dbent.Client) error {
		created, err := client.DegradationTarget.Create().
			SetGroupID(target.GroupID).
			SetModel(target.Model).
			SetProbeIds(target.ProbeIDs).
			SetIntervalSeconds(target.IntervalSeconds).
			SetEnabled(target.Enabled).
			Save(ctx)
		if err != nil {
			if isUniqueConstraintViolation(err) {
				return service.ErrDegradationTargetDuplicate.WithCause(err)
			}
			return fmt.Errorf("create degradation target: %w", err)
		}
		target.ID, target.CreatedAt, target.UpdatedAt, target.LastRunAt = created.ID, created.CreatedAt, created.UpdatedAt, created.LastRunAt
		return nil
	})
}

func (r *degradationRepository) GetTarget(ctx context.Context, id int64) (*service.DegradationTarget, error) {
	row, err := clientFromContext(ctx, r.client).DegradationTarget.Query().Where(degradationtarget.IDEQ(id)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrDegradationTargetNotFound
		}
		return nil, fmt.Errorf("get degradation target: %w", err)
	}
	return entToDegradationTarget(row), nil
}

func (r *degradationRepository) UpdateTarget(ctx context.Context, target *service.DegradationTarget) error {
	return r.withLiveGroup(ctx, target.GroupID, func(client *dbent.Client) error {
		updated, err := client.DegradationTarget.UpdateOneID(target.ID).
			SetGroupID(target.GroupID).
			SetModel(target.Model).
			SetProbeIds(target.ProbeIDs).
			SetIntervalSeconds(target.IntervalSeconds).
			SetEnabled(target.Enabled).
			Save(ctx)
		if err != nil {
			if dbent.IsNotFound(err) {
				return service.ErrDegradationTargetNotFound
			}
			if isUniqueConstraintViolation(err) {
				return service.ErrDegradationTargetDuplicate.WithCause(err)
			}
			return fmt.Errorf("update degradation target: %w", err)
		}
		target.UpdatedAt = updated.UpdatedAt
		return nil
	})
}

func (r *degradationRepository) withLiveGroup(ctx context.Context, groupID int64, fn func(*dbent.Client) error) error {
	client := clientFromContext(ctx, r.client)
	tx, err := client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return err
	}
	if tx != nil {
		defer func() { _ = tx.Rollback() }()
		client = tx.Client()
	}
	// Match the group deletion row-lock protocol so a concurrent insert or
	// reassignment cannot leave a live target pointing at a deleted group.
	if err := lockLiveGroups(ctx, client, []int64{groupID}); err != nil {
		return err
	}
	if err := fn(client); err != nil {
		return err
	}
	if tx != nil {
		return tx.Commit()
	}
	return nil
}

func (r *degradationRepository) DeleteTarget(ctx context.Context, id int64) error {
	err := clientFromContext(ctx, r.client).DegradationTarget.DeleteOneID(id).Exec(ctx)
	if dbent.IsNotFound(err) {
		return service.ErrDegradationTargetNotFound
	}
	if err != nil {
		return fmt.Errorf("delete degradation target: %w", err)
	}
	return nil
}

func (r *degradationRepository) SoftDeleteTargetsByGroupID(ctx context.Context, groupID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE degradation_targets SET deleted_at = NOW(), updated_at = NOW() WHERE group_id = $1 AND deleted_at IS NULL`, groupID)
	if err != nil {
		return fmt.Errorf("soft-delete degradation targets by group: %w", err)
	}
	return nil
}

func (r *degradationRepository) ListTargets(ctx context.Context, groupID *int64, enabledOnly bool) ([]*service.DegradationTarget, error) {
	query := clientFromContext(ctx, r.client).DegradationTarget.Query()
	if groupID != nil {
		query = query.Where(degradationtarget.GroupIDEQ(*groupID))
	}
	if enabledOnly {
		query = query.Where(degradationtarget.Enabled(true))
	}
	query = query.Order(degradationtarget.ByGroupID(), degradationtarget.ByModel(), degradationtarget.ByID())
	rows, err := query.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list degradation targets: %w", err)
	}
	out := make([]*service.DegradationTarget, 0, len(rows))
	for _, row := range rows {
		out = append(out, entToDegradationTarget(row))
	}
	return out, nil
}

func (r *degradationRepository) ListDueTargets(ctx context.Context, now time.Time) ([]*service.DegradationTarget, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, group_id, model, probe_ids, interval_seconds, enabled, last_run_at, created_at, updated_at
FROM degradation_targets
WHERE enabled = TRUE AND deleted_at IS NULL
  AND (last_run_at IS NULL OR last_run_at + interval_seconds * INTERVAL '1 second' <= $1)
ORDER BY COALESCE(last_run_at, created_at), id`, now)
	if err != nil {
		return nil, fmt.Errorf("list due degradation targets: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]*service.DegradationTarget, 0)
	for rows.Next() {
		var target service.DegradationTarget
		var raw []byte
		if err := rows.Scan(&target.ID, &target.GroupID, &target.Model, &raw, &target.IntervalSeconds, &target.Enabled, &target.LastRunAt, &target.CreatedAt, &target.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &target.ProbeIDs); err != nil {
			return nil, fmt.Errorf("decode degradation target probe ids: %w", err)
		}
		out = append(out, &target)
	}
	return out, rows.Err()
}

func (r *degradationRepository) UpdateTargetLastRun(ctx context.Context, id int64, at time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE degradation_targets SET last_run_at = $2, updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id, at)
	if err != nil {
		return fmt.Errorf("update degradation target last run: %w", err)
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return service.ErrDegradationTargetNotFound
	}
	return nil
}

func entToDegradationTarget(row *dbent.DegradationTarget) *service.DegradationTarget {
	ids := append([]int64(nil), row.ProbeIds...)
	if ids == nil {
		ids = []int64{}
	}
	return &service.DegradationTarget{
		ID: row.ID, GroupID: row.GroupID, Model: row.Model, ProbeIDs: ids,
		IntervalSeconds: row.IntervalSeconds, Enabled: row.Enabled, LastRunAt: row.LastRunAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func (r *degradationRepository) CreateRun(ctx context.Context, run *service.DegradationRun) error {
	created, err := clientFromContext(ctx, r.client).DegradationRun.Create().
		SetTargetID(run.TargetID).
		SetGroupID(run.GroupID).
		SetModel(run.Model).
		SetProbeID(run.ProbeID).
		SetProbeName(run.ProbeName).
		SetStatus(run.Status).
		SetAnswerText(run.AnswerText).
		SetErrorMessage(run.ErrorMessage).
		SetLatencyMs(int(run.LatencyMs)).
		SetAccountID(run.AccountID).
		SetProbeType(run.ProbeType).
		SetArtifactKind(run.ArtifactKind).
		SetArtifactContent(run.ArtifactContent).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create degradation run: %w", err)
	}
	run.ID, run.CreatedAt = created.ID, created.CreatedAt
	run.ArtifactBytes = int64(len(run.ArtifactContent))
	return nil
}

func (r *degradationRepository) GetRun(ctx context.Context, id int64) (*service.DegradationRun, error) {
	return r.getRun(ctx, id, false)
}

func (r *degradationRepository) GetRunArtifact(ctx context.Context, id int64) (*service.DegradationRun, error) {
	return r.getRun(ctx, id, true)
}

func (r *degradationRepository) getRun(ctx context.Context, id int64, includeArtifact bool) (*service.DegradationRun, error) {
	query := `SELECT ` + degradationRunColumns(true, includeArtifact) + ` FROM degradation_runs WHERE id = $1`
	runs, err := r.queryRuns(ctx, query, id)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, service.ErrDegradationRunNotFound
	}
	return runs[0], nil
}

func (r *degradationRepository) ListRunsByTarget(ctx context.Context, targetID int64, limit int) ([]*service.DegradationRun, error) {
	query := `SELECT ` + degradationRunColumns(true, false) + ` FROM degradation_runs WHERE target_id = $1 ORDER BY created_at DESC,id DESC LIMIT $2`
	return r.queryRuns(ctx, query, targetID, limit)
}

func (r *degradationRepository) ListRuns(ctx context.Context, groupIDs []int64, model string, since time.Time, limit int, admin bool) ([]*service.DegradationRun, error) {
	if len(groupIDs) == 0 {
		return []*service.DegradationRun{}, nil
	}
	query := `SELECT ` + degradationRunColumns(admin, false) + ` FROM degradation_runs WHERE group_id = ANY($1) AND created_at >= $2`
	args := []any{pq.Array(groupIDs), since}
	if model != "" {
		query += ` AND model = $3`
		args = append(args, model)
	}
	if !admin {
		// Filter before LIMIT so disabled/deleted models cannot crowd out visible history.
		query += ` AND EXISTS (SELECT 1 FROM degradation_targets t WHERE t.group_id = degradation_runs.group_id AND t.model = degradation_runs.model AND t.enabled = TRUE AND t.deleted_at IS NULL)`
	}
	query += fmt.Sprintf(` ORDER BY created_at DESC,id DESC LIMIT $%d`, len(args)+1)
	args = append(args, limit)
	return r.queryRuns(ctx, query, args...)
}

func (r *degradationRepository) ListRunsSince(ctx context.Context, since time.Time, groupIDs []int64) ([]*service.DegradationRun, error) {
	query := `SELECT ` + degradationRunColumns(false, false) + ` FROM degradation_runs WHERE created_at >= $1`
	args := []any{since}
	if len(groupIDs) > 0 {
		query += ` AND group_id = ANY($2)`
		args = append(args, pq.Array(groupIDs))
	}
	query += ` ORDER BY group_id,model,created_at DESC,id DESC`
	return r.queryRuns(ctx, query, args...)
}

func (r *degradationRepository) LatestProbeRuns(ctx context.Context, targetID int64, probeIDs []int64) (map[int64]time.Time, error) {
	out := make(map[int64]time.Time)
	if len(probeIDs) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT probe_id, MAX(created_at) FROM degradation_runs WHERE target_id = $1 AND probe_id = ANY($2) GROUP BY probe_id`, targetID, pq.Array(probeIDs))
	if err != nil {
		return nil, fmt.Errorf("query latest degradation probe runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var probeID int64
		var at time.Time
		if err := rows.Scan(&probeID, &at); err != nil {
			return nil, fmt.Errorf("scan latest degradation probe run: %w", err)
		}
		out[probeID] = at
	}
	return out, rows.Err()
}

func (r *degradationRepository) ListVisualRuns(ctx context.Context, groupID int64, model string, probeID int64, limit int) ([]*service.DegradationRun, error) {
	query := `SELECT ` + degradationRunColumns(false, false) + ` FROM degradation_runs WHERE group_id = $1 AND model = $2 AND probe_id = $3 AND probe_type = 'visual' AND status = 'rendered' AND created_at >= $4 ORDER BY created_at DESC,id DESC LIMIT $5`
	return r.queryRuns(ctx, query, groupID, model, probeID, time.Now().Add(-30*24*time.Hour), limit)
}

func (r *degradationRepository) ListLatestVisualRuns(ctx context.Context, groupIDs []int64) ([]*service.DegradationRun, error) {
	if len(groupIDs) == 0 {
		return []*service.DegradationRun{}, nil
	}
	query := `SELECT DISTINCT ON (group_id,model,probe_id) ` + degradationRunColumns(false, false) + ` FROM degradation_runs WHERE group_id = ANY($1) AND probe_type = 'visual' AND status = 'rendered' AND created_at >= $2 ORDER BY group_id,model,probe_id,created_at DESC,id DESC`
	return r.queryRuns(ctx, query, pq.Array(groupIDs), time.Now().Add(-30*24*time.Hour))
}

func degradationRunColumns(includeDetails, includeArtifact bool) string {
	columns := `id,target_id,group_id,model,probe_id,probe_name,status,answer_text,error_message,latency_ms,account_id,created_at,probe_type,artifact_kind,octet_length(artifact_content) AS artifact_bytes`
	if !includeDetails {
		columns = `id,target_id,group_id,model,probe_id,probe_name,status,'' AS answer_text,'' AS error_message,latency_ms,0 AS account_id,created_at,probe_type,artifact_kind,octet_length(artifact_content) AS artifact_bytes`
	}
	if includeArtifact {
		return columns + `,artifact_content`
	}
	return columns + `,'' AS artifact_content`
}

func (r *degradationRepository) queryRuns(ctx context.Context, query string, args ...any) ([]*service.DegradationRun, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query degradation runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]*service.DegradationRun, 0)
	for rows.Next() {
		var row service.DegradationRun
		if err := rows.Scan(&row.ID, &row.TargetID, &row.GroupID, &row.Model, &row.ProbeID, &row.ProbeName, &row.Status, &row.AnswerText, &row.ErrorMessage, &row.LatencyMs, &row.AccountID, &row.CreatedAt, &row.ProbeType, &row.ArtifactKind, &row.ArtifactBytes, &row.ArtifactContent); err != nil {
			return nil, err
		}
		out = append(out, &row)
	}
	return out, rows.Err()
}

func (r *degradationRepository) DeleteRunsBefore(ctx context.Context, before time.Time) (int64, error) {
	var total int64
	for {
		result, err := r.db.ExecContext(ctx, `DELETE FROM degradation_runs WHERE id IN (SELECT id FROM degradation_runs WHERE created_at < $1 ORDER BY created_at LIMIT 5000)`, before)
		if err != nil {
			return total, fmt.Errorf("delete old degradation runs: %w", err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return total, err
		}
		total += count
		if count < 5000 {
			return total, nil
		}
	}
}

var _ service.DegradationRepository = (*degradationRepository)(nil)
