//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type degradationUserSettingsRepoStub struct {
	*settingHandlerPublicRepoStub
	calls           int
	enableAfterRead bool
}

func (r *degradationUserSettingsRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	r.calls++
	values, err := r.settingHandlerPublicRepoStub.GetMultiple(ctx, keys)
	if r.enableAfterRead {
		r.values[service.SettingKeyDegradationDetectionEnabled] = "true"
	}
	return values, err
}

type degradationUserUserRepoStub struct {
	service.UserRepository
	user    *service.User
	userIDs []int64
}

func (r *degradationUserUserRepoStub) GetByID(_ context.Context, id int64) (*service.User, error) {
	r.userIDs = append(r.userIDs, id)
	return r.user, nil
}

type degradationUserSubscriptionRepoStub struct {
	service.UserSubscriptionRepository
	subscriptions []service.UserSubscription
	userIDs       []int64
}

func (r *degradationUserSubscriptionRepoStub) ListActiveByUserID(_ context.Context, id int64) ([]service.UserSubscription, error) {
	r.userIDs = append(r.userIDs, id)
	return r.subscriptions, nil
}

type degradationUserGroupRepoStub struct {
	service.GroupRepository
	groups []service.Group
	calls  int
}

func (r *degradationUserGroupRepoStub) ListActive(context.Context) ([]service.Group, error) {
	r.calls++
	return r.groups, nil
}

type degradationUserRunRepoStub struct {
	service.DegradationRepository
	runs           []*service.DegradationRun
	calls          int
	groupIDs       []int64
	model          string
	limit          int
	includeDetails bool
}

func (r *degradationUserRunRepoStub) ListRuns(_ context.Context, groupIDs []int64, model string, _ time.Time, limit int, includeDetails bool) ([]*service.DegradationRun, error) {
	r.calls++
	r.groupIDs = append([]int64(nil), groupIDs...)
	r.model, r.limit, r.includeDetails = model, limit, includeDetails
	return r.runs, nil
}

func TestDegradationUserHandler_DisabledSkipsServiceAndAuthorization(t *testing.T) {
	for _, endpoint := range []string{"overview", "runs"} {
		for _, authenticated := range []bool{false, true} {
			name := endpoint + "/anonymous"
			if authenticated {
				name = endpoint + "/authenticated"
			}
			t.Run(name, func(t *testing.T) {
				repo := &degradationUserSettingsRepoStub{
					settingHandlerPublicRepoStub: &settingHandlerPublicRepoStub{values: map[string]string{
						service.SettingKeyDegradationDetectionEnabled: "false",
						service.SettingKeyDegradationThreshold:        "0.65",
					}},
					enableAfterRead: true,
				}
				settings := service.NewSettingService(repo, &config.Config{})
				// Nil dependencies turn any service or authorization access into a failure.
				h := NewDegradationUserHandler(nil, settings, nil, nil, nil)
				path := "/api/v1/degradation/" + endpoint + "?window_hours=72&group_id=invalid"
				rec, c := degradationUserTestContext(path, authenticated)
				if endpoint == "overview" {
					h.Overview(c)
				} else {
					h.Runs(c)
				}

				require.Equal(t, http.StatusOK, rec.Code)
				require.Equal(t, 1, repo.calls, "a false-to-true toggle must not trigger another read")
				require.Equal(t, "true", repo.values[service.SettingKeyDegradationDetectionEnabled])
				var body struct {
					Code int                        `json:"code"`
					Data map[string]json.RawMessage `json:"data"`
				}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				require.Zero(t, body.Code)
				if endpoint == "overview" {
					require.JSONEq(t, "[]", string(body.Data["groups"]))
					require.JSONEq(t, "false", string(body.Data["enabled"]))
					require.JSONEq(t, "72", string(body.Data["window_hours"]))
					require.JSONEq(t, "0.65", string(body.Data["threshold"]))
				} else {
					require.JSONEq(t, "[]", string(body.Data["items"]))
				}
			})
		}
	}
}

func TestDegradationUserHandler_RunsUsesWhitelistDTOAndVisibleGroups(t *testing.T) {
	createdAt := time.Date(2026, time.October, 30, 9, 0, 0, 0, time.UTC)
	runs := &degradationUserRunRepoStub{runs: []*service.DegradationRun{{
		ID: 7, TargetID: 100, GroupID: 21, Model: "claude-sonnet-4-6", ProbeID: 200,
		ProbeName: "Reasoning probe", Status: service.DegradationRunPassed, ProbeType: service.DegradationProbeTypeRegex,
		AnswerText: "private answer sentinel", ErrorMessage: "private error sentinel",
		AccountID: 300, LatencyMs: 1234, CreatedAt: createdAt,
	}}}
	h, users, subscriptions, groups := newDegradationUserTestHandler(runs, false)
	rec, c := degradationUserTestContext("/api/v1/degradation/runs?model=claude-sonnet-4-6&limit=250", true)
	h.Runs(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, runs.calls)
	require.Equal(t, []int64{10, 21, 31}, runs.groupIDs)
	require.Equal(t, "claude-sonnet-4-6", runs.model)
	require.Equal(t, 200, runs.limit)
	require.False(t, runs.includeDetails)
	require.Equal(t, []int64{91}, users.userIDs)
	require.Equal(t, []int64{91}, subscriptions.userIDs)
	require.Equal(t, 1, groups.calls)

	var body struct {
		Code int `json:"code"`
		Data struct {
			Items []map[string]json.RawMessage `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Zero(t, body.Code)
	require.Len(t, body.Data.Items, 1)
	item := body.Data.Items[0]
	fields := make([]string, 0, len(item))
	for field := range item {
		fields = append(fields, field)
	}
	require.ElementsMatch(t, []string{"id", "group_id", "model", "probe_name", "probe_type", "artifact_kind", "status", "latency_ms", "created_at"}, fields)
	require.JSONEq(t, `{"id":7,"group_id":21,"model":"claude-sonnet-4-6","probe_name":"Reasoning probe","probe_type":"regex","artifact_kind":"","status":"passed","latency_ms":1234,"created_at":"2026-10-30T09:00:00Z"}`, mustMarshalDegradationUserItem(t, item))
	require.NotContains(t, rec.Body.String(), "private answer sentinel")
	require.NotContains(t, rec.Body.String(), "private error sentinel")
}

func TestDegradationUserHandler_RunsGroupScope(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		restrictPublic bool
		emptyGroups    bool
		wantGroupIDs   []int64
		wantCalls      int
	}{
		{name: "explicit grant", query: "?group_id=21", wantGroupIDs: []int64{21}, wantCalls: 1},
		{name: "subscription grant", query: "?group_id=31", wantGroupIDs: []int64{31}, wantCalls: 1},
		{name: "unrestricted public group", query: "?group_id=10", wantGroupIDs: []int64{10}, wantCalls: 1},
		{name: "invisible exclusive group", query: "?group_id=40"},
		{name: "restricted public group", query: "?group_id=10", restrictPublic: true},
		{name: "all restricted grants", restrictPublic: true, wantGroupIDs: []int64{21, 31}, wantCalls: 1},
		{name: "no visible groups", emptyGroups: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runs := &degradationUserRunRepoStub{}
			h, users, _, groups := newDegradationUserTestHandler(runs, tt.restrictPublic)
			if tt.emptyGroups {
				groups.groups = nil
			}
			rec, c := degradationUserTestContext("/api/v1/degradation/runs"+tt.query, true)
			h.Runs(c)

			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, tt.wantCalls, runs.calls)
			require.Equal(t, tt.wantGroupIDs, runs.groupIDs)
			require.Equal(t, []int64{91}, users.userIDs)
			require.JSONEq(t, `{"code":0,"message":"success","data":{"items":[]}}`, rec.Body.String())
		})
	}
}

func newDegradationUserTestHandler(runs *degradationUserRunRepoStub, restrictPublic bool) (*DegradationUserHandler, *degradationUserUserRepoStub, *degradationUserSubscriptionRepoStub, *degradationUserGroupRepoStub) {
	settings := service.NewSettingService(&settingHandlerPublicRepoStub{values: map[string]string{
		service.SettingKeyDegradationDetectionEnabled: "true",
	}}, &config.Config{})
	users := &degradationUserUserRepoStub{user: &service.User{
		ID: 91, AllowedGroups: []int64{21}, RestrictPublicGroups: restrictPublic,
	}}
	subscriptions := &degradationUserSubscriptionRepoStub{subscriptions: []service.UserSubscription{{
		UserID: 91, GroupID: 31, Status: service.SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour),
	}}}
	groups := &degradationUserGroupRepoStub{groups: []service.Group{
		{ID: 10}, {ID: 21, IsExclusive: true}, {ID: 31, IsExclusive: true}, {ID: 40, IsExclusive: true},
	}}
	keys := service.NewAPIKeyService(nil, users, groups, subscriptions, nil, nil, &config.Config{})
	svc := service.NewDegradationService(runs, groups, nil, nil, settings)
	return NewDegradationUserHandler(svc, settings, keys, groups, nil), users, subscriptions, groups
}

func degradationUserTestContext(path string, authenticated bool) (*httptest.ResponseRecorder, *gin.Context) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	if authenticated {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 91})
	}
	return rec, c
}

func mustMarshalDegradationUserItem(t *testing.T, item map[string]json.RawMessage) string {
	t.Helper()
	encoded, err := json.Marshal(item)
	require.NoError(t, err)
	return string(encoded)
}
