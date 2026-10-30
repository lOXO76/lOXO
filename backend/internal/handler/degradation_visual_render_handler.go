package handler

import (
	"html"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const degradationVisualContentSecurityPolicy = "sandbox allow-scripts; default-src 'none'; script-src 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net https://unpkg.com https://cdnjs.cloudflare.com https://esm.sh; style-src 'unsafe-inline' https://cdn.jsdelivr.net https://unpkg.com https://cdnjs.cloudflare.com https://fonts.googleapis.com; img-src data: blob:; font-src data: https://fonts.gstatic.com https://cdn.jsdelivr.net https://unpkg.com https://cdnjs.cloudflare.com; media-src data: blob:; worker-src blob:; connect-src 'none'; frame-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'self'"

type DegradationVisualRenderHandler struct {
	svc    *service.DegradationService
	tokens *service.DegradationVisualTokenService
}

func NewDegradationVisualRenderHandler(svc *service.DegradationService, tokens *service.DegradationVisualTokenService) *DegradationVisualRenderHandler {
	return &DegradationVisualRenderHandler{svc: svc, tokens: tokens}
}

func (h *DegradationVisualRenderHandler) Render(c *gin.Context) {
	c.Header("Content-Security-Policy", degradationVisualContentSecurityPolicy)
	c.Header("X-Frame-Options", "SAMEORIGIN")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Cache-Control", "private, no-store")
	if h == nil || h.svc == nil || h.tokens == nil {
		degradationVisualNotFound(c)
		return
	}
	runID, err := h.tokens.Verify(c.Param("token"))
	if err != nil {
		degradationVisualNotFound(c)
		return
	}
	run, err := h.svc.GetVisualArtifact(c.Request.Context(), runID)
	if err != nil || run == nil || run.ID != runID {
		degradationVisualNotFound(c)
		return
	}
	var content string
	switch run.ArtifactKind {
	case service.DegradationArtifactHTML:
		content = run.ArtifactContent
	case service.DegradationArtifactSVG:
		content = "<!doctype html><meta charset=\"utf-8\"><body style=\"margin:0;min-height:100vh;display:grid;place-items:center;background:#fff\">" + run.ArtifactContent + "</body>"
	case service.DegradationArtifactText:
		content = "<!doctype html><meta charset=\"utf-8\"><pre style=\"white-space:pre-wrap;overflow-wrap:anywhere;margin:0;padding:16px;font-family:monospace\">" + html.EscapeString(run.ArtifactContent) + "</pre>"
	default:
		degradationVisualNotFound(c)
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(content))
}

func degradationVisualNotFound(c *gin.Context) {
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusNotFound, "Not found")
}
