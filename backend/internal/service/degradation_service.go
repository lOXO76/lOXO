package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type DegradationService struct {
	repo      DegradationRepository
	groupRepo GroupRepository
	gateway   *GatewayService
	settings  *SettingService
	executor  degradationExecutor
}

type degradationExecutor interface {
	Execute(context.Context, int64, string, string, DegradationProbeExecOptions) DegradationProbeExecResult
}

func NewDegradationService(repo DegradationRepository, groupRepo GroupRepository, gateway *GatewayService, accountTests *AccountTestService, settings *SettingService) *DegradationService {
	executor := NewDegradationProbeExecutor(gateway, accountTests)
	return &DegradationService{repo: repo, groupRepo: groupRepo, gateway: gateway, settings: settings, executor: executor}
}

func (s *DegradationService) ListProbes(ctx context.Context, includeDisabled bool) ([]*DegradationProbe, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.InternalServer("DEGRADATION_UNAVAILABLE", "degradation service is unavailable")
	}
	return s.repo.ListProbes(ctx, includeDisabled)
}

func (s *DegradationService) CreateProbe(ctx context.Context, params DegradationProbeCreateParams) (*DegradationProbe, error) {
	params = normalizeDegradationProbeParams(params)
	if err := validateDegradationProbe(params); err != nil {
		return nil, infraerrors.BadRequest("DEGRADATION_INVALID_REGEX", err.Error())
	}
	probe := &DegradationProbe{Name: params.Name, Prompt: params.Prompt, ExpectRegex: params.ExpectRegex, RegexFlags: params.RegexFlags, Negate: params.Negate, Enabled: true, SortOrder: params.SortOrder,
		ProbeType: params.ProbeType, MaxOutputTokens: params.MaxOutputTokens, TimeoutSeconds: params.TimeoutSeconds, RunIntervalSeconds: params.RunIntervalSeconds}
	if params.Enabled != nil {
		probe.Enabled = *params.Enabled
	}
	if err := s.repo.CreateProbe(ctx, probe); err != nil {
		return nil, err
	}
	return probe, nil
}

func (s *DegradationService) UpdateProbe(ctx context.Context, id int64, params DegradationProbeCreateParams) (*DegradationProbe, error) {
	probe, err := s.repo.GetProbe(ctx, id)
	if err != nil {
		return nil, err
	}
	params = normalizeDegradationProbeParams(params)
	if err := validateDegradationProbe(params); err != nil {
		return nil, infraerrors.BadRequest("DEGRADATION_INVALID_REGEX", err.Error())
	}
	probe.Name, probe.Prompt, probe.ExpectRegex, probe.RegexFlags = params.Name, params.Prompt, params.ExpectRegex, params.RegexFlags
	probe.Negate, probe.SortOrder = params.Negate, params.SortOrder
	probe.ProbeType, probe.MaxOutputTokens, probe.TimeoutSeconds, probe.RunIntervalSeconds = params.ProbeType, params.MaxOutputTokens, params.TimeoutSeconds, params.RunIntervalSeconds
	if params.Enabled != nil {
		probe.Enabled = *params.Enabled
	}
	if err := s.repo.UpdateProbe(ctx, probe); err != nil {
		return nil, err
	}
	return s.repo.GetProbe(ctx, id)
}

func (s *DegradationService) DeleteProbe(ctx context.Context, id int64) error {
	return s.repo.DeleteProbe(ctx, id)
}

func (s *DegradationService) TestRegex(_ context.Context, pattern, flags string, negate bool, sample string) (bool, bool, error) {
	re, err := compileProbeRegex(pattern, flags)
	if err != nil {
		return false, false, infraerrors.BadRequest("DEGRADATION_INVALID_REGEX", err.Error())
	}
	matched := re.MatchString(sample)
	return matched, judgeProbeAnswer(re, negate, sample), nil
}

func (s *DegradationService) ListTargets(ctx context.Context, groupID *int64, includeDisabled bool) ([]*DegradationTarget, error) {
	targets, err := s.repo.ListTargets(ctx, groupID, !includeDisabled)
	if err != nil {
		return nil, err
	}
	return s.enrichTargets(ctx, targets), nil
}

func (s *DegradationService) CreateTarget(ctx context.Context, params DegradationTargetParams) (*DegradationTarget, error) {
	if err := validateTargetParams(params); err != nil {
		return nil, err
	}
	if s.groupRepo != nil {
		if _, err := s.groupRepo.GetByIDLite(ctx, params.GroupID); err != nil {
			return nil, err
		}
	}
	probeIDs, err := s.resolveTargetProbeIDs(ctx, params.ProbeIDs)
	if err != nil {
		return nil, err
	}
	interval := params.IntervalSeconds
	if interval == 0 {
		interval = s.settings.GetDegradationRuntime(ctx).DefaultIntervalSeconds
	}
	if interval < degradationMinIntervalSeconds || interval > degradationMaxIntervalSeconds {
		return nil, infraerrors.BadRequest("DEGRADATION_INVALID_INTERVAL", "interval_seconds must be between 300 and 86400")
	}
	target := &DegradationTarget{GroupID: params.GroupID, Model: strings.TrimSpace(params.Model), ProbeIDs: probeIDs, IntervalSeconds: interval, Enabled: true}
	if params.Enabled != nil {
		target.Enabled = *params.Enabled
	}
	if err := s.repo.CreateTarget(ctx, target); err != nil {
		return nil, err
	}
	return s.enrichTarget(ctx, target), nil
}

func (s *DegradationService) UpdateTarget(ctx context.Context, id int64, params DegradationTargetParams) (*DegradationTarget, error) {
	target, err := s.repo.GetTarget(ctx, id)
	if err != nil {
		return nil, err
	}
	if params.GroupID == 0 {
		params.GroupID = target.GroupID
	}
	if params.Model == "" {
		params.Model = target.Model
	}
	if params.IntervalSeconds == 0 {
		params.IntervalSeconds = target.IntervalSeconds
	}
	if params.ProbeIDs == nil {
		params.ProbeIDs = target.ProbeIDs
	}
	if err := validateTargetParams(params); err != nil {
		return nil, err
	}
	if s.groupRepo != nil {
		if _, err := s.groupRepo.GetByIDLite(ctx, params.GroupID); err != nil {
			return nil, err
		}
	}
	probeIDs, err := s.resolveTargetProbeIDs(ctx, params.ProbeIDs)
	if err != nil {
		return nil, err
	}
	target.GroupID, target.Model, target.ProbeIDs, target.IntervalSeconds = params.GroupID, strings.TrimSpace(params.Model), probeIDs, params.IntervalSeconds
	if params.Enabled != nil {
		target.Enabled = *params.Enabled
	}
	if err := s.repo.UpdateTarget(ctx, target); err != nil {
		return nil, err
	}
	return s.enrichTarget(ctx, target), nil
}

func (s *DegradationService) DeleteTarget(ctx context.Context, id int64) error {
	return s.repo.DeleteTarget(ctx, id)
}

func validateTargetParams(params DegradationTargetParams) error {
	if params.GroupID <= 0 {
		return infraerrors.BadRequest("DEGRADATION_INVALID_GROUP", "group_id must be positive")
	}
	if strings.TrimSpace(params.Model) == "" || len([]rune(params.Model)) > 200 {
		return infraerrors.BadRequest("DEGRADATION_INVALID_MODEL", "model must be non-empty and at most 200 characters")
	}
	if params.IntervalSeconds != 0 && (params.IntervalSeconds < degradationMinIntervalSeconds || params.IntervalSeconds > degradationMaxIntervalSeconds) {
		return infraerrors.BadRequest("DEGRADATION_INVALID_INTERVAL", "interval_seconds must be between 300 and 86400")
	}
	return nil
}

func (s *DegradationService) resolveTargetProbeIDs(ctx context.Context, ids []int64) ([]int64, error) {
	for _, id := range ids {
		if id <= 0 {
			return nil, ErrDegradationNoProbes
		}
	}
	if len(ids) == 0 {
		probes, err := s.repo.ListProbes(ctx, false)
		if err != nil {
			return nil, err
		}
		if len(probes) == 0 {
			return nil, ErrDegradationNoProbes
		}
		return []int64{}, nil
	}
	probes, err := s.repo.ListProbesByIDs(ctx, ids, true)
	if err != nil {
		return nil, err
	}
	uniqueIDs := uniqueInt64s(ids)
	if len(uniqueIDs) == 0 || len(probes) != len(uniqueIDs) {
		return nil, ErrDegradationNoProbes
	}
	return uniqueInt64s(ids), nil
}

func uniqueInt64s(values []int64) []int64 {
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; !ok {
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	return out
}

func (s *DegradationService) RunTargetOnce(ctx context.Context, id int64, force bool) (out []*DegradationRun, runErr error) {
	target, err := s.repo.GetTarget(ctx, id)
	if err != nil {
		return nil, err
	}
	probes, err := s.probesForTarget(ctx, target)
	if err != nil {
		return nil, err
	}
	if len(probes) == 0 {
		return nil, ErrDegradationNoProbes
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	latest := map[int64]time.Time{}
	if !force {
		ids := make([]int64, 0, len(probes))
		for _, probe := range probes {
			if probe.RunIntervalSeconds > 0 {
				ids = append(ids, probe.ID)
			}
		}
		if len(ids) > 0 {
			latest, err = s.repo.LatestProbeRuns(ctx, target.ID, ids)
			if err != nil {
				return nil, err
			}
		}
	}
	out = make([]*DegradationRun, 0, len(probes))
	startedRound := false
	defer func() {
		if !startedRound {
			return
		}
		// Even a round that skips all daily probes must advance the target.
		markCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := s.repo.UpdateTargetLastRun(markCtx, target.ID, time.Now()); err != nil {
			runErr = errors.Join(runErr, err)
		}
	}()
	for _, probe := range probes {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		startedRound = true
		if at := latest[probe.ID]; !force && probe.RunIntervalSeconds > 0 && !at.IsZero() && time.Now().Before(at.Add(time.Duration(probe.RunIntervalSeconds)*time.Second)) {
			continue
		}
		probeType := degradationProbeType(probe.ProbeType)
		run := &DegradationRun{TargetID: target.ID, GroupID: target.GroupID, Model: target.Model, ProbeID: probe.ID, ProbeName: probe.Name, ProbeType: probeType, Status: DegradationRunError}
		var re *regexp.Regexp
		var compileErr error
		if probeType == DegradationProbeTypeRegex {
			re, compileErr = compileProbeRegex(probe.ExpectRegex, probe.RegexFlags)
		}
		if compileErr != nil {
			run.ErrorMessage = truncateDegradationRunes(sanitizeErrorMessage(compileErr.Error()), degradationMaxErrorRunes)
		} else {
			execResult := s.executor.Execute(ctx, target.GroupID, target.Model, probe.Prompt, degradationExecOptions(probe))
			run.AnswerText, run.LatencyMs, run.AccountID = execResult.AnswerText, execResult.LatencyMs, execResult.AccountID
			if execResult.Err != nil {
				run.ErrorMessage = truncateDegradationRunes(sanitizeErrorMessage(execResult.Err.Error()), degradationMaxErrorRunes)
			} else if probeType == DegradationProbeTypeVisual {
				kind, content, artifactErr := extractDegradationArtifact(execResult.fullAnswer)
				if artifactErr != nil {
					run.ErrorMessage = truncateDegradationRunes(sanitizeErrorMessage(artifactErr.Error()), degradationMaxErrorRunes)
				} else {
					run.Status, run.ArtifactKind, run.ArtifactContent = DegradationRunRendered, kind, content
					run.ArtifactBytes = int64(len(content))
				}
			} else if judgeProbeAnswer(re, probe.Negate, execResult.fullAnswer) {
				run.Status = DegradationRunPassed
			} else {
				run.Status = DegradationRunFailed
			}
		}
		// Persist the terminal error of an already issued request even when its
		// caller disconnects or the runner loses its lease.
		persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err := s.repo.CreateRun(persistCtx, run)
		cancel()
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, nil
}

func (s *DegradationService) probesForTarget(ctx context.Context, target *DegradationTarget) ([]*DegradationProbe, error) {
	if len(target.ProbeIDs) == 0 {
		return s.repo.ListProbes(ctx, false)
	}
	return s.repo.ListProbesByIDs(ctx, target.ProbeIDs, true)
}

func (s *DegradationService) GetTargetRuns(ctx context.Context, targetID int64, limit int) ([]*DegradationRun, error) {
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}
	if _, err := s.repo.GetTarget(ctx, targetID); err != nil {
		return nil, err
	}
	return s.repo.ListRunsByTarget(ctx, targetID, limit)
}

func (s *DegradationService) GetRuns(ctx context.Context, groupIDs []int64, model string, limit int, admin bool) ([]*DegradationRun, error) {
	if len(groupIDs) == 0 {
		return []*DegradationRun{}, nil
	}
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}
	return s.repo.ListRuns(ctx, groupIDs, strings.TrimSpace(model), time.Now().Add(-30*24*time.Hour), limit, admin)
}

func (s *DegradationService) GetOverview(ctx context.Context, viewerGroupIDs []int64, windowHours int, adminView ...bool) (*DegradationOverview, error) {
	admin := len(adminView) > 0 && adminView[0]
	if windowHours < 1 || windowHours > 168 {
		windowHours = degradationDefaultWindowHours
	}
	runtime := DegradationRuntime{Enabled: true, Threshold: 0.8}
	if s.settings != nil {
		runtime = s.settings.GetDegradationRuntime(ctx)
	}
	if !runtime.Enabled && !admin {
		return &DegradationOverview{Enabled: false, WindowHours: windowHours, Threshold: runtime.Threshold, GeneratedAt: time.Now().UTC(), Groups: []DegradationGroupOverview{}}, nil
	}
	targets, err := s.repo.ListTargets(ctx, nil, !admin)
	if err != nil {
		return nil, err
	}
	allowed := make(map[int64]struct{}, len(viewerGroupIDs))
	for _, id := range viewerGroupIDs {
		allowed[id] = struct{}{}
	}
	if !admin && viewerGroupIDs != nil {
		filtered := targets[:0]
		for _, target := range targets {
			if _, ok := allowed[target.GroupID]; ok {
				filtered = append(filtered, target)
			}
		}
		targets = filtered
	}
	groupIDs := make([]int64, 0, len(targets))
	seenGroups := map[int64]struct{}{}
	for _, target := range targets {
		if _, ok := seenGroups[target.GroupID]; !ok {
			seenGroups[target.GroupID] = struct{}{}
			groupIDs = append(groupIDs, target.GroupID)
		}
	}
	if len(targets) == 0 {
		return &DegradationOverview{Enabled: runtime.Enabled, WindowHours: windowHours, Threshold: runtime.Threshold, GeneratedAt: time.Now().UTC(), Groups: []DegradationGroupOverview{}}, nil
	}
	runs, err := s.repo.ListRunsSince(ctx, time.Now().Add(-time.Duration(windowHours)*time.Hour), groupIDs)
	if err != nil {
		return nil, err
	}
	groupMap := make(map[int64]*Group)
	if s.groupRepo != nil {
		for _, id := range groupIDs {
			if group, groupErr := s.groupRepo.GetByIDLite(ctx, id); groupErr == nil && group != nil {
				groupMap[id] = group
			}
		}
	}
	type modelKey struct {
		groupID int64
		model   string
	}
	byModel := make(map[modelKey][]*DegradationRun)
	for _, run := range runs {
		key := modelKey{run.GroupID, run.Model}
		byModel[key] = append(byModel[key], run)
	}
	probes, err := s.repo.ListProbes(ctx, false)
	if err != nil {
		return nil, err
	}
	visualProbes := make([]*DegradationProbe, 0)
	for _, probe := range probes {
		if probe.Enabled && degradationProbeType(probe.ProbeType) == DegradationProbeTypeVisual {
			visualProbes = append(visualProbes, probe)
		}
	}
	sort.Slice(visualProbes, func(i, j int) bool {
		if visualProbes[i].SortOrder != visualProbes[j].SortOrder {
			return visualProbes[i].SortOrder < visualProbes[j].SortOrder
		}
		return visualProbes[i].ID < visualProbes[j].ID
	})
	type showcaseKey struct {
		modelKey
		probeID int64
	}
	latestVisual := make(map[showcaseKey]*DegradationRun)
	if len(visualProbes) > 0 {
		visualRuns, err := s.repo.ListLatestVisualRuns(ctx, groupIDs)
		if err != nil {
			return nil, err
		}
		for _, run := range visualRuns {
			key := showcaseKey{modelKey{run.GroupID, run.Model}, run.ProbeID}
			if previous := latestVisual[key]; previous == nil || run.CreatedAt.After(previous.CreatedAt) || run.CreatedAt.Equal(previous.CreatedAt) && run.ID > previous.ID {
				latestVisual[key] = run
			}
		}
	}
	groupModels := make(map[int64][]DegradationModelOverview)
	for _, target := range targets {
		items := byModel[modelKey{target.GroupID, target.Model}]
		model := summarizeDegradationModel(target.Model, items, runtime.Threshold)
		for _, probe := range visualProbes {
			if len(target.ProbeIDs) > 0 && !containsInt64(target.ProbeIDs, probe.ID) {
				continue
			}
			if run := latestVisual[showcaseKey{modelKey{target.GroupID, target.Model}, probe.ID}]; run != nil {
				model.Showcases = append(model.Showcases, degradationShowcase(run))
			}
		}
		groupModels[target.GroupID] = append(groupModels[target.GroupID], model)
	}
	out := &DegradationOverview{Enabled: runtime.Enabled, WindowHours: windowHours, Threshold: runtime.Threshold, GeneratedAt: time.Now().UTC(), Groups: []DegradationGroupOverview{}}
	for groupID, models := range groupModels {
		sort.Slice(models, func(i, j int) bool { return models[i].Model < models[j].Model })
		group := DegradationGroupOverview{GroupID: groupID, Models: models, Verdict: groupVerdict(models)}
		if info := groupMap[groupID]; info != nil {
			group.GroupName, group.Platform = info.Name, info.Platform
		} else {
			group.GroupName = fmt.Sprintf("Group %d", groupID)
		}
		for _, model := range models {
			if model.LastCheckedAt != nil && (group.LastCheckedAt == nil || model.LastCheckedAt.After(*group.LastCheckedAt)) {
				at := *model.LastCheckedAt
				group.LastCheckedAt = &at
			}
		}
		out.Groups = append(out.Groups, group)
	}
	sort.Slice(out.Groups, func(i, j int) bool { return out.Groups[i].GroupID < out.Groups[j].GroupID })
	return out, nil
}

func summarizeDegradationModel(model string, runs []*DegradationRun, threshold float64) DegradationModelOverview {
	item := DegradationModelOverview{Model: model, Verdict: DegradationVerdictUnknown, Showcases: []DegradationShowcase{}}
	var latencyTotal int64
	var lastRegexAt time.Time
	for _, run := range runs {
		if item.LastCheckedAt == nil || run.CreatedAt.After(*item.LastCheckedAt) {
			at := run.CreatedAt
			item.LastCheckedAt = &at
		}
		if degradationProbeType(run.ProbeType) != DegradationProbeTypeRegex {
			continue
		}
		item.Total++
		switch run.Status {
		case DegradationRunPassed:
			item.Passed++
			latencyTotal += run.LatencyMs
		case DegradationRunFailed:
			item.Failed++
			latencyTotal += run.LatencyMs
		case DegradationRunError:
			item.Error++
		}
		if item.LastRunStatus == "" || run.CreatedAt.After(lastRegexAt) {
			lastRegexAt = run.CreatedAt
			item.LastRunStatus = run.Status
		}
	}
	denominator := item.Passed + item.Failed
	if denominator > 0 {
		item.PassRate = float64(item.Passed) / float64(denominator)
		item.AvgLatencyMs = latencyTotal / denominator
	}
	if denominator >= 3 {
		if item.PassRate < threshold {
			item.Verdict = DegradationVerdictDegraded
		} else {
			item.Verdict = DegradationVerdictHealthy
		}
	}
	return item
}

func groupVerdict(models []DegradationModelOverview) string {
	hasHealthy := false
	for _, model := range models {
		if model.Verdict == DegradationVerdictDegraded {
			return DegradationVerdictDegraded
		}
		if model.Verdict == DegradationVerdictHealthy {
			hasHealthy = true
		}
	}
	if hasHealthy {
		return DegradationVerdictHealthy
	}
	return DegradationVerdictUnknown
}

func (s *DegradationService) enrichTargets(ctx context.Context, targets []*DegradationTarget) []*DegradationTarget {
	for _, target := range targets {
		s.enrichTarget(ctx, target)
	}
	return targets
}

func (s *DegradationService) enrichTarget(ctx context.Context, target *DegradationTarget) *DegradationTarget {
	if target == nil {
		return target
	}
	if s.groupRepo != nil {
		if group, err := s.groupRepo.GetByIDLite(ctx, target.GroupID); err == nil && group != nil {
			target.GroupName, target.Platform = group.Name, group.Platform
		}
	}
	if s.repo != nil {
		if probes, err := s.probesForTarget(ctx, target); err == nil {
			target.EnabledProbeCount = len(probes)
			target.VisualProbeCount = 0
			for _, probe := range probes {
				if degradationProbeType(probe.ProbeType) == DegradationProbeTypeVisual {
					target.VisualProbeCount++
				}
			}
		}
	}
	return target
}

func (s *DegradationService) PreviewAccount(ctx context.Context, groupID int64, model string) (*DegradationPreviewAccount, error) {
	if s.gateway == nil {
		return nil, infraerrors.InternalServer("DEGRADATION_UNAVAILABLE", "gateway service is unavailable")
	}
	account, err := s.gateway.SelectAccountForModelWithExclusions(ctx, &groupID, "", model, nil)
	if err != nil {
		if errors.Is(err, ErrNoAvailableAccounts) {
			return &DegradationPreviewAccount{Available: false, Reason: "No available account"}, nil
		}
		return nil, err
	}
	return &DegradationPreviewAccount{AccountID: account.ID, Name: account.Name, Platform: account.Platform, Available: true}, nil
}

func (s *DegradationService) SoftDeleteTargetsByGroupID(ctx context.Context, groupID int64) error {
	return s.repo.SoftDeleteTargetsByGroupID(ctx, groupID)
}
