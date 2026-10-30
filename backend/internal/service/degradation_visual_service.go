package service

import (
	"context"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func degradationShowcase(run *DegradationRun) DegradationShowcase {
	return DegradationShowcase{RunID: run.ID, ProbeID: run.ProbeID, ProbeName: run.ProbeName, ArtifactKind: run.ArtifactKind, LatencyMs: run.LatencyMs, CreatedAt: run.CreatedAt}
}

func (s *DegradationService) GetVisualArtifact(ctx context.Context, id int64) (*DegradationRun, error) {
	run, err := s.repo.GetRunArtifact(ctx, id)
	if err != nil {
		return nil, err
	}
	if run.ProbeType != DegradationProbeTypeVisual || run.Status != DegradationRunRendered {
		return nil, ErrDegradationRunNotFound
	}
	return run, nil
}

func (s *DegradationService) visualModelVisible(ctx context.Context, viewerGroupIDs []int64, groupID int64, model string) (bool, error) {
	if !containsInt64(viewerGroupIDs, groupID) {
		return false, nil
	}
	targets, err := s.repo.ListTargets(ctx, &groupID, true)
	if err != nil {
		return false, err
	}
	for _, target := range targets {
		if target.Model == model && target.Enabled {
			return true, nil
		}
	}
	return false, nil
}

func (s *DegradationService) GetVisualRunForToken(ctx context.Context, id int64, viewerGroupIDs []int64, admin bool) (*DegradationRun, error) {
	run, err := s.repo.GetRun(ctx, id)
	if err != nil {
		return nil, err
	}
	if run.ProbeType != DegradationProbeTypeVisual || run.Status != DegradationRunRendered {
		return nil, ErrDegradationRunNotFound
	}
	if !admin {
		if s.settings != nil && !s.settings.GetDegradationRuntime(ctx).Enabled {
			return nil, ErrDegradationRunNotFound
		}
		visible, err := s.visualModelVisible(ctx, viewerGroupIDs, run.GroupID, run.Model)
		if err != nil {
			return nil, err
		}
		if !visible {
			return nil, ErrDegradationRunNotFound
		}
	}
	return run, nil
}

func (s *DegradationService) ListVisualRuns(ctx context.Context, viewerGroupIDs []int64, groupID int64, model string, probeID int64, limit int) ([]DegradationShowcase, error) {
	out := make([]DegradationShowcase, 0)
	model = strings.TrimSpace(model)
	if groupID <= 0 || probeID <= 0 || model == "" {
		return nil, infraerrors.BadRequest("DEGRADATION_INVALID_VISUAL_QUERY", "group_id, model and probe_id are required")
	}
	visible, err := s.visualModelVisible(ctx, viewerGroupIDs, groupID, model)
	if err != nil || !visible {
		return out, err
	}
	if limit <= 0 {
		limit = 20
	} else if limit > 50 {
		limit = 50
	}
	runs, err := s.repo.ListVisualRuns(ctx, groupID, model, probeID, limit)
	if err != nil {
		return nil, err
	}
	for _, run := range runs {
		out = append(out, degradationShowcase(run))
	}
	return out, nil
}
