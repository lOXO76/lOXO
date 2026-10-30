//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func adaptiveCNAccountTestAccount(id int64, platform string) *Account {
	return &Account{
		ID:          id,
		Name:        "adaptive-cn-test",
		Platform:    platform,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":      "sk-adaptive-test",
			"api_protocol": APIProtocolAdaptive,
			"api_base_urls": map[string]any{
				APIProtocolChatCompletions: "http://chat.example/v1",
				APIProtocolAnthropic:       "http://anthropic.example",
				APIProtocolResponses:       "http://responses.example",
			},
		},
	}
}

func adaptiveCNAccountTestService(account *Account, responses ...*http.Response) (*AccountTestService, *httpUpstreamRecorder) {
	repo := &openAIAccountTestRepo{
		mockAccountRepoForGemini: mockAccountRepoForGemini{
			accountsByID: map[int64]*Account{account.ID: account},
		},
	}
	upstream := &httpUpstreamRecorder{responses: responses}
	return &AccountTestService{
		accountRepo:  repo,
		httpUpstream: upstream,
		cfg:          rawChatCompletionsTestConfig(),
	}, upstream
}

func adaptiveCNChatTestResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(`data: {"choices":[{"delta":{"content":"chat ok"},"finish_reason":"stop"}]}

data: [DONE]

`)),
	}
}

func adaptiveCNAnthropicTestResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(`data: {"type":"content_block_delta","delta":{"text":"anthropic ok"}}

data: {"type":"message_stop"}

`)),
	}
}

func adaptiveCNResponsesTestResponse() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(`data: {"type":"response.output_text.delta","delta":"responses ok"}

data: {"type":"response.completed"}

`)),
	}
}

func TestAccountTestService_AdaptiveChatOnlyProvidersTestChatAndAnthropicEndpoints(t *testing.T) {
	account := adaptiveCNAccountTestAccount(301, PlatformZhipu)
	svc, upstream := adaptiveCNAccountTestService(
		account,
		adaptiveCNChatTestResponse(),
		adaptiveCNAnthropicTestResponse(),
	)
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "glm-4.7", "hello", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "http://chat.example/v1/chat/completions", upstream.requests[0].URL.String())
	require.Equal(t, "http://anthropic.example/v1/messages", upstream.requests[1].URL.String())
	require.Equal(t, "Bearer sk-adaptive-test", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "sk-adaptive-test", upstream.requests[1].Header.Get("x-api-key"))
	require.Equal(t, 1, strings.Count(recorder.Body.String(), `"type":"test_start"`))
	require.Equal(t, 1, strings.Count(recorder.Body.String(), `"type":"test_complete"`))
	require.Contains(t, recorder.Body.String(), "已通过原生 /v1/messages 验证")
}

func TestAccountTestService_AdaptiveDeepSeekAlsoTestsResponsesEndpoint(t *testing.T) {
	account := adaptiveCNAccountTestAccount(302, PlatformDeepseek)
	svc, upstream := adaptiveCNAccountTestService(
		account,
		adaptiveCNChatTestResponse(),
		adaptiveCNAnthropicTestResponse(),
		adaptiveCNResponsesTestResponse(),
	)
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "deepseek-chat", "", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 3)
	require.Equal(t, "http://responses.example/responses", upstream.requests[2].URL.String())
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(upstream.requests[2].Context()))
	require.Equal(t, "Bearer sk-adaptive-test", upstream.requests[2].Header.Get("Authorization"))
	require.True(t, gjson.GetBytes(upstream.bodies[2], "stream").Bool())
	require.False(t, gjson.GetBytes(upstream.bodies[2], "store").Bool())
	require.False(t, gjson.GetBytes(upstream.bodies[2], "instructions").Exists())
	require.Equal(t, 1, strings.Count(recorder.Body.String(), `"type":"test_complete"`))
	require.Contains(t, recorder.Body.String(), "已通过原生 /responses 验证")
}

func TestAccountTestService_AdaptiveKimiAlsoTestsResponsesEndpoint(t *testing.T) {
	account := adaptiveCNAccountTestAccount(306, PlatformKimi)
	svc, upstream := adaptiveCNAccountTestService(
		account,
		adaptiveCNChatTestResponse(),
		adaptiveCNAnthropicTestResponse(),
		adaptiveCNResponsesTestResponse(),
	)
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "k3-256k", "", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 3)
	require.Equal(t, "http://responses.example/v1/responses", upstream.requests[2].URL.String())
	require.Equal(t, HTTPUpstreamProfileOpenAI, HTTPUpstreamProfileFromContext(upstream.requests[2].Context()))
	require.Equal(t, "Bearer sk-adaptive-test", upstream.requests[2].Header.Get("Authorization"))
	require.True(t, gjson.GetBytes(upstream.bodies[2], "stream").Bool())
	require.False(t, gjson.GetBytes(upstream.bodies[2], "store").Bool())
	require.False(t, gjson.GetBytes(upstream.bodies[2], "instructions").Exists())
	require.Equal(t, 1, strings.Count(recorder.Body.String(), `"type":"test_complete"`))
	require.Contains(t, recorder.Body.String(), "已通过原生 /responses 验证")
}

func TestAccountTestService_AdaptiveStopsAndNamesFailingEndpoint(t *testing.T) {
	account := adaptiveCNAccountTestAccount(303, PlatformDeepseek)
	svc, upstream := adaptiveCNAccountTestService(
		account,
		adaptiveCNChatTestResponse(),
		newJSONResponse(http.StatusNotFound, `{"error":{"message":"missing messages route"}}`),
	)
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "deepseek-chat", "", AccountTestModeDefault)

	require.Error(t, err)
	require.Contains(t, err.Error(), "Adaptive Anthropic endpoint returned 404")
	require.Len(t, upstream.requests, 2)
	require.Contains(t, recorder.Body.String(), `"type":"error"`)
	require.NotContains(t, recorder.Body.String(), `"type":"test_complete"`)
}

func TestAccountTestService_AdaptiveRejectsInvalidAnthropicSuccessBody(t *testing.T) {
	account := adaptiveCNAccountTestAccount(305, PlatformKimi)
	svc, upstream := adaptiveCNAccountTestService(
		account,
		adaptiveCNChatTestResponse(),
		newJSONResponse(http.StatusOK, `<html>not an Anthropic stream</html>`),
	)
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "kimi-k2.5", "", AccountTestModeDefault)

	require.Error(t, err)
	require.Contains(t, err.Error(), "Adaptive Anthropic stream ended before message_stop")
	require.Len(t, upstream.requests, 2)
	require.Contains(t, recorder.Body.String(), `"type":"error"`)
	require.NotContains(t, recorder.Body.String(), `"type":"test_complete"`)
}

func TestAccountTestService_FixedCNChatProtocolStillTestsOnlyChatEndpoint(t *testing.T) {
	account := adaptiveCNAccountTestAccount(304, PlatformZhipu)
	account.Credentials["api_protocol"] = APIProtocolChatCompletions
	account.Credentials["base_url"] = "http://fixed-chat.example/v1"
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNChatTestResponse())
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "glm-4.7", "", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "http://fixed-chat.example/v1/chat/completions", upstream.requests[0].URL.String())
	require.Equal(t, 1, strings.Count(recorder.Body.String(), `"type":"test_complete"`))
}

func anthropicProtocolCNAccount(id int64, platform string, credentials map[string]any) *Account {
	base := map[string]any{
		"api_key":      "sk-anthropic-test",
		"api_protocol": APIProtocolAnthropic,
	}
	for key, value := range credentials {
		base[key] = value
	}
	return &Account{
		ID:          id,
		Name:        "anthropic-protocol-cn-test",
		Platform:    platform,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Concurrency: 1,
		Credentials: base,
	}
}

func TestAccountTestService_AnthropicProtocolProbesNativeEndpointWithoutBetaQuery(t *testing.T) {
	account := anthropicProtocolCNAccount(311, PlatformZhipu, map[string]any{
		"base_url": "https://open.bigmodel.cn/api/anthropic",
	})
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNAnthropicTestResponse())
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "glm-4.7", "", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	req := upstream.requests[0]
	// Native Anthropic path without the ?beta=true suffix the generic Claude tester appends.
	require.Equal(t, "https://open.bigmodel.cn/api/anthropic/v1/messages", req.URL.String())
	require.Empty(t, req.URL.RawQuery)
	require.Equal(t, "sk-anthropic-test", req.Header.Get("x-api-key"))
	require.Equal(t, "2023-06-01", req.Header.Get("anthropic-version"))
	require.Contains(t, recorder.Body.String(), `"type":"test_complete"`)
}

func TestAccountTestService_AnthropicProtocolFallsBackToProviderDefaultNotAnthropicDotCom(t *testing.T) {
	// base_url intentionally absent: forwarding resolves the per-platform default
	// Anthropic endpoint. The old fall-through probed https://api.anthropic.com
	// with the provider's API key.
	account := anthropicProtocolCNAccount(312, PlatformZhipu, nil)
	svc, upstream := adaptiveCNAccountTestService(account, adaptiveCNAnthropicTestResponse())
	c, _ := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "glm-4.7", "", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "https://open.bigmodel.cn/api/anthropic/v1/messages", upstream.requests[0].URL.String())
}

func TestAccountTestService_AnthropicProtocolRejectsOpenAICompatBaseURL(t *testing.T) {
	account := anthropicProtocolCNAccount(313, PlatformZhipu, map[string]any{
		"base_url": "https://open.bigmodel.cn/api/paas/v4",
	})
	svc, upstream := adaptiveCNAccountTestService(account)
	c, recorder := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "glm-4.7", "", AccountTestModeDefault)

	require.Error(t, err)
	// Fails fast locally: no upstream request with the wrong endpoint shape.
	require.Empty(t, upstream.requests)
	require.Contains(t, recorder.Body.String(), "looks like an OpenAI-compatible endpoint")
	require.Contains(t, recorder.Body.String(), "https://open.bigmodel.cn/api/anthropic")
}

func TestAccountTestService_AnthropicProtocol401MarksAccountError(t *testing.T) {
	account := anthropicProtocolCNAccount(314, PlatformKimi, map[string]any{
		"base_url": "https://api.moonshot.cn/anthropic",
	})
	svc, _ := adaptiveCNAccountTestService(
		account,
		newJSONResponse(http.StatusUnauthorized, `{"error":{"message":"invalid key"}}`),
	)
	c, _ := newTestContext()

	err := svc.TestAccountConnection(c, account.ID, "kimi-k2.5", "", AccountTestModeDefault)

	require.Error(t, err)
	require.Contains(t, err.Error(), "Anthropic endpoint returned 401")
	repo := svc.accountRepo.(*openAIAccountTestRepo)
	require.Equal(t, account.ID, repo.setErrorID)
}

func TestAccountTestService_AdaptivePassesPromptToEveryProtocol(t *testing.T) {
	account := adaptiveCNAccountTestAccount(401, PlatformDeepseek)
	svc, upstream := adaptiveCNAccountTestService(account,
		adaptiveCNChatTestResponse(), adaptiveCNAnthropicTestResponse(), adaptiveCNResponsesTestResponse())
	c, _ := newTestContext()
	const prompt = "Solve 13 + 29."
	require.NoError(t, svc.TestAccountConnection(c, account.ID, "deepseek-chat", prompt, AccountTestModeDefault))
	require.Len(t, upstream.bodies, 3)
	for i, path := range []string{"messages.0.content", "messages.0.content.0.text", "input.0.content.0.text"} {
		require.Equal(t, prompt, gjson.GetBytes(upstream.bodies[i], path).String())
		require.True(t, gjson.GetBytes(upstream.bodies[i], "stream").Bool())
	}
}

func TestAccountTestService_RunPromptBackgroundAdaptiveIsOneNonStreamingRequest(t *testing.T) {
	account := adaptiveCNAccountTestAccount(402, PlatformDeepseek)
	svc, upstream := adaptiveCNAccountTestService(account,
		newJSONResponse(http.StatusCreated, `{"choices":[{"message":{"content":"42"}}]}`))
	result, err := svc.RunPromptBackground(context.Background(), account.ID, "deepseek-chat", "Solve 13 + 29.")
	require.NoError(t, err)
	require.Empty(t, result.ErrorMessage)
	require.Equal(t, "42", result.ResponseText)
	require.Equal(t, account.ID, result.AccountID)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "http://chat.example/v1/chat/completions", upstream.requests[0].URL.String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Nil(t, upstream.requests[0].GetBody)
	require.True(t, HTTPUpstreamRedirectsDisabled(upstream.requests[0].Context()))
	require.Equal(t, "Solve 13 + 29.", gjson.GetBytes(upstream.lastBody, "messages.0.content").String())
	deadline, ok := upstream.requests[0].Context().Deadline()
	require.True(t, ok)
	require.WithinDuration(t, time.Now().Add(90*time.Second), deadline, 2*time.Second)
}

func TestAccountTestService_RunPromptBackgroundNativeProtocols(t *testing.T) {
	for _, tc := range []struct {
		name, platform, protocol, model, response, promptPath, urlSuffix string
	}{
		{"anthropic", PlatformZhipu, APIProtocolAnthropic, "glm-4.7", `{"content":[{"type":"thinking","thinking":"private"},{"type":"text","text":"42"}]}`, "messages.0.content.0.text", "/v1/messages"},
		{"responses", PlatformDeepseek, APIProtocolResponses, "deepseek-chat", `{"output":[{"type":"reasoning","summary":[{"text":"private"}]},{"type":"message","content":[{"type":"output_text","text":"42"}]}]}`, "input.0.content.0.text", "/responses"},
		{"claude", PlatformAnthropic, "", "claude-sonnet-4-6", `{"content":[{"type":"text","text":"42"}]}`, "messages.0.content.0.text", "/v1/messages?beta=true"},
		{"openai", PlatformOpenAI, "", "gpt-5.6", `{"output_text":"42"}`, "input.0.content.0.text", "/v1/responses"},
		{"gemini", PlatformGemini, "", "gemini-2.5-flash", `{"candidates":[{"content":{"parts":[{"thought":true,"text":"private"},{"text":"42"}]}}]}`, "contents.0.parts.0.text", ":generateContent"},
		{"bedrock", PlatformAnthropic, "bedrock", "claude-sonnet-4-6", `{"content":[{"type":"text","text":"42"}]}`, "messages.0.content.0.text", "/invoke"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := adaptiveCNAccountTestAccount(403, tc.platform)
			account.Credentials["api_protocol"] = tc.protocol
			account.Credentials["base_url"] = "http://native.example"
			if tc.protocol == "bedrock" {
				account.Type = AccountTypeBedrock
				account.Credentials["auth_mode"] = "apikey"
			}
			svc, upstream := adaptiveCNAccountTestService(account, newJSONResponse(http.StatusOK, tc.response))
			result, err := svc.RunPromptBackground(context.Background(), account.ID, tc.model, "Return 42.")
			require.NoError(t, err)
			require.Empty(t, result.ErrorMessage)
			require.Equal(t, "42", result.ResponseText)
			require.Len(t, upstream.requests, 1)
			require.True(t, strings.HasSuffix(upstream.lastReq.URL.String(), tc.urlSuffix), upstream.lastReq.URL.String())
			require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
			require.Equal(t, "Return 42.", gjson.GetBytes(upstream.lastBody, tc.promptPath).String())
		})
	}
}

func TestAccountTestService_RunPromptBackgroundReportsInvalidOrEmptyAnswers(t *testing.T) {
	for _, body := range []string{`{"choices":[]}`, `{"choices":[{"message":{"content":" "}}]}`, `{"error":{"message":"upstream failed"}}`, `<html>invalid</html>`} {
		account := adaptiveCNAccountTestAccount(404, PlatformDeepseek)
		svc, upstream := adaptiveCNAccountTestService(account, newJSONResponse(http.StatusOK, body))
		result, err := svc.RunPromptBackground(context.Background(), account.ID, "deepseek-chat", "Return 42.")
		require.NoError(t, err)
		require.Empty(t, result.ResponseText)
		require.NotEmpty(t, result.ErrorMessage)
		require.Len(t, upstream.requests, 1)
	}
}

func TestAccountTestService_RunTestBackgroundRetainsStreamingAllProtocolChecks(t *testing.T) {
	account := adaptiveCNAccountTestAccount(405, PlatformDeepseek)
	svc, upstream := adaptiveCNAccountTestService(account,
		adaptiveCNChatTestResponse(), adaptiveCNAnthropicTestResponse(), adaptiveCNResponsesTestResponse())
	result, err := svc.RunTestBackground(context.Background(), account.ID, "deepseek-chat")
	require.NoError(t, err)
	require.Equal(t, "success", result.Status)
	require.Len(t, upstream.requests, 3)
	for _, body := range upstream.bodies {
		require.True(t, gjson.GetBytes(body, "stream").Bool())
	}
}

func TestAccountTestService_RunPromptBackgroundAntigravityProbesOnce(t *testing.T) {
	for _, model := range []string{"claude-sonnet-4-6", "gemini-2.5-flash"} {
		t.Run(model, func(t *testing.T) {
			account := &Account{ID: 406, Platform: PlatformAntigravity, Type: AccountTypeUpstream,
				Credentials: map[string]any{"api_key": "test-key", "project_id": "test-project", "model_mapping": map[string]any{model: model}}}
			svc, upstream := adaptiveCNAccountTestService(account,
				newJSONResponse(http.StatusOK, `{"response":{"candidates":[{"content":{"parts":[{"text":"42"}]}}]}}`))
			svc.antigravityGatewayService = &AntigravityGatewayService{tokenProvider: &AntigravityTokenProvider{}, httpUpstream: upstream}
			result, err := svc.RunPromptBackground(context.Background(), account.ID, model, "Return 42.")
			require.NoError(t, err)
			require.Empty(t, result.ErrorMessage)
			require.Equal(t, "42", result.ResponseText)
			require.Len(t, upstream.requests, 1)
			require.True(t, strings.HasSuffix(upstream.lastReq.URL.String(), ":generateContent"))
			require.Equal(t, "Return 42.", gjson.GetBytes(upstream.lastBody, "request.contents.0.parts.0.text").String())
			require.Greater(t, gjson.GetBytes(upstream.lastBody, "request.generationConfig.maxOutputTokens").Int(), int64(1))
		})
	}
}

func TestAccountTestService_RunPromptBackgroundGrokPassesPrompt(t *testing.T) {
	account := &Account{ID: 407, Platform: PlatformGrok, Type: AccountTypeOAuth, Credentials: map[string]any{
		"access_token": "test-token", "refresh_token": "refresh-token", "expires_at": "2100-01-01T00:00:00Z"}}
	svc, upstream := adaptiveCNAccountTestService(account, newJSONResponse(http.StatusOK, `{"output_text":"42"}`))
	svc.grokTokenProvider = NewGrokTokenProvider(svc.accountRepo, nil)
	result, err := svc.RunPromptBackground(context.Background(), account.ID, "grok-4.3", "Return 42.")
	require.NoError(t, err)
	require.Empty(t, result.ErrorMessage)
	require.Equal(t, "42", result.ResponseText)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "Return 42.", gjson.GetBytes(upstream.lastBody, "input").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Nil(t, upstream.lastReq.GetBody)
}

func TestAccountTestService_RunPromptBackgroundCodexOAuthKeepsSingleSSERequest(t *testing.T) {
	account := &Account{ID: 408, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Credentials: map[string]any{"access_token": "test-token"}}
	svc, upstream := adaptiveCNAccountTestService(account, newJSONResponse(http.StatusOK,
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"42\"}\n\n"+
			"data: {\"type\":\"response.completed\"}\n\n"))
	result, err := svc.RunPromptBackground(context.Background(), account.ID, "gpt-5.6", "Return 42.")
	require.NoError(t, err)
	require.Empty(t, result.ErrorMessage)
	require.Equal(t, "42", result.ResponseText)
	require.Len(t, upstream.requests, 1)
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.Equal(t, "Return 42.", gjson.GetBytes(upstream.lastBody, "input.0.content.0.text").String())
	require.Nil(t, upstream.lastReq.GetBody)
}

func TestAccountTestService_RunPromptBackgroundNeverRetriesFailures(t *testing.T) {
	for _, tc := range []struct{ platform, accountType, body string }{
		{PlatformOpenAI, AccountTypeOAuth, `{"error":{"message":"busy"}}`},
		{PlatformGrok, AccountTypeOAuth, `{"error":{"message":"Access denied"}}`},
		{PlatformAntigravity, AccountTypeUpstream, `{"error":{"message":"busy","retryDelay":"1s"}}`},
	} {
		t.Run(tc.platform, func(t *testing.T) {
			account := &Account{ID: 409, Platform: tc.platform, Type: tc.accountType, Credentials: map[string]any{
				"api_key": "test-key", "project_id": "test-project", "access_token": "test-token", "refresh_token": "refresh-token",
				"expires_at": "2100-01-01T00:00:00Z"}}
			svc, upstream := adaptiveCNAccountTestService(account, newJSONResponse(http.StatusForbidden, tc.body))
			svc.grokTokenProvider = NewGrokTokenProvider(svc.accountRepo, nil)
			svc.antigravityGatewayService = &AntigravityGatewayService{tokenProvider: &AntigravityTokenProvider{}, httpUpstream: upstream}
			model := "gpt-5.6"
			if tc.platform == PlatformGrok {
				model = "grok-4.3"
			}
			if tc.platform == PlatformAntigravity {
				model = "claude-sonnet-4-6"
			}
			result, err := svc.RunPromptBackground(context.Background(), account.ID, model, "Return 42.")
			require.NoError(t, err)
			require.NotEmpty(t, result.ErrorMessage)
			require.Len(t, upstream.requests, 1)
			require.Nil(t, upstream.lastReq.GetBody)
		})
	}
}

func TestAccountTestService_RunPromptBackgroundRejectsTypeSafeWithoutRequest(t *testing.T) {
	account := &Account{ID: 410, Platform: PlatformTypeSafe, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "test-key"}}
	svc, upstream := adaptiveCNAccountTestService(account)
	result, err := svc.RunPromptBackground(context.Background(), account.ID, "jev-latest", "Return 42.")
	require.ErrorIs(t, err, errPromptProbeTypeSafeUnsupported)
	require.Nil(t, result)
	require.Empty(t, upstream.requests)
}

func TestAccountTestService_PromptProbeVisualStreams(t *testing.T) {
	for _, tc := range []struct {
		name, platform, protocol, model, body, suffix string
	}{
		{"adaptive", PlatformDeepseek, APIProtocolAdaptive, "deepseek-chat", "data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"private\",\"content\":\"<html>visual</html>\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", "/v1/chat/completions"},
		{"anthropic", PlatformZhipu, APIProtocolAnthropic, "glm-4.7", "data: {\"type\":\"content_block_delta\",\"delta\":{\"thinking\":\"private\"}}\n\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"<html>visual</html>\"}}\n\ndata: {\"type\":\"message_stop\"}\n\n", "/v1/messages"},
		{"responses", PlatformOpenAI, "", "gpt-5.6", "data: {\"type\":\"response.reasoning_summary_text.delta\",\"delta\":\"private\"}\n\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"<html>visual</html>\"}\n\ndata: {\"type\":\"response.completed\"}\n\n", "/v1/responses"},
		{"gemini", PlatformGemini, "", "gemini-2.5-flash", "data: {\"candidates\":[{\"content\":{\"parts\":[{\"thought\":true,\"text\":\"private\"},{\"text\":\"<html>visual</html>\"}]},\"finishReason\":\"STOP\"}]}\n\n", ":streamGenerateContent?alt=sse"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := adaptiveCNAccountTestAccount(501, tc.platform)
			account.Credentials["api_protocol"] = tc.protocol
			account.Credentials["base_url"] = "http://visual.example"
			svc, upstream := adaptiveCNAccountTestService(account, newJSONResponse(http.StatusOK, tc.body))
			result, err := svc.RunPromptBackgroundWithOptions(context.Background(), account.ID, tc.model, "Build HTML.", PromptProbeOptions{Stream: true, MaxOutputTokens: 8192, Timeout: 300 * time.Second})
			require.NoError(t, err)
			require.Empty(t, result.ErrorMessage)
			require.Equal(t, "<html>visual</html>", result.ResponseText)
			require.Len(t, upstream.requests, 1)
			require.True(t, strings.HasSuffix(upstream.lastReq.URL.String(), tc.suffix), upstream.lastReq.URL.String())
			require.Equal(t, "text/event-stream", upstream.lastReq.Header.Get("Accept"))
			require.Nil(t, upstream.lastReq.GetBody)
			if tc.platform != PlatformGemini {
				require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
			}
			if tc.protocol == APIProtocolAnthropic || tc.protocol == APIProtocolAdaptive {
				require.Equal(t, int64(8192), gjson.GetBytes(upstream.lastBody, "max_tokens").Int())
			} else {
				require.False(t, gjson.GetBytes(upstream.lastBody, "max_tokens").Exists())
				require.False(t, gjson.GetBytes(upstream.lastBody, "max_output_tokens").Exists())
			}
			deadline, ok := upstream.lastReq.Context().Deadline()
			require.True(t, ok)
			require.WithinDuration(t, time.Now().Add(300*time.Second), deadline, 2*time.Second)
		})
	}
}

func TestAccountTestService_PromptProbeCCOutputLimitPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, platform string
		stream         bool
		limit          int
		wantLimit      bool
	}{
		{"openai_visual_no_limit", PlatformOpenAI, true, 8192, false},
		{"non_openai_visual_limit", PlatformDeepseek, true, 8192, true},
		{"non_openai_regex_default_no_limit", PlatformDeepseek, false, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := adaptiveCNAccountTestAccount(502, tc.platform)
			account.Credentials["api_protocol"] = APIProtocolChatCompletions
			account.Credentials["base_url"] = "http://chat.example/v1"
			body := `{"choices":[{"message":{"content":"42"}}]}`
			if tc.stream {
				body = "data: {\"choices\":[{\"delta\":{\"content\":\"42\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
			}
			svc, upstream := adaptiveCNAccountTestService(account, newJSONResponse(http.StatusOK, body))
			// Force an OpenAI API-key account onto the existing CC capability path.
			if tc.platform == PlatformOpenAI {
				account.Extra = map[string]any{"openai_responses_supported": false}
			}
			result, err := svc.RunPromptBackgroundWithOptions(context.Background(), account.ID, "text-model", "Return 42.", PromptProbeOptions{Stream: tc.stream, MaxOutputTokens: tc.limit})
			require.NoError(t, err)
			require.Empty(t, result.ErrorMessage)
			require.Equal(t, "42", result.ResponseText)
			require.Len(t, upstream.requests, 1)
			require.Equal(t, tc.stream, gjson.GetBytes(upstream.lastBody, "stream").Bool())
			require.Equal(t, tc.wantLimit, gjson.GetBytes(upstream.lastBody, "max_tokens").Exists())
			if tc.wantLimit {
				require.Equal(t, int64(tc.limit), gjson.GetBytes(upstream.lastBody, "max_tokens").Int())
			}
		})
	}
}

func TestAccountTestService_PromptProbeAntigravityVisualStreams(t *testing.T) {
	for _, model := range []string{"claude-sonnet-4-6", "gemini-2.5-flash"} {
		t.Run(model, func(t *testing.T) {
			account := &Account{ID: 503, Platform: PlatformAntigravity, Type: AccountTypeUpstream,
				Credentials: map[string]any{"api_key": "key", "project_id": "project", "model_mapping": map[string]any{model: model}}}
			svc, upstream := adaptiveCNAccountTestService(account, newJSONResponse(http.StatusOK,
				"data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"thought\":true,\"text\":\"private\"},{\"text\":\"visual\"}]},\"finishReason\":\"STOP\"}]}}\n\n"))
			svc.antigravityGatewayService = &AntigravityGatewayService{tokenProvider: &AntigravityTokenProvider{}, httpUpstream: upstream}
			result, err := svc.RunPromptBackgroundWithOptions(context.Background(), account.ID, model, "Build HTML.", PromptProbeOptions{Stream: true, MaxOutputTokens: 8192})
			require.NoError(t, err)
			require.Empty(t, result.ErrorMessage)
			require.Equal(t, "visual", result.ResponseText)
			require.Len(t, upstream.requests, 1)
			require.True(t, strings.HasSuffix(upstream.lastReq.URL.String(), ":streamGenerateContent?alt=sse"))
			require.Equal(t, int64(8192), gjson.GetBytes(upstream.lastBody, "request.generationConfig.maxOutputTokens").Int())
		})
	}
}

func promptProbeBedrockFrame(t *testing.T, event string) []byte {
	t.Helper()
	name := ":event-type"
	var headers bytes.Buffer
	_ = headers.WriteByte(byte(len(name)))
	_, _ = headers.WriteString(name)
	_ = headers.WriteByte(7)
	require.NoError(t, binary.Write(&headers, binary.BigEndian, uint16(len("chunk"))))
	_, _ = headers.WriteString("chunk")
	payload, err := json.Marshal(map[string]string{"bytes": base64.StdEncoding.EncodeToString([]byte(event))})
	require.NoError(t, err)
	frame := make([]byte, 12)
	binary.BigEndian.PutUint32(frame[0:4], uint32(16+headers.Len()+len(payload)))
	binary.BigEndian.PutUint32(frame[4:8], uint32(headers.Len()))
	binary.BigEndian.PutUint32(frame[8:12], crc32.ChecksumIEEE(frame[:8]))
	frame = append(frame, headers.Bytes()...)
	frame = append(frame, payload...)
	return binary.BigEndian.AppendUint32(frame, crc32.ChecksumIEEE(frame))
}

func TestAccountTestService_PromptProbeBedrockVisualStreams(t *testing.T) {
	account := &Account{ID: 504, Platform: PlatformAnthropic, Type: AccountTypeBedrock,
		Credentials: map[string]any{"auth_mode": "apikey", "api_key": "key"}}
	body := append(promptProbeBedrockFrame(t, `{"type":"content_block_delta","delta":{"text":"visual"}}`), promptProbeBedrockFrame(t, `{"type":"message_stop"}`)...)
	svc, upstream := adaptiveCNAccountTestService(account, &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))})
	result, err := svc.RunPromptBackgroundWithOptions(context.Background(), account.ID, "claude-sonnet-4-6", "Build HTML.", PromptProbeOptions{Stream: true, MaxOutputTokens: 8192})
	require.NoError(t, err)
	require.Empty(t, result.ErrorMessage)
	require.Equal(t, "visual", result.ResponseText)
	require.Len(t, upstream.requests, 1)
	require.True(t, strings.HasSuffix(upstream.lastReq.URL.String(), "/invoke-with-response-stream"))
	require.Equal(t, int64(8192), gjson.GetBytes(upstream.lastBody, "max_tokens").Int())
}

type promptProbeBlockingUpstream struct {
	HTTPUpstream
	request   *http.Request
	blockBody bool
}

func (u *promptProbeBlockingUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	u.request = req
	if !u.blockBody {
		<-req.Context().Done()
		return nil, req.Context().Err()
	}
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: &promptProbeBlockingBody{ctx: req.Context()}}, nil
}

type promptProbeBlockingBody struct{ ctx context.Context }

func (r *promptProbeBlockingBody) Read([]byte) (int, error) { <-r.ctx.Done(); return 0, r.ctx.Err() }
func (r *promptProbeBlockingBody) Close() error             { return nil }

func TestAccountTestService_PromptProbeTimeoutBoundsEntireRequest(t *testing.T) {
	for _, blockBody := range []bool{false, true} {
		account := adaptiveCNAccountTestAccount(505, PlatformDeepseek)
		svc, _ := adaptiveCNAccountTestService(account)
		upstream := &promptProbeBlockingUpstream{blockBody: blockBody}
		svc.httpUpstream = upstream
		parent, cancel := context.WithTimeout(context.Background(), time.Hour)
		started := time.Now()
		result, err := svc.RunPromptBackgroundWithOptions(parent, account.ID, "deepseek-chat", "Build HTML.", PromptProbeOptions{Stream: true, Timeout: 30 * time.Millisecond})
		cancel()
		require.NoError(t, err)
		require.Contains(t, result.ErrorMessage, context.DeadlineExceeded.Error())
		require.Less(t, time.Since(started), time.Second)
		require.ErrorIs(t, upstream.request.Context().Err(), context.DeadlineExceeded)
	}
}
