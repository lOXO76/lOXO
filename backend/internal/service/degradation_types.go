package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	DegradationRunPassed   = "passed"
	DegradationRunFailed   = "failed"
	DegradationRunError    = "error"
	DegradationRunRendered = "rendered"

	DegradationProbeTypeRegex  = "regex"
	DegradationProbeTypeVisual = "visual"

	DegradationArtifactHTML = "html"
	DegradationArtifactSVG  = "svg"
	DegradationArtifactText = "text"

	DegradationVerdictHealthy  = "healthy"
	DegradationVerdictDegraded = "degraded"
	DegradationVerdictUnknown  = "unknown"

	degradationDefaultWindowHours = 24
	degradationMinIntervalSeconds = 300
	degradationMaxIntervalSeconds = 86400
	degradationMaxAnswerRunes     = 4000
	degradationMaxErrorRunes      = 1000
)

var (
	ErrDegradationInvalidWindow   = infraerrors.BadRequest("DEGRADATION_INVALID_WINDOW", "window_hours must be between 1 and 168")
	ErrDegradationProbeNotFound   = infraerrors.NotFound("DEGRADATION_PROBE_NOT_FOUND", "degradation probe not found")
	ErrDegradationTargetNotFound  = infraerrors.NotFound("DEGRADATION_TARGET_NOT_FOUND", "degradation target not found")
	ErrDegradationTargetDuplicate = infraerrors.Conflict("DEGRADATION_TARGET_DUPLICATE", "该分组下的该模型已存在检测目标")
	ErrDegradationNoProbes        = infraerrors.BadRequest("DEGRADATION_NO_PROBES", "目标没有任何可用题目")
	ErrDegradationTargetRunning   = infraerrors.Conflict("DEGRADATION_TARGET_RUNNING", "degradation target is already running")
	ErrDegradationRunNotFound     = infraerrors.NotFound("DEGRADATION_RUN_NOT_FOUND", "degradation run not found")
)

type DegradationProbe struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	Prompt             string    `json:"prompt"`
	ExpectRegex        string    `json:"expect_regex"`
	RegexFlags         string    `json:"regex_flags"`
	Negate             bool      `json:"negate"`
	Enabled            bool      `json:"enabled"`
	SortOrder          int       `json:"sort_order"`
	ProbeType          string    `json:"probe_type"`
	MaxOutputTokens    int       `json:"max_output_tokens"`
	TimeoutSeconds     int       `json:"timeout_seconds"`
	RunIntervalSeconds int       `json:"run_interval_seconds"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type DegradationTarget struct {
	ID                int64      `json:"id"`
	GroupID           int64      `json:"group_id"`
	GroupName         string     `json:"group_name"`
	Platform          string     `json:"platform"`
	Model             string     `json:"model"`
	ProbeIDs          []int64    `json:"probe_ids"`
	IntervalSeconds   int        `json:"interval_seconds"`
	Enabled           bool       `json:"enabled"`
	EnabledProbeCount int        `json:"enabled_probe_count"`
	VisualProbeCount  int        `json:"visual_probe_count"`
	LastRunAt         *time.Time `json:"last_run_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type DegradationRun struct {
	ID              int64     `json:"id"`
	TargetID        int64     `json:"target_id"`
	GroupID         int64     `json:"group_id"`
	Model           string    `json:"model"`
	ProbeID         int64     `json:"probe_id"`
	ProbeName       string    `json:"probe_name"`
	ProbeType       string    `json:"probe_type"`
	Status          string    `json:"status"`
	AnswerText      string    `json:"answer_text"`
	ErrorMessage    string    `json:"error_message"`
	LatencyMs       int64     `json:"latency_ms"`
	AccountID       int64     `json:"account_id"`
	ArtifactKind    string    `json:"artifact_kind"`
	ArtifactContent string    `json:"-"`
	ArtifactBytes   int64     `json:"artifact_bytes"`
	CreatedAt       time.Time `json:"created_at"`
}

type DegradationProbeRepository interface {
	CreateProbe(context.Context, *DegradationProbe) error
	GetProbe(context.Context, int64) (*DegradationProbe, error)
	UpdateProbe(context.Context, *DegradationProbe) error
	DeleteProbe(context.Context, int64) error
	ListProbes(context.Context, bool) ([]*DegradationProbe, error)
	ListProbesByIDs(context.Context, []int64, bool) ([]*DegradationProbe, error)
}

type DegradationTargetRepository interface {
	CreateTarget(context.Context, *DegradationTarget) error
	GetTarget(context.Context, int64) (*DegradationTarget, error)
	UpdateTarget(context.Context, *DegradationTarget) error
	DeleteTarget(context.Context, int64) error
	SoftDeleteTargetsByGroupID(context.Context, int64) error
	ListTargets(context.Context, *int64, bool) ([]*DegradationTarget, error)
	ListDueTargets(context.Context, time.Time) ([]*DegradationTarget, error)
	UpdateTargetLastRun(context.Context, int64, time.Time) error
}

type DegradationRunRepository interface {
	CreateRun(context.Context, *DegradationRun) error
	GetRun(context.Context, int64) (*DegradationRun, error)
	GetRunArtifact(context.Context, int64) (*DegradationRun, error)
	LatestProbeRuns(context.Context, int64, []int64) (map[int64]time.Time, error)
	ListVisualRuns(context.Context, int64, string, int64, int) ([]*DegradationRun, error)
	ListLatestVisualRuns(context.Context, []int64) ([]*DegradationRun, error)
	ListRunsByTarget(context.Context, int64, int) ([]*DegradationRun, error)
	ListRuns(context.Context, []int64, string, time.Time, int, bool) ([]*DegradationRun, error)
	ListRunsSince(context.Context, time.Time, []int64) ([]*DegradationRun, error)
	DeleteRunsBefore(context.Context, time.Time) (int64, error)
}

type DegradationRepository interface {
	DegradationProbeRepository
	DegradationTargetRepository
	DegradationRunRepository
}

type DegradationProbeCreateParams struct {
	Name               string `json:"name"`
	Prompt             string `json:"prompt"`
	ExpectRegex        string `json:"expect_regex"`
	RegexFlags         string `json:"regex_flags"`
	Negate             bool   `json:"negate"`
	Enabled            *bool  `json:"enabled"`
	SortOrder          int    `json:"sort_order"`
	ProbeType          string `json:"probe_type"`
	MaxOutputTokens    int    `json:"max_output_tokens"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
	RunIntervalSeconds int    `json:"run_interval_seconds"`
}

type DegradationTargetParams struct {
	GroupID         int64   `json:"group_id"`
	Model           string  `json:"model"`
	ProbeIDs        []int64 `json:"probe_ids"`
	IntervalSeconds int     `json:"interval_seconds"`
	Enabled         *bool   `json:"enabled"`
}

type DegradationRunListItem struct {
	ID           int64     `json:"id"`
	GroupID      int64     `json:"group_id"`
	Model        string    `json:"model"`
	ProbeName    string    `json:"probe_name"`
	ProbeType    string    `json:"probe_type"`
	ArtifactKind string    `json:"artifact_kind"`
	Status       string    `json:"status"`
	LatencyMs    int64     `json:"latency_ms"`
	CreatedAt    time.Time `json:"created_at"`
}

type DegradationModelOverview struct {
	Model         string                `json:"model"`
	Verdict       string                `json:"verdict"`
	PassRate      float64               `json:"pass_rate"`
	Total         int64                 `json:"total"`
	Passed        int64                 `json:"passed"`
	Failed        int64                 `json:"failed"`
	Error         int64                 `json:"error"`
	AvgLatencyMs  int64                 `json:"avg_latency_ms"`
	LastCheckedAt *time.Time            `json:"last_checked_at"`
	LastRunStatus string                `json:"last_run_status"`
	Showcases     []DegradationShowcase `json:"showcases"`
}

type DegradationShowcase struct {
	RunID        int64     `json:"run_id"`
	ProbeID      int64     `json:"probe_id"`
	ProbeName    string    `json:"probe_name"`
	ArtifactKind string    `json:"artifact_kind"`
	LatencyMs    int64     `json:"latency_ms"`
	CreatedAt    time.Time `json:"created_at"`
}

type DegradationRunTriggerResult struct {
	Accepted   bool      `json:"accepted"`
	ProbeCount int       `json:"probe_count"`
	StartedAt  time.Time `json:"started_at"`
}

type DegradationVisualViewToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type DegradationGroupOverview struct {
	GroupID       int64                      `json:"group_id"`
	GroupName     string                     `json:"group_name"`
	Platform      string                     `json:"platform"`
	Verdict       string                     `json:"verdict"`
	LastCheckedAt *time.Time                 `json:"last_checked_at"`
	Models        []DegradationModelOverview `json:"models"`
}

type DegradationOverview struct {
	Enabled     bool                       `json:"enabled"`
	WindowHours int                        `json:"window_hours"`
	Threshold   float64                    `json:"threshold"`
	GeneratedAt time.Time                  `json:"generated_at"`
	Groups      []DegradationGroupOverview `json:"groups"`
}

type DegradationPreviewAccount struct {
	AccountID int64  `json:"account_id"`
	Name      string `json:"account_name"`
	Platform  string `json:"platform"`
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}

type PromptProbeResult struct {
	ResponseText string
	ErrorMessage string
	LatencyMs    int64
	StartedAt    time.Time
	FinishedAt   time.Time
	AccountID    int64
}

type DegradationProbeExecOptions struct {
	Timeout         time.Duration
	MaxAnswerBytes  int
	Stream          bool
	MaxOutputTokens int
}

type DegradationProbeExecResult struct {
	AnswerText string
	LatencyMs  int64
	AccountID  int64
	Err        error
	fullAnswer string
}
