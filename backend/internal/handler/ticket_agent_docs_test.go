package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTicketAgentPublicDocuments(t *testing.T) {
	router := gin.New()
	router.GET("/api/v1/agent/docs.md", TicketAgentDocs)
	router.GET("/api/v1/agent/openapi.json", TicketAgentOpenAPI)

	markdown := httptest.NewRecorder()
	router.ServeHTTP(markdown, httptest.NewRequest(http.MethodGet, "/api/v1/agent/docs.md", nil))
	if markdown.Code != http.StatusOK || markdown.Header().Get("Content-Type") != "text/markdown; charset=utf-8" {
		t.Fatalf("markdown response: code=%d content-type=%q", markdown.Code, markdown.Header().Get("Content-Type"))
	}
	for _, text := range []string{"curl", "Idempotency-Key", "Retry-After", "next_cursor", "has_more", "exponential", "AGENT_RATE_LIMIT_UNAVAILABLE", "X-Agent-Key", "only affects authentication failures", "two-minute lease", "agent_ip_requests_per_minute", "600", "0 disables", "before any key lookup", "public unicast IP"} {
		if !strings.Contains(markdown.Body.String(), text) {
			t.Errorf("markdown lacks %q", text)
		}
	}

	openapi := httptest.NewRecorder()
	router.ServeHTTP(openapi, httptest.NewRequest(http.MethodGet, "/api/v1/agent/openapi.json", nil))
	if openapi.Code != http.StatusOK {
		t.Fatalf("openapi status=%d", openapi.Code)
	}
	var document map[string]any
	if err := json.Unmarshal(openapi.Body.Bytes(), &document); err != nil {
		t.Fatalf("invalid OpenAPI JSON: %v", err)
	}
	if document["openapi"] != "3.1.0" {
		t.Fatalf("version=%v", document["openapi"])
	}
	paths := ticketDocumentObject(t, document["paths"])
	me := ticketDocumentObject(t, paths["/me"])
	meGet := ticketDocumentObject(t, me["get"])
	meResponses := ticketDocumentObject(t, meGet["responses"])
	rateLimited := ticketDocumentObject(t, meResponses["429"])
	description, ok := rateLimited["description"].(string)
	if !ok || !strings.Contains(description, "only affects authentication failures") || !strings.Contains(description, "valid usable keys are not blocked") {
		t.Fatalf("OpenAPI must describe failure-only IP blocking: %v", rateLimited["description"])
	}
	if !strings.Contains(description, "agent_ip_requests_per_minute") || !strings.Contains(description, "before any key lookup") {
		t.Fatalf("OpenAPI must describe the pre-authentication IP quota: %v", description)
	}
	rateHeaders := ticketDocumentObject(t, rateLimited["headers"])
	for _, name := range []string{"Retry-After", "X-RateLimit-Limit", "X-RateLimit-Remaining"} {
		if rateHeaders[name] == nil {
			t.Errorf("OpenAPI 429 lacks %s header", name)
		}
	}
	for path, methods := range map[string][]string{
		"/me": {"get"}, "/tickets": {"get", "post"}, "/tickets/{id}": {"get", "patch"},
		"/tickets/{id}/messages": {"post"}, "/tickets/{id}/close": {"post"}, "/tickets/{id}/reopen": {"post"},
		"/updates": {"get"}, "/docs.md": {"get"}, "/openapi.json": {"get"},
	} {
		entry, ok := paths[path].(map[string]any)
		if !ok {
			t.Errorf("missing OpenAPI path %s", path)
			continue
		}
		for _, method := range methods {
			if _, ok := entry[method]; !ok {
				t.Errorf("missing %s %s", method, path)
			}
		}
	}
	for _, path := range []string{"/docs.md", "/openapi.json"} {
		entry := ticketDocumentObject(t, paths[path])
		get := ticketDocumentObject(t, entry["get"])
		security, ok := get["security"].([]any)
		if !ok {
			t.Fatalf("public document %s lacks security array", path)
		}
		if len(security) != 0 {
			t.Errorf("public document %s unexpectedly requires authentication", path)
		}
	}
	components := ticketDocumentObject(t, document["components"])
	schemas := ticketDocumentObject(t, components["schemas"])
	for _, name := range []string{"TicketSummary", "AdminTicketSummary", "TicketMessage", "TicketDetail", "TicketAgentKey", "AgentMe", "AgentUpdates"} {
		if _, ok := schemas[name]; !ok {
			t.Errorf("missing schema %s", name)
		}
	}
	for _, path := range []string{"/tickets", "/tickets/{id}/messages"} {
		entry := ticketDocumentObject(t, paths[path])
		post := ticketDocumentObject(t, entry["post"])
		if post["requestBody"] == nil {
			t.Errorf("POST %s lacks request body", path)
		}
		responses := ticketDocumentObject(t, post["responses"])
		if responses["201"] == nil || responses["429"] == nil || responses["503"] == nil {
			t.Errorf("POST %s lacks success/rate-limit/unavailable response", path)
		}
	}
}

func ticketDocumentObject(t *testing.T, value any) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected JSON object, got %T", value)
	}
	return object
}
