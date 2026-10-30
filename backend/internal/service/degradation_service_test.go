//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type degradationServiceRepoStub struct {
	DegradationRepository
	probes             []*DegradationProbe
	targets            []*DegradationTarget
	runs               []*DegradationRun
	createTargetErr    error
	createdTargets     []*DegradationTarget
	targetsEnabledOnly bool
	runGroupIDs        []int64
}

func (r *degradationServiceRepoStub) CreateTarget(_ context.Context, target *DegradationTarget) error {
	r.createdTargets = append(r.createdTargets, target)
	return r.createTargetErr
}

func (r *degradationServiceRepoStub) ListProbes(context.Context, bool) ([]*DegradationProbe, error) {
	return r.probes, nil
}

func (r *degradationServiceRepoStub) ListTargets(_ context.Context, groupID *int64, enabledOnly bool) ([]*DegradationTarget, error) {
	r.targetsEnabledOnly = enabledOnly
	out := make([]*DegradationTarget, 0, len(r.targets))
	for _, target := range r.targets {
		if groupID != nil && target.GroupID != *groupID || enabledOnly && !target.Enabled {
			continue
		}
		out = append(out, target)
	}
	return out, nil
}

func (r *degradationServiceRepoStub) ListRunsSince(_ context.Context, _ time.Time, groupIDs []int64) ([]*DegradationRun, error) {
	r.runGroupIDs = append([]int64(nil), groupIDs...)
	return r.runs, nil
}

type degradationServiceGroupRepoStub struct {
	GroupRepository
	groups map[int64]*Group
}

func (r *degradationServiceGroupRepoStub) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	if group := r.groups[id]; group != nil {
		return group, nil
	}
	return nil, ErrGroupNotFound
}

func TestDegradationModelVerdict(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statuses   []string
		verdict    string
		passed     int64
		failed     int64
		errorCount int64
		passRate   float64
	}{
		{name: "no runs", verdict: DegradationVerdictUnknown},
		{name: "two successful samples are insufficient", statuses: []string{DegradationRunPassed, DegradationRunPassed}, verdict: DegradationVerdictUnknown, passed: 2, passRate: 1},
		{name: "errors do not satisfy sample minimum", statuses: []string{DegradationRunPassed, DegradationRunFailed, DegradationRunError, DegradationRunError}, verdict: DegradationVerdictUnknown, passed: 1, failed: 1, errorCount: 2, passRate: 0.5},
		{name: "three passed samples", statuses: []string{DegradationRunPassed, DegradationRunPassed, DegradationRunPassed}, verdict: DegradationVerdictHealthy, passed: 3, passRate: 1},
		{name: "below threshold", statuses: []string{DegradationRunPassed, DegradationRunPassed, DegradationRunFailed}, verdict: DegradationVerdictDegraded, passed: 2, failed: 1, passRate: 2.0 / 3},
		{name: "threshold boundary is healthy", statuses: []string{DegradationRunPassed, DegradationRunPassed, DegradationRunPassed, DegradationRunPassed, DegradationRunFailed}, verdict: DegradationVerdictHealthy, passed: 4, failed: 1, passRate: 0.8},
		{name: "errors excluded from denominator", statuses: []string{DegradationRunPassed, DegradationRunPassed, DegradationRunPassed, DegradationRunPassed, DegradationRunFailed, DegradationRunError, DegradationRunError}, verdict: DegradationVerdictHealthy, passed: 4, failed: 1, errorCount: 2, passRate: 0.8},
		{name: "only errors", statuses: []string{DegradationRunError, DegradationRunError, DegradationRunError}, verdict: DegradationVerdictUnknown, errorCount: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runs := make([]*DegradationRun, 0, len(tt.statuses))
			for _, status := range tt.statuses {
				runs = append(runs, &DegradationRun{Status: status})
			}
			got := summarizeDegradationModel("model-a", runs, 0.8)
			require.Equal(t, "model-a", got.Model)
			require.Equal(t, tt.verdict, got.Verdict)
			require.Equal(t, int64(len(tt.statuses)), got.Total)
			require.Equal(t, tt.passed, got.Passed)
			require.Equal(t, tt.failed, got.Failed)
			require.Equal(t, tt.errorCount, got.Error)
			require.InDelta(t, tt.passRate, got.PassRate, 1e-12)
		})
	}
}

func TestDegradationModelSummaryExcludesErrorLatencyAndUsesLatestRun(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.October, 30, 10, 0, 0, 0, time.UTC)
	runs := []*DegradationRun{
		{Status: DegradationRunFailed, LatencyMs: 200, CreatedAt: at.Add(-time.Minute)},
		{Status: DegradationRunError, LatencyMs: 90000, CreatedAt: at},
		{Status: DegradationRunPassed, LatencyMs: 100, CreatedAt: at.Add(-3 * time.Minute)},
		{Status: DegradationRunPassed, LatencyMs: 300, CreatedAt: at.Add(-2 * time.Minute)},
	}
	got := summarizeDegradationModel("model-a", runs, 0.8)

	require.Equal(t, int64(200), got.AvgLatencyMs)
	require.InDelta(t, 2.0/3, got.PassRate, 1e-12)
	require.Equal(t, DegradationVerdictDegraded, got.Verdict)
	require.Equal(t, DegradationRunError, got.LastRunStatus)
	require.Equal(t, &at, got.LastCheckedAt)
}

func TestDegradationCreateTargetDuplicateConflict(t *testing.T) {
	t.Parallel()

	repo := &degradationServiceRepoStub{
		probes:          []*DegradationProbe{{ID: 1, Enabled: true}},
		createTargetErr: ErrDegradationTargetDuplicate,
	}
	svc := NewDegradationService(repo, nil, nil, nil, nil)
	target, err := svc.CreateTarget(context.Background(), DegradationTargetParams{
		GroupID: 12, Model: "model-a", IntervalSeconds: 1800,
	})

	require.Nil(t, target)
	require.ErrorIs(t, err, ErrDegradationTargetDuplicate)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Equal(t, "DEGRADATION_TARGET_DUPLICATE", infraerrors.Reason(err))
	require.Len(t, repo.createdTargets, 1)
	require.Equal(t, int64(12), repo.createdTargets[0].GroupID)
	require.Equal(t, "model-a", repo.createdTargets[0].Model)
}

func TestDegradationOverviewSortsGroupsModelsAndKeepsTargetsWithoutRuns(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.October, 30, 10, 0, 0, 0, time.UTC)
	repo := &degradationServiceRepoStub{
		targets: []*DegradationTarget{
			{ID: 201, GroupID: 20, Model: "zeta", Enabled: true},
			{ID: 302, GroupID: 3, Model: "zeta", Enabled: true},
			{ID: 501, GroupID: 5, Model: "no-runs", Enabled: true},
			{ID: 301, GroupID: 3, Model: "alpha", Enabled: true},
			{ID: 202, GroupID: 20, Model: "alpha", Enabled: true},
		},
		runs: []*DegradationRun{
			{TargetID: 301, GroupID: 3, Model: "alpha", Status: DegradationRunFailed, CreatedAt: at.Add(-time.Minute)},
			{TargetID: 302, GroupID: 3, Model: "zeta", Status: DegradationRunPassed, CreatedAt: at},
			{TargetID: 301, GroupID: 3, Model: "alpha", Status: DegradationRunPassed, CreatedAt: at.Add(-3 * time.Minute)},
			{TargetID: 301, GroupID: 3, Model: "alpha", Status: DegradationRunPassed, CreatedAt: at.Add(-2 * time.Minute)},
			{TargetID: 302, GroupID: 3, Model: "zeta", Status: DegradationRunPassed, CreatedAt: at.Add(-2 * time.Minute)},
			{TargetID: 302, GroupID: 3, Model: "zeta", Status: DegradationRunPassed, CreatedAt: at.Add(-time.Minute)},
			{TargetID: 201, GroupID: 20, Model: "zeta", Status: DegradationRunPassed, CreatedAt: at.Add(-time.Hour)},
			{TargetID: 201, GroupID: 20, Model: "zeta", Status: DegradationRunPassed, CreatedAt: at.Add(-2 * time.Hour)},
			{TargetID: 201, GroupID: 20, Model: "zeta", Status: DegradationRunPassed, CreatedAt: at.Add(-3 * time.Hour)},
			{TargetID: 999, GroupID: 99, Model: "unconfigured", Status: DegradationRunFailed, CreatedAt: at},
		},
	}
	groups := &degradationServiceGroupRepoStub{groups: map[int64]*Group{
		3:  {ID: 3, Name: "Zulu", Platform: "anthropic"},
		5:  {ID: 5, Name: "Middle", Platform: "gemini"},
		20: {ID: 20, Name: "Alpha", Platform: "openai"},
	}}
	svc := NewDegradationService(repo, groups, nil, nil, nil)
	got, err := svc.GetOverview(context.Background(), nil, 24, false)

	require.NoError(t, err)
	require.True(t, got.Enabled)
	require.Equal(t, 24, got.WindowHours)
	require.InDelta(t, 0.8, got.Threshold, 1e-12)
	require.True(t, repo.targetsEnabledOnly)
	require.ElementsMatch(t, []int64{3, 5, 20}, repo.runGroupIDs)
	require.Len(t, got.Groups, 3)
	require.Equal(t, []int64{3, 5, 20}, []int64{got.Groups[0].GroupID, got.Groups[1].GroupID, got.Groups[2].GroupID})
	require.Equal(t, "Zulu", got.Groups[0].GroupName)
	require.Equal(t, "anthropic", got.Groups[0].Platform)
	require.Equal(t, DegradationVerdictDegraded, got.Groups[0].Verdict)
	require.Equal(t, &at, got.Groups[0].LastCheckedAt)
	require.Equal(t, []string{"alpha", "zeta"}, []string{got.Groups[0].Models[0].Model, got.Groups[0].Models[1].Model})
	require.Equal(t, DegradationVerdictDegraded, got.Groups[0].Models[0].Verdict)
	require.Equal(t, DegradationVerdictHealthy, got.Groups[0].Models[1].Verdict)

	require.Equal(t, DegradationVerdictUnknown, got.Groups[1].Verdict)
	require.Nil(t, got.Groups[1].LastCheckedAt)
	require.Len(t, got.Groups[1].Models, 1)
	emptyModel := got.Groups[1].Models[0]
	require.Equal(t, "no-runs", emptyModel.Model)
	require.Equal(t, DegradationVerdictUnknown, emptyModel.Verdict)
	require.Zero(t, emptyModel.Total)
	require.Zero(t, emptyModel.PassRate)
	require.Zero(t, emptyModel.AvgLatencyMs)
	require.Nil(t, emptyModel.LastCheckedAt)
	require.Empty(t, emptyModel.LastRunStatus)

	require.Equal(t, DegradationVerdictHealthy, got.Groups[2].Verdict)
	require.Equal(t, []string{"alpha", "zeta"}, []string{got.Groups[2].Models[0].Model, got.Groups[2].Models[1].Model})
	require.Equal(t, DegradationVerdictUnknown, got.Groups[2].Models[0].Verdict)
	require.Equal(t, DegradationVerdictHealthy, got.Groups[2].Models[1].Verdict)
}

func TestDegradationOverviewWithoutTargetsIsEmpty(t *testing.T) {
	t.Parallel()

	svc := NewDegradationService(&degradationServiceRepoStub{}, nil, nil, nil, nil)
	got, err := svc.GetOverview(context.Background(), nil, 24)

	require.NoError(t, err)
	require.NotNil(t, got.Groups)
	require.Empty(t, got.Groups)
}

func TestDegradationOverviewAggregatesRecreatedTargetHistoryByGroupAndModel(t *testing.T) {
	t.Parallel()

	repo := &degradationServiceRepoStub{
		targets: []*DegradationTarget{{ID: 20, GroupID: 12, Model: "model-a", Enabled: true}},
		runs: []*DegradationRun{
			{TargetID: 10, GroupID: 12, Model: "model-a", Status: DegradationRunPassed},
			{TargetID: 10, GroupID: 12, Model: "model-a", Status: DegradationRunPassed},
			{TargetID: 20, GroupID: 12, Model: "model-a", Status: DegradationRunPassed},
			{TargetID: 10, GroupID: 13, Model: "model-a", Status: DegradationRunFailed},
			{TargetID: 10, GroupID: 12, Model: "model-b", Status: DegradationRunFailed},
		},
	}
	svc := NewDegradationService(repo, nil, nil, nil, nil)
	got, err := svc.GetOverview(context.Background(), nil, 24)

	require.NoError(t, err)
	require.Len(t, got.Groups, 1)
	require.Equal(t, int64(12), got.Groups[0].GroupID)
	require.Len(t, got.Groups[0].Models, 1)
	model := got.Groups[0].Models[0]
	require.Equal(t, "model-a", model.Model)
	require.Equal(t, int64(3), model.Total)
	require.Equal(t, int64(3), model.Passed)
	require.Zero(t, model.Failed)
	require.Equal(t, DegradationVerdictHealthy, model.Verdict)
}
