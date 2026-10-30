//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type degradationExecutorStub struct {
	answer  string
	options []DegradationProbeExecOptions
	started chan context.Context
	release chan struct{}
	once    sync.Once
}

func (s *degradationExecutorStub) Execute(ctx context.Context, _ int64, _, _ string, options DegradationProbeExecOptions) DegradationProbeExecResult {
	s.options = append(s.options, options)
	if s.started != nil {
		s.once.Do(func() { s.started <- ctx })
		select {
		case <-s.release:
		case <-ctx.Done():
			return DegradationProbeExecResult{Err: ctx.Err()}
		}
	}
	return DegradationProbeExecResult{AnswerText: truncateDegradationRunes(s.answer, 4000), fullAnswer: s.answer, AccountID: 7}
}

func TestDegradationVisualRunIntervalsAndForcedExecution(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(map[bool]string{false: "scheduled skips", true: "force runs"}[force], func(t *testing.T) {
			repo := &degradationRunnerRepoStub{
				target: &DegradationTarget{ID: 7, GroupID: 3, Model: "model"},
				probes: []*DegradationProbe{{ID: 1, Name: "visual", ProbeType: DegradationProbeTypeVisual, RunIntervalSeconds: 86400}},
				latest: map[int64]time.Time{1: time.Now().Add(-time.Hour)},
			}
			svc := NewDegradationService(repo, nil, nil, nil, nil)
			executor := &degradationExecutorStub{answer: "```html\n<p>work</p>\n```"}
			svc.executor = executor
			runs, err := svc.RunTargetOnce(context.Background(), 7, force)
			require.NoError(t, err)
			require.Equal(t, []int64{7}, repo.marked, "even all-skipped targets must advance")
			if force {
				require.Len(t, runs, 1)
				require.Equal(t, DegradationRunRendered, runs[0].Status)
				require.Equal(t, DegradationArtifactHTML, runs[0].ArtifactKind)
				require.Equal(t, "<p>work</p>", strings.TrimSpace(runs[0].ArtifactContent))
				require.Equal(t, int64(len(runs[0].ArtifactContent)), runs[0].ArtifactBytes)
				require.True(t, executor.options[0].Stream)
				require.Equal(t, 300*time.Second, executor.options[0].Timeout)
				require.Equal(t, 8192, executor.options[0].MaxOutputTokens)
			} else {
				require.Empty(t, runs)
				require.Empty(t, executor.options)
			}
		})
	}
}

func TestDegradationVisualOverdueProbeAndArtifactLimit(t *testing.T) {
	for _, oversized := range []bool{false, true} {
		repo := &degradationRunnerRepoStub{
			target: &DegradationTarget{ID: 7, GroupID: 3, Model: "model"},
			probes: []*DegradationProbe{{ID: 1, Name: "visual", ProbeType: DegradationProbeTypeVisual, RunIntervalSeconds: 86400, TimeoutSeconds: 600, MaxOutputTokens: 12000}},
			latest: map[int64]time.Time{1: time.Now().Add(-25 * time.Hour)},
		}
		svc := NewDegradationService(repo, nil, nil, nil, nil)
		answer := "I can only describe the chair."
		if oversized {
			answer = strings.Repeat("a", 512*1024+1)
		}
		executor := &degradationExecutorStub{answer: answer}
		svc.executor = executor
		runs, err := svc.RunTargetOnce(context.Background(), 7, false)
		require.NoError(t, err)
		require.Len(t, runs, 1)
		require.Equal(t, DegradationProbeTypeVisual, runs[0].ProbeType)
		require.Equal(t, 600*time.Second, executor.options[0].Timeout)
		require.Equal(t, 12000, executor.options[0].MaxOutputTokens)
		if oversized {
			require.Equal(t, DegradationRunError, runs[0].Status)
			require.Empty(t, runs[0].ArtifactContent)
			require.NotEmpty(t, runs[0].ErrorMessage)
			require.Len(t, []rune(runs[0].AnswerText), 4000)
		} else {
			require.Equal(t, DegradationRunRendered, runs[0].Status)
			require.Equal(t, DegradationArtifactText, runs[0].ArtifactKind)
		}
		encoded, err := json.Marshal(runs[0])
		require.NoError(t, err)
		require.NotContains(t, string(encoded), "artifact_content")
	}
}

func TestDegradationVisualRunsExcludedFromVerdict(t *testing.T) {
	now := time.Now()
	runs := []*DegradationRun{
		{ProbeType: DegradationProbeTypeRegex, Status: DegradationRunPassed, LatencyMs: 100, CreatedAt: now.Add(-time.Hour)},
		{ProbeType: DegradationProbeTypeRegex, Status: DegradationRunPassed, LatencyMs: 100, CreatedAt: now.Add(-time.Hour)},
		{ProbeType: DegradationProbeTypeRegex, Status: DegradationRunPassed, LatencyMs: 100, CreatedAt: now.Add(-time.Hour)},
		{ProbeType: DegradationProbeTypeVisual, Status: DegradationRunRendered, LatencyMs: 90000, CreatedAt: now},
		{ProbeType: DegradationProbeTypeVisual, Status: DegradationRunError, LatencyMs: 90000, CreatedAt: now},
	}
	model := summarizeDegradationModel("model", runs, 0.8)
	require.Equal(t, int64(3), model.Total)
	require.Zero(t, model.Error)
	require.Equal(t, DegradationVerdictHealthy, model.Verdict)
	require.Equal(t, DegradationRunPassed, model.LastRunStatus)
	require.Equal(t, int64(100), model.AvgLatencyMs)
	require.Equal(t, &now, model.LastCheckedAt)
	pureVisual := summarizeDegradationModel("model", runs[3:], 0.8)
	require.Zero(t, pureVisual.Total)
	require.Empty(t, pureVisual.LastRunStatus)
	require.Equal(t, DegradationVerdictUnknown, pureVisual.Verdict)
	require.Equal(t, &now, pureVisual.LastCheckedAt)
}

func TestDegradationManualTriggerAsyncConflictAndLifecycle(t *testing.T) {
	repo := &degradationRunnerRepoStub{
		target: &DegradationTarget{ID: 7, GroupID: 3, Model: "model"},
		probes: []*DegradationProbe{{ID: 1, Name: "visual", ProbeType: DegradationProbeTypeVisual}},
	}
	svc := NewDegradationService(repo, nil, nil, nil, nil)
	executor := &degradationExecutorStub{answer: "work", started: make(chan context.Context, 1), release: make(chan struct{})}
	svc.executor = executor
	runner := NewDegradationRunner(svc, nil)
	defer runner.Stop()
	// Saturating the scheduled pool must not delay a manual trigger.
	for range 4 {
		runner.pool.Submit(func() { <-executor.release })
	}
	defer close(executor.release)
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	result, err := runner.Trigger(requestCtx, 7)
	require.NoError(t, err)
	require.True(t, result.Accepted)
	require.Equal(t, 1, result.ProbeCount)
	require.Equal(t, []int64{7}, repo.marked, "mark execution before returning accepted")
	cancelRequest()
	select {
	case workCtx := <-executor.started:
		require.NoError(t, workCtx.Err(), "request cancellation cannot cancel accepted work")
	case <-time.After(time.Second):
		t.Fatal("manual work queued behind scheduled pool")
	}
	_, err = runner.Trigger(context.Background(), 7)
	require.ErrorIs(t, err, ErrDegradationTargetRunning)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	runner.cancel()
	runner.manualPool.StopAndWait()
	require.Empty(t, runner.inFlight)
}

type degradationShowcaseRepoStub struct {
	degradationServiceRepoStub
	visualRuns []*DegradationRun
	metaRun    *DegradationRun
}

func (r *degradationShowcaseRepoStub) ListLatestVisualRuns(context.Context, []int64) ([]*DegradationRun, error) {
	return r.visualRuns, nil
}
func (r *degradationShowcaseRepoStub) GetRun(context.Context, int64) (*DegradationRun, error) {
	return r.metaRun, nil
}

func TestDegradationOverviewShowcasesCurrentProbesAndOrder(t *testing.T) {
	repo := &degradationShowcaseRepoStub{
		degradationServiceRepoStub: degradationServiceRepoStub{
			targets: []*DegradationTarget{{ID: 1, GroupID: 12, Model: "model", Enabled: true, ProbeIDs: []int64{3, 1, 2}}},
			probes: []*DegradationProbe{
				{ID: 1, ProbeType: DegradationProbeTypeVisual, Enabled: true, SortOrder: 2},
				{ID: 2, ProbeType: DegradationProbeTypeVisual, Enabled: false},
				{ID: 3, ProbeType: DegradationProbeTypeVisual, Enabled: true, SortOrder: 1},
				{ID: 4, ProbeType: DegradationProbeTypeVisual, Enabled: true},
				{ID: 5, ProbeType: DegradationProbeTypeRegex, Enabled: true},
			},
		},
		visualRuns: []*DegradationRun{
			{ID: 101, GroupID: 12, Model: "model", ProbeID: 1},
			{ID: 102, GroupID: 12, Model: "model", ProbeID: 2},
			{ID: 103, GroupID: 12, Model: "model", ProbeID: 3},
			{ID: 104, GroupID: 12, Model: "model", ProbeID: 4},
			{ID: 105, GroupID: 12, Model: "model", ProbeID: 5},
		},
	}
	got, err := NewDegradationService(repo, nil, nil, nil, nil).GetOverview(context.Background(), []int64{12}, 1)
	require.NoError(t, err)
	model := got.Groups[0].Models[0]
	require.Zero(t, model.Total)
	require.Equal(t, []int64{103, 101}, []int64{model.Showcases[0].RunID, model.Showcases[1].RunID})
	require.Len(t, model.Showcases, 2)
}

func TestDegradationVisualTokenRequiresVisibleEnabledModel(t *testing.T) {
	for _, tc := range []struct {
		name           string
		groups         []int64
		enabled, admin bool
		want           bool
	}{
		{name: "visible", groups: []int64{12}, enabled: true, want: true},
		{name: "foreign group", groups: []int64{13}, enabled: true},
		{name: "disabled target", groups: []int64{12}},
		{name: "admin can inspect disabled", admin: true, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &degradationShowcaseRepoStub{
				degradationServiceRepoStub: degradationServiceRepoStub{targets: []*DegradationTarget{{GroupID: 12, Model: "model", Enabled: tc.enabled}}},
				metaRun:                    &DegradationRun{ID: 1, GroupID: 12, Model: "model", ProbeType: DegradationProbeTypeVisual, Status: DegradationRunRendered},
			}
			_, err := NewDegradationService(repo, nil, nil, nil, nil).GetVisualRunForToken(context.Background(), 1, tc.groups, tc.admin)
			if tc.want {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrDegradationRunNotFound)
			}
		})
	}
}
