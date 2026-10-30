//go:build unit

package service

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestDegradationExecutorUsesCompositeUpstreamModelAndFullAnswer(t *testing.T) {
	const groupID int64 = 12
	account := adaptiveCNAccountTestAccount(500, PlatformDeepseek)
	account.Schedulable = true
	account.Credentials["model_mapping"] = map[string]any{"deepseek-chat": "deepseek-chat"}
	answer := strings.Repeat("x", 4100) + "42"
	accountTests, upstream := adaptiveCNAccountTestService(account,
		newJSONResponse(http.StatusOK, `{"choices":[{"message":{"content":"`+answer+`"}}]}`))
	groups := &degradationServiceGroupRepoStub{groups: map[int64]*Group{
		groupID: {ID: groupID, Platform: PlatformComposite, Status: StatusActive},
	}}
	gateway := &GatewayService{
		groupRepo:   groups,
		accountRepo: &mockAccountRepoForPlatform{accounts: []Account{*account}, accountsByID: map[int64]*Account{account.ID: account}},
		cfg:         testConfig(),
		compositeResolver: NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{
			{ID: 1, GroupID: groupID, PublicModel: "public-alias", UpstreamModel: "deepseek-chat", MatchType: CompositeRouteMatchExact, Endpoint: CompositeRouteEndpointAny, TargetPlatform: PlatformDeepseek, Enabled: true},
		}}),
	}
	executor := NewDegradationProbeExecutor(gateway, accountTests)
	result := executor.Execute(context.Background(), groupID, "public-alias", "Return 42.", DegradationProbeExecOptions{})
	require.NoError(t, result.Err)
	require.Equal(t, account.ID, result.AccountID)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "deepseek-chat", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "Return 42.", gjson.GetBytes(upstream.lastBody, "messages.0.content").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Len(t, []rune(result.AnswerText), 4000)
	require.Equal(t, answer, result.fullAnswer)
	re, err := compileProbeRegex("42$", "")
	require.NoError(t, err)
	require.True(t, judgeProbeAnswer(re, false, result.fullAnswer), "judgment must not lose a suffix after the storage limit")
	require.False(t, judgeProbeAnswer(re, false, result.AnswerText))
}

func TestDegradationTruncateRunesPreservesUTF8(t *testing.T) {
	require.Equal(t, "测试", truncateDegradationRunes("测试答案", 2))
	require.Empty(t, truncateDegradationRunes("answer", 0))
}
