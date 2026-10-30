//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type degradationRunnerRepoStub struct {
	DegradationRepository
	ctxCancel context.CancelFunc
	target    *DegradationTarget
	probes    []*DegradationProbe
	runs      []*DegradationRun
	marked    []int64
	dueCalls  int
	getCalls  int
	cleaned   int
	latest    map[int64]time.Time
}

func (s *degradationRunnerRepoStub) DeleteRunsBefore(context.Context, time.Time) (int64, error) {
	s.cleaned++
	return 0, nil
}

func (s *degradationRunnerRepoStub) ListDueTargets(context.Context, time.Time) ([]*DegradationTarget, error) {
	s.dueCalls++
	if s.ctxCancel != nil {
		s.ctxCancel()
	}
	return []*DegradationTarget{s.target}, nil
}

func (s *degradationRunnerRepoStub) GetTarget(context.Context, int64) (*DegradationTarget, error) {
	s.getCalls++
	return s.target, nil
}

func (s *degradationRunnerRepoStub) ListProbes(context.Context, bool) ([]*DegradationProbe, error) {
	return s.probes, nil
}

func (s *degradationRunnerRepoStub) LatestProbeRuns(context.Context, int64, []int64) (map[int64]time.Time, error) {
	return s.latest, nil
}

func (s *degradationRunnerRepoStub) CreateRun(ctx context.Context, run *DegradationRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.runs = append(s.runs, run)
	if s.ctxCancel != nil {
		s.ctxCancel()
	}
	return nil
}

func (s *degradationRunnerRepoStub) UpdateTargetLastRun(ctx context.Context, id int64, _ time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.marked = append(s.marked, id)
	return nil
}

type degradationLockStub struct {
	acquired bool
	err      error
	key      string
	ttl      time.Duration
	released int
}

func (s *degradationLockStub) TryAcquireLeaderLock(_ context.Context, key, _ string, ttl time.Duration) (bool, error) {
	s.key, s.ttl = key, ttl
	return s.acquired, s.err
}

func (s *degradationLockStub) ReleaseLeaderLock(context.Context, string, string) error {
	s.released++
	return nil
}

func TestDegradationRunnerLeaderLock(t *testing.T) {
	for _, tc := range []struct {
		name string
		lock *degradationLockStub
		want bool
	}{
		{name: "no redis runs locally", want: true},
		{name: "owner", lock: &degradationLockStub{acquired: true}, want: true},
		{name: "peer holds lock", lock: &degradationLockStub{}},
		{name: "redis fails closed", lock: &degradationLockStub{err: errors.New("redis unavailable")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var locks LeaderLockCache
			if tc.lock != nil {
				locks = tc.lock
			}
			runner := NewDegradationRunner(nil, locks)
			defer runner.Stop()
			release, acquired := runner.tryAcquireLeaderLock(context.Background())
			require.Equal(t, tc.want, acquired)
			if tc.lock != nil {
				require.Equal(t, degradationLeaderKey, tc.lock.key)
				require.Equal(t, 2*time.Minute, tc.lock.ttl)
			}
			if acquired {
				release()
				if tc.lock != nil {
					require.Equal(t, 1, tc.lock.released)
				}
			}
		})
	}
}

func TestDegradationRunnerCancellationDoesNotMarkUnstartedTargets(t *testing.T) {
	repo := &degradationRunnerRepoStub{target: &DegradationTarget{ID: 7}}
	runner := NewDegradationRunner(NewDegradationService(repo, nil, nil, nil, nil), nil)
	defer runner.Stop()
	repo.ctxCancel = runner.cancel
	runner.runCycle()
	require.Equal(t, 1, repo.dueCalls)
	require.Zero(t, repo.getCalls)
	require.Empty(t, repo.marked)
	require.Empty(t, runner.inFlight)
}

func TestDegradationRunnerCleanupThrottled(t *testing.T) {
	repo := &degradationRunnerRepoStub{target: &DegradationTarget{ID: 7}}
	lock := &degradationLockStub{acquired: true}
	runner := NewDegradationRunner(NewDegradationService(repo, nil, nil, nil, nil), lock)
	defer runner.Stop()
	// Busy targets are skipped without changing their execution timestamp.
	runner.inFlight[7] = struct{}{}
	runner.runCycle()
	runner.runCycle()
	require.Equal(t, 1, repo.cleaned)
	require.Equal(t, 2, repo.dueCalls)
	require.Equal(t, 2, lock.released)
	require.Zero(t, repo.getCalls)
	require.Empty(t, repo.marked)
}

func TestDegradationRunPartialCancellationMarksStartedTarget(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := &degradationRunnerRepoStub{
		ctxCancel: cancel,
		target:    &DegradationTarget{ID: 7, GroupID: 3, Model: "model"},
		probes: []*DegradationProbe{
			{ID: 1, Name: "invalid legacy regex", ExpectRegex: "["},
			{ID: 2, Name: "never started", ExpectRegex: "42"},
		},
	}
	svc := NewDegradationService(repo, nil, nil, nil, nil)
	runs, err := svc.RunTargetOnce(ctx, 7, false)
	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, runs, 1)
	require.Len(t, repo.runs, 1)
	require.Equal(t, DegradationRunError, runs[0].Status)
	require.Contains(t, runs[0].ErrorMessage, "error parsing regexp")
	require.Equal(t, []int64{7}, repo.marked)
}
