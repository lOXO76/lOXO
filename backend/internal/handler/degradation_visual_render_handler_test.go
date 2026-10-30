//go:build unit

package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type degradationVisualRenderRepoStub struct {
	service.DegradationRepository
	run   *service.DegradationRun
	err   error
	calls []int64
}

func (r *degradationVisualRenderRepoStub) GetRunArtifact(_ context.Context, id int64) (*service.DegradationRun, error) {
	r.calls = append(r.calls, id)
	return r.run, r.err
}

func TestDegradationVisualRender_BodiesAndSecurityHeaders(t *testing.T) {
	tests := []struct {
		name, kind, content, expected string
	}{
		{
			name: "html unchanged", kind: service.DegradationArtifactHTML,
			content:  "<!doctype html><script>window.test=1;</script><p>scene</p>",
			expected: "<!doctype html><script>window.test=1;</script><p>scene</p>",
		},
		{
			name: "svg centered document", kind: service.DegradationArtifactSVG,
			content:  "<svg><circle r=\"4\"/></svg>",
			expected: "<!doctype html><meta charset=\"utf-8\"><body style=\"margin:0;min-height:100vh;display:grid;place-items:center;background:#fff\"><svg><circle r=\"4\"/></svg></body>",
		},
		{
			name: "text escaped", kind: service.DegradationArtifactText,
			content:  "<script>alert('unsafe')</script>&\"text\"",
			expected: "<!doctype html><meta charset=\"utf-8\"><pre style=\"white-space:pre-wrap;overflow-wrap:anywhere;margin:0;padding:16px;font-family:monospace\">&lt;script&gt;alert(&#39;unsafe&#39;)&lt;/script&gt;&amp;&#34;text&#34;</pre>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &degradationVisualRenderRepoStub{run: &service.DegradationRun{
				ID: 42, ProbeType: service.DegradationProbeTypeVisual, Status: service.DegradationRunRendered,
				ArtifactKind: tt.kind, ArtifactContent: tt.content,
			}}
			tokens := service.NewDegradationVisualTokenService(degradationVisualRenderTestConfig())
			token, err := tokens.Issue(42)
			require.NoError(t, err)
			router := degradationVisualRenderTestRouter(repo, tokens)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/degradation/visual-render/"+token.Token, nil))

			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, tt.expected, rec.Body.String())
			require.Equal(t, []int64{42}, repo.calls)
			require.Equal(t, "text/html; charset=utf-8", rec.Header().Get("Content-Type"))
			assertDegradationVisualSecurityHeaders(t, rec)
		})
	}
}

func TestDegradationVisualRender_InvalidTokensAndMissingRunsShare404(t *testing.T) {
	cfg := degradationVisualRenderTestConfig()
	tokens := service.NewDegradationVisualTokenService(cfg)
	token, err := tokens.Issue(42)
	require.NoError(t, err)
	tampered := strings.Replace(token.Token, "v1.42.", "v1.43.", 1)
	for _, tt := range []struct {
		name, token string
		run         *service.DegradationRun
		err         error
		wantCalls   int
	}{
		{name: "invalid", token: "bad"},
		{name: "cross run tampering", token: tampered},
		{name: "expired", token: signExpiredDegradationVisualToken(cfg.JWT.Secret)},
		{name: "missing", token: token.Token, err: service.ErrDegradationRunNotFound, wantCalls: 1},
		{name: "repository failure", token: token.Token, err: errors.New("private database error"), wantCalls: 1},
		{name: "not visual", token: token.Token, run: &service.DegradationRun{ID: 42, ProbeType: service.DegradationProbeTypeRegex, Status: service.DegradationRunPassed}, wantCalls: 1},
		{name: "not rendered", token: token.Token, run: &service.DegradationRun{ID: 42, ProbeType: service.DegradationProbeTypeVisual, Status: service.DegradationRunError}, wantCalls: 1},
		{name: "invalid artifact kind", token: token.Token, run: &service.DegradationRun{ID: 42, ProbeType: service.DegradationProbeTypeVisual, Status: service.DegradationRunRendered, ArtifactKind: "unknown"}, wantCalls: 1},
		{name: "wrong run returned", token: token.Token, run: &service.DegradationRun{ID: 43, ProbeType: service.DegradationProbeTypeVisual, Status: service.DegradationRunRendered, ArtifactKind: service.DegradationArtifactHTML}, wantCalls: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &degradationVisualRenderRepoStub{run: tt.run, err: tt.err}
			router := degradationVisualRenderTestRouter(repo, tokens)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/degradation/visual-render/"+tt.token, nil))
			require.Equal(t, http.StatusNotFound, rec.Code)
			require.Equal(t, "Not found", rec.Body.String())
			require.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
			require.Len(t, repo.calls, tt.wantCalls)
			assertDegradationVisualSecurityHeaders(t, rec)
		})
	}
}

func degradationVisualRenderTestConfig() *config.Config {
	return &config.Config{JWT: config.JWTConfig{Secret: "degradation-render-test-secret"}}
}

func degradationVisualRenderTestRouter(repo service.DegradationRepository, tokens *service.DegradationVisualTokenService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.SecurityHeaders(config.CSPConfig{Enabled: true}, nil))
	handler := NewDegradationVisualRenderHandler(service.NewDegradationService(repo, nil, nil, nil, nil), tokens)
	router.GET("/api/v1/degradation/visual-render/:token", handler.Render)
	return router
}

func assertDegradationVisualSecurityHeaders(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, "SAMEORIGIN", rec.Header().Get("X-Frame-Options"), "must replace the global DENY value")
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))
	require.Equal(t, "private, no-store", rec.Header().Get("Cache-Control"))
	csp := rec.Header().Get("Content-Security-Policy")
	require.Equal(t, "sandbox allow-scripts; default-src 'none'; script-src 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net https://unpkg.com https://cdnjs.cloudflare.com https://esm.sh; style-src 'unsafe-inline' https://cdn.jsdelivr.net https://unpkg.com https://cdnjs.cloudflare.com https://fonts.googleapis.com; img-src data: blob:; font-src data: https://fonts.gstatic.com https://cdn.jsdelivr.net https://unpkg.com https://cdnjs.cloudflare.com; media-src data: blob:; worker-src blob:; connect-src 'none'; frame-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'self'", csp)
	require.Contains(t, csp, "sandbox allow-scripts")
	require.Contains(t, csp, "connect-src 'none'")
	require.NotContains(t, csp, "allow-same-origin")
	require.Len(t, rec.Header().Values("Content-Security-Policy"), 1)
	require.Len(t, rec.Header().Values("X-Frame-Options"), 1)
}

func signExpiredDegradationVisualToken(secret string) string {
	keyMAC := hmac.New(sha256.New, []byte(secret))
	_, _ = keyMAC.Write([]byte("sub2api:degradation-visual-render:v1"))
	payload := fmt.Sprintf("v1.42.%d", time.Now().Add(-time.Minute).Unix())
	mac := hmac.New(sha256.New, keyMAC.Sum(nil))
	_, _ = mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
