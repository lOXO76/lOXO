package service

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/alitto/pond/v2"
	"github.com/google/uuid"
)

const degradationLeaderKey = "degradation:runner:leader"

type degradationLeaderLockRenewer interface {
	RenewLeaderLock(context.Context, string, string, time.Duration) (bool, error)
}

type DegradationRunner struct {
	svc        *DegradationService
	locks      LeaderLockCache
	owner      string
	pool       pond.Pool
	manualPool pond.Pool
	ctx        context.Context
	cancel     context.CancelFunc
	startOnce  sync.Once
	stopOnce   sync.Once
	wg         sync.WaitGroup
	mu         sync.Mutex
	inFlight   map[int64]struct{}
	lastClean  time.Time
	noLockLog  sync.Once
}

func NewDegradationRunner(svc *DegradationService, locks LeaderLockCache) *DegradationRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &DegradationRunner{svc: svc, locks: locks, owner: uuid.NewString(), pool: pond.NewPool(4), manualPool: pond.NewPool(2), ctx: ctx, cancel: cancel, inFlight: map[int64]struct{}{}}
}

func (r *DegradationRunner) Start() {
	if r == nil || r.svc == nil {
		return
	}
	r.startOnce.Do(func() {
		r.wg.Add(1)
		go func() {
			defer r.wg.Done()
			// One ticker per process keeps the scan cadence independent of timezone.
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-r.ctx.Done():
					return
				case <-ticker.C:
					r.runCycle()
				}
			}
		}()
	})
}

func (r *DegradationRunner) Stop() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() {
		r.cancel()
		r.wg.Wait()
		r.pool.StopAndWait()
		r.manualPool.StopAndWait()
	})
}

// Trigger keeps manual work independent of the scheduled queue while sharing
// the same per-target exclusion and shutdown cancellation.
func (r *DegradationRunner) Trigger(ctx context.Context, id int64) (*DegradationRunTriggerResult, error) {
	if r == nil || r.svc == nil || r.ctx.Err() != nil {
		return nil, infraerrors.ServiceUnavailable("DEGRADATION_UNAVAILABLE", "degradation runner is unavailable")
	}
	r.mu.Lock()
	_, busy := r.inFlight[id]
	if !busy {
		r.inFlight[id] = struct{}{}
	}
	r.mu.Unlock()
	if busy {
		return nil, ErrDegradationTargetRunning
	}
	accepted := false
	defer func() {
		if !accepted {
			r.releaseInFlight(id)
		}
	}()
	target, err := r.svc.repo.GetTarget(ctx, id)
	if err != nil {
		return nil, err
	}
	probes, err := r.svc.probesForTarget(ctx, target)
	if err != nil {
		return nil, err
	}
	if len(probes) == 0 {
		return nil, ErrDegradationNoProbes
	}
	startedAt := time.Now().UTC()
	markCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = r.svc.repo.UpdateTargetLastRun(markCtx, id, startedAt)
	cancel()
	if err != nil {
		return nil, err
	}
	_, submitted := r.manualPool.TrySubmit(func() {
		defer r.releaseInFlight(id)
		if _, err := r.svc.RunTargetOnce(r.ctx, id, true); err != nil {
			slog.Warn("degradation: manual target run failed", "target_id", id, "error", err)
		}
	})
	if !submitted {
		return nil, infraerrors.ServiceUnavailable("DEGRADATION_UNAVAILABLE", "degradation runner is stopping")
	}
	accepted = true
	return &DegradationRunTriggerResult{Accepted: true, ProbeCount: len(probes), StartedAt: startedAt}, nil
}

func (r *DegradationRunner) runCycle() {
	ctx, cancel := context.WithCancel(r.ctx)
	defer cancel()
	release, acquired := r.tryAcquireLeaderLock(ctx)
	if !acquired {
		return
	}
	defer release()
	stopHeartbeat := r.maintainLease(ctx, cancel)
	defer stopHeartbeat()
	now := time.Now()
	if r.lastClean.IsZero() || now.Sub(r.lastClean) >= 24*time.Hour {
		if _, err := r.svc.repo.DeleteRunsBefore(ctx, now.Add(-30*24*time.Hour)); err != nil {
			slog.Warn("degradation: retention cleanup failed", "error", err)
		} else {
			r.lastClean = now
		}
	}
	targets, err := r.svc.repo.ListDueTargets(ctx, now)
	if err != nil {
		slog.Warn("degradation: list due targets failed", "error", err)
		return
	}
	var pending sync.WaitGroup
	for _, target := range targets {
		if ctx.Err() != nil {
			break
		}
		r.mu.Lock()
		_, busy := r.inFlight[target.ID]
		if !busy {
			r.inFlight[target.ID] = struct{}{}
		}
		r.mu.Unlock()
		if busy {
			continue
		}
		pending.Add(1)
		_, submitted := r.pool.TrySubmit(func() {
			defer pending.Done()
			defer r.releaseInFlight(target.ID)
			if ctx.Err() != nil {
				return
			}
			if _, err := r.svc.RunTargetOnce(ctx, target.ID, false); err != nil {
				slog.Warn("degradation: target run failed", "target_id", target.ID, "error", err)
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				markCtx, markCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer markCancel()
				if updateErr := r.svc.repo.UpdateTargetLastRun(markCtx, target.ID, time.Now()); updateErr != nil {
					slog.Warn("degradation: last run update failed", "target_id", target.ID, "error", updateErr)
				}
			}
		})
		if !submitted {
			pending.Done()
			r.releaseInFlight(target.ID)
		}
	}
	pending.Wait()
}

func (r *DegradationRunner) maintainLease(ctx context.Context, cancel context.CancelFunc) func() {
	renewer, supported := r.locks.(degradationLeaderLockRenewer)
	if !supported {
		if r.locks == nil {
			return func() {}
		}
		// Alternate lock implementations without renewal must finish before TTL.
		timer := time.AfterFunc(110*time.Second, cancel)
		return func() { timer.Stop() }
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(40 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				lockCtx, lockCancel := context.WithTimeout(ctx, 5*time.Second)
				ok, err := renewer.RenewLeaderLock(lockCtx, degradationLeaderKey, r.owner, 2*time.Minute)
				lockCancel()
				if err != nil || !ok {
					slog.Warn("degradation: leader lease lost; cancelling cycle", "error", err)
					cancel()
					return
				}
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}

func (r *DegradationRunner) releaseInFlight(id int64) {
	r.mu.Lock()
	delete(r.inFlight, id)
	r.mu.Unlock()
}

func (r *DegradationRunner) tryAcquireLeaderLock(ctx context.Context) (func(), bool) {
	if r.locks == nil {
		r.noLockLog.Do(func() { slog.Warn("degradation: redis not configured; running without distributed lock") })
		return func() {}, true
	}
	ok, err := r.locks.TryAcquireLeaderLock(ctx, degradationLeaderKey, r.owner, 2*time.Minute)
	if err != nil {
		// Mirror OpsScheduledReportService: no Redis runs locally; a configured
		// but failing Redis fails closed to avoid duplicate upstream requests.
		slog.Warn("degradation: leader lock unavailable; skipping cycle", "error", err)
		return nil, false
	}
	if !ok {
		return nil, false
	}
	return func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := r.locks.ReleaseLeaderLock(releaseCtx, degradationLeaderKey, r.owner); err != nil {
			slog.Warn("degradation: leader lock release failed", "error", err)
		}
	}, true
}
