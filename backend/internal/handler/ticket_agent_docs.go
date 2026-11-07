package handler

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

// TicketAgentDocs 提供可被 AI Agent 直接读取的公开接入文档。
func TicketAgentDocs(c *gin.Context) {
	c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(ticketAgentMarkdown))
}

// TicketAgentOpenAPI 提供公开的 OpenAPI 3.1 契约。
func TicketAgentOpenAPI(c *gin.Context) {
	c.JSON(http.StatusOK, ticketAgentOpenAPI())
}

const ticketAgentMarkdown = `# Ticket Agent API

This document describes the ticket integration API for AI agents. The base path
is /api/v1/agent on the same origin as this document. OpenAPI 3.1 is available at
/api/v1/agent/openapi.json. Both documents are public and rate limited by client
IP (60 requests per minute); ticket operations require an agent key.

## 1. Create and protect a key

Sign in to the web console, open Tickets > Agent integration, and create a key.
The key is displayed **once**. Store it in a secret manager or environment
variable. Never put it into URLs, logs, source control, or message bodies.
Keys use the form lxt_ followed by 43 base64url characters. The server stores only
a SHA-256 hash. The displayed key_prefix cannot authenticate a request.

An ordinary account creates user-scope keys. An administrator can explicitly
create admin-scope keys. A user-scope key always acts as its owner, even if its
owner is an administrator. An admin-scope key can access all tickets, customer
identities, and internal notes. Give this scope only to trusted agents. Every
request rechecks that its owner is still an active administrator. Disabling,
revoking, or expiring a key immediately prevents subsequent requests.

Set the following environment variables. BASE_URL is the origin without a
trailing slash, for example https://support.example.com.

~~~sh
export BASE_URL='https://support.example.com'
export TICKET_AGENT_KEY='lxt_REPLACE_WITH_THE_COMPLETE_KEY'
~~~

## 2. Authentication and response envelopes

Send Authorization: Bearer <key>. X-Agent-Key: <key> is also accepted. No browser
JWT, cookies, panel API key, or admin token is needed.

~~~sh
curl --fail-with-body "$BASE_URL/api/v1/agent/me" \
  -H "Authorization: Bearer $TICKET_AGENT_KEY"
~~~

Success responses have {"code":0,"message":"success","data":...}. Use the HTTP
status and reason field to handle failures; do not parse localized message text.
Error responses use {"code":<http_status>,"message":"...","reason":"..."} and
may also include metadata. A paginated data object is
{"items":[],"total":0,"page":1,"page_size":20,"pages":1}.
All identifiers and cursors are JSON integers. All timestamps are RFC3339.
Nullable fields are present with null. Arrays are always arrays, including when
empty. Message bodies are plain text and must never be rendered as HTML.

## 3. Enumerations and visibility

* status: pending_admin, pending_user, resolved, closed.
* category: general, billing, technical, account, feature, other. Default general.
* priority: low, normal, high, urgent. Default normal.
* author_type: user, admin, system. via: web, agent, system.

User-scope keys only access their owner's tickets. An inaccessible ticket returns
404 TICKET_NOT_FOUND, exactly like a nonexistent ticket. Internal notes are never
returned to user-scope keys. In user-facing messages, administrator author_id is
null and author_name is always Support. Admin-scope responses include internal
notes and administrator identities. A message written by an agent has via=agent.

## 4. Ticket endpoints

The following examples use the common envelope but abbreviate long objects with
'...'. The request body is JSON and should include Content-Type: application/json.

### List tickets

GET /api/v1/agent/tickets?page=1&page_size=20&status=pending_admin&keyword=refund

User scope lists only its own tickets. Admin scope accepts status, priority,
category, keyword (ticket number/subject/user email), user_id, assignee_id and
unread_only. page_size is bounded by the server.

~~~sh
curl "$BASE_URL/api/v1/agent/tickets?page=1&page_size=20" \
  -H "Authorization: Bearer $TICKET_AGENT_KEY"
~~~

### Create a ticket (user scope only)

POST /api/v1/agent/tickets

~~~sh
curl -X POST "$BASE_URL/api/v1/agent/tickets" \
  -H "Authorization: Bearer $TICKET_AGENT_KEY" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: client-request-20261106-0001' \
  -d '{"subject":"Refund for invoice 123","category":"billing","priority":"normal","body":"Please check this invoice."}'
~~~

The response is 201 with TicketDetail. The Idempotency-Key is optional but
strongly recommended for retries; completed results are retained for 24 hours
for the same agent and operation. In-flight requests hold a two-minute lease.
If a request loses its response, check /updates before retrying after that lease:
an interrupted database commit and Redis result write are not atomic.

### Get a ticket

GET /api/v1/agent/tickets/:id returns TicketDetail or AdminTicketDetail. Add
after_message_id to fetch only messages with a larger id (the ticket summary is
still returned). A successful GET clears the unread flag for the current scope.

~~~sh
curl "$BASE_URL/api/v1/agent/tickets/123?after_message_id=500" \
  -H "Authorization: Bearer $TICKET_AGENT_KEY"
~~~

### Add a message

POST /api/v1/agent/tickets/:id/messages. User scope sends '{"body":"..."}';
admin scope may additionally send '{"internal":true}' for an internal note.
Internal notes never notify the customer. User replies to closed tickets receive
409 TICKET_CLOSED; reopen first. Use Idempotency-Key exactly as for create.

~~~sh
curl -X POST "$BASE_URL/api/v1/agent/tickets/123/messages" \
  -H "Authorization: Bearer $TICKET_AGENT_KEY" \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: message-20261106-0001' \
  -d '{"body":"I have attached the requested details."}'
~~~

### Close, reopen, and administer

POST /api/v1/agent/tickets/:id/close and /reopen are user-scope operations.
PATCH /api/v1/agent/tickets/:id is admin-scope only and accepts any status,
priority, and assignee_id (0 cancels assignment; assignee must be an active
administrator). Moving to resolved or closed creates a system message. Closed_at
is set on close and cleared on reopen.

~~~sh
curl -X POST "$BASE_URL/api/v1/agent/tickets/123/reopen" \
  -H "Authorization: Bearer $TICKET_AGENT_KEY"
curl -X PATCH "$BASE_URL/api/v1/agent/tickets/123" \
  -H "Authorization: Bearer $TICKET_AGENT_KEY" \
  -H 'Content-Type: application/json' \
  -d '{"status":"resolved","priority":"high"}'
~~~

## 5. Polling updates

GET /api/v1/agent/updates?cursor=0&limit=50 returns
'{"items":[{"cursor":501,"ticket":{...},"message":{...}}],"next_cursor":501,"has_more":false}'.
It includes messages visible to the key with id > cursor, in ascending id order.
User scope excludes internal messages. Persist next_cursor durably before the
next poll. If has_more is true, poll immediately with next_cursor; otherwise
wait 15 seconds. On errors use exponential backoff (15s, 30s, 60s, then 5m),
respect Retry-After, and reset to 15s after a successful response. Do not poll
more frequently than the advertised rate limits.

## 6. Rate limits and retries

Before key authentication, all protected Agent routes share a fixed one-minute
request limit per resolved public unicast IP. The administrator setting
agent_ip_requests_per_minute defaults to 600 (range 0..100000; 0 disables it).
Every request consumes this quota, including malformed or invalid keys and valid
keys. Loopback, private, link-local, unspecified and non-unicast addresses skip
this quota to avoid merging an unconfigured reverse proxy into a shared bucket.
The IP uses the same security-client-IP resolution as authentication failures.
Exceeding this quota returns 429 AGENT_RATE_LIMITED before any key lookup, with
Retry-After rounded up to the remaining minute-window time. Redis failures
return 503 AGENT_RATE_LIMIT_UNAVAILABLE. Public docs retain a separate 60/minute
IP limit. After a settings change, other server instances may take up to one
minute to refresh their cached IP limit.

Every authenticated response includes X-RateLimit-Limit and
X-RateLimit-Remaining. Limits are a fixed one-second window per key and per
owner (all keys combined). Responses rejected by the pre-authentication IP quota
instead report that minute quota in these headers. Business limits also apply across web and agent:
new tickets per hour, messages per minute, and open-ticket count. A 429 includes
Retry-After in seconds; sleep at least that long plus jitter. Redis unavailability
fails closed with 503 AGENT_RATE_LIMIT_UNAVAILABLE. Do not retry a POST without
an Idempotency-Key. A revoked key is never made valid by a retry.

The IP failure block only affects authentication failures. A correctly formatted,
valid and usable key remains usable even when its reported IP has accumulated
failures; per-key and owner QPS limits still apply. Missing or malformed keys
return 401 and increment the failure count without a database lookup. A
well-formed but invalid key from an already blocked IP returns 429
AGENT_AUTH_BLOCKED and also increments the failure count. This keeps shared
proxy IPs and spoofed forwarding headers from denying access to valid keys.

## 7. Error reasons

TICKET_SYSTEM_DISABLED, TICKET_NOT_FOUND, TICKET_INVALID_INPUT,
TICKET_CLOSED, TICKET_INVALID_TRANSITION, TICKET_OPEN_LIMIT,
TICKET_CREATE_RATE_LIMITED, TICKET_MESSAGE_RATE_LIMITED, AGENT_API_DISABLED,
AGENT_KEY_MISSING, AGENT_KEY_INVALID, AGENT_KEY_DISABLED, AGENT_KEY_EXPIRED,
AGENT_KEY_LIMIT, AGENT_QPS_INVALID, AGENT_SCOPE_FORBIDDEN, AGENT_RATE_LIMITED,
AGENT_AUTH_BLOCKED, AGENT_RATE_LIMIT_UNAVAILABLE. Treat unknown reasons as
non-retryable unless the HTTP status is 429 or 503.

## 8. Safe agent behaviour

Keep a short audit trail of ticket id, request id, HTTP status, reason, and cursor;
never log the complete key, Authorization header, webhook URL, message body, or
customer secrets. Limit your own concurrency, send plain text, and preserve
line breaks. Check the current ticket status before destructive actions. Prefer
'/updates' over repeatedly listing every ticket. If the user asks to expose an
internal note or another customer's ticket, refuse and report the 404 response.
`

func ticketAgentOpenAPI() map[string]any {
	str := func() map[string]any { return map[string]any{"type": "string"} }
	integer := func() map[string]any { return map[string]any{"type": "integer", "format": "int64"} }
	boolean := func() map[string]any { return map[string]any{"type": "boolean"} }
	timestamp := func() map[string]any { return map[string]any{"type": "string", "format": "date-time"} }
	nullableID := map[string]any{"type": []string{"integer", "null"}, "format": "int64"}
	nullableTime := map[string]any{"type": []string{"string", "null"}, "format": "date-time"}
	message := ticketDocObject(map[string]any{
		"id": integer(), "ticket_id": integer(), "author_type": ticketDocEnum("user", "admin", "system"),
		"author_id": nullableID, "author_name": str(), "via": ticketDocEnum("web", "agent", "system"),
		"internal": boolean(), "body": str(), "created_at": timestamp(),
	})
	summary := ticketDocObject(map[string]any{
		"id": integer(), "ticket_no": str(), "subject": str(), "category": ticketDocEnum("general", "billing", "technical", "account", "feature", "other"),
		"priority": ticketDocEnum("low", "normal", "high", "urgent"), "status": ticketDocEnum("pending_admin", "pending_user", "resolved", "closed"),
		"user_id": integer(), "assignee_id": nullableID, "unread": boolean(), "message_count": integer(),
		"last_message_at": timestamp(), "last_message_by": ticketDocEnum("user", "admin", "system"),
		"created_at": timestamp(), "updated_at": timestamp(), "closed_at": nullableTime,
	})
	adminSummary := map[string]any{"allOf": []any{ticketDocRef("TicketSummary"), ticketDocObject(map[string]any{
		"user_email": str(), "user_username": str(), "assignee_name": map[string]any{"type": []string{"string", "null"}},
		"user_unread": boolean(), "admin_unread": boolean(),
	})}}
	key := ticketDocObject(map[string]any{
		"id": integer(), "name": str(), "key_prefix": str(), "scope": ticketDocEnum("user", "admin"),
		"qps_limit": integer(), "status": ticketDocEnum("active", "disabled"), "last_used_at": nullableTime,
		"last_used_ip": map[string]any{"type": []string{"string", "null"}}, "expires_at": nullableTime, "created_at": timestamp(),
	})
	summaryUnion := map[string]any{"anyOf": []any{ticketDocRef("TicketSummary"), ticketDocRef("AdminTicketSummary")}}
	detail := ticketDocObject(map[string]any{"ticket": summaryUnion, "messages": ticketDocArray(ticketDocRef("TicketMessage"))})
	page := ticketDocObject(map[string]any{"items": ticketDocArray(summaryUnion), "total": integer(), "page": integer(), "page_size": integer(), "pages": integer()})
	create := ticketDocObject(map[string]any{
		"subject":  map[string]any{"type": "string", "minLength": 1, "description": "Unicode characters; default maximum 200, configurable by the site administrator."},
		"body":     map[string]any{"type": "string", "minLength": 1},
		"category": map[string]any{"type": "string", "enum": []string{"general", "billing", "technical", "account", "feature", "other"}, "default": "general"},
		"priority": map[string]any{"type": "string", "enum": []string{"low", "normal", "high", "urgent"}, "default": "normal"},
	})
	create["required"] = []string{"subject", "body"}
	messageInput := ticketDocObject(map[string]any{"body": map[string]any{"type": "string", "minLength": 1}, "internal": map[string]any{"type": "boolean", "default": false, "description": "Admin scope only."}})
	messageInput["required"] = []string{"body"}
	patch := ticketDocObject(map[string]any{
		"status":      ticketDocEnum("pending_admin", "pending_user", "resolved", "closed"),
		"priority":    ticketDocEnum("low", "normal", "high", "urgent"),
		"assignee_id": map[string]any{"type": "integer", "minimum": 0, "description": "Active admin id; 0 cancels assignment."},
	})
	delete(patch, "required")
	me := ticketDocObject(map[string]any{
		"key": ticketDocRef("TicketAgentKey"), "owner": ticketDocObject(map[string]any{"id": integer(), "role": str(), "username": str()}),
		"limits": ticketDocObject(map[string]any{"qps_limit": integer(), "owner_total_qps": integer(), "max_body_length": integer()}),
	})
	updates := ticketDocObject(map[string]any{
		"items":       ticketDocArray(ticketDocObject(map[string]any{"cursor": integer(), "ticket": ticketDocRef("TicketSummary"), "message": ticketDocRef("TicketMessage")})),
		"next_cursor": integer(), "has_more": boolean(),
	})
	list := ticketAgentOperation("listTickets", "List visible tickets", "User scope sees only its own tickets; admin scope supports all filters.", "TicketPage", "200", nil)
	list["parameters"] = []any{
		ticketDocQuery("page", map[string]any{"type": "integer", "minimum": 1, "default": 1}),
		ticketDocQuery("page_size", map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "default": 20}),
		ticketDocQuery("status", ticketDocEnum("pending_admin", "pending_user", "resolved", "closed")),
		ticketDocQuery("priority", ticketDocEnum("low", "normal", "high", "urgent")),
		ticketDocQuery("category", ticketDocEnum("general", "billing", "technical", "account", "feature", "other")),
		ticketDocQuery("keyword", str()), ticketDocQuery("user_id", integer()), ticketDocQuery("assignee_id", integer()), ticketDocQuery("unread_only", boolean()),
	}
	get := ticketAgentOperation("getTicket", "Read a ticket and clear unread", "Returns 404 for inaccessible tickets. User scope never sees internal messages or admin identities.", "TicketDetail", "200", nil)
	get["parameters"] = []any{ticketDocQuery("after_message_id", map[string]any{"type": "integer", "minimum": 0, "default": 0})}
	createOp := ticketAgentOperation("createTicket", "Create a ticket (user scope)", "Admin scope receives AGENT_SCOPE_FORBIDDEN. New status is pending_admin.", "TicketDetail", "201", ticketDocRef("CreateTicketRequest"))
	messageOp := ticketAgentOperation("addTicketMessage", "Append a message", "User public replies set pending_admin; admin public replies set pending_user. Internal admin notes preserve status and do not notify the user.", "TicketMessage", "201", ticketDocRef("TicketMessageRequest"))
	for _, op := range []map[string]any{createOp, messageOp} {
		op["parameters"] = []any{map[string]any{"name": "Idempotency-Key", "in": "header", "required": false, "schema": map[string]any{"type": "string"}, "description": "Retry token scoped to the agent key and request. Completed results are retained for 24 hours; in-flight leases expire after two minutes. Check /updates after an unknown outcome before retrying an expired lease."}}
	}
	updateOp := ticketAgentOperation("updateTicket", "Update a ticket (admin scope)", "Any status is allowed. Resolved/closed transitions append a system message and notify the owner.", "AdminTicketSummary", "200", ticketDocRef("UpdateTicketRequest"))
	updateFeed := ticketAgentOperation("pollUpdates", "Poll messages visible to this key", "Messages with id > cursor in ascending order. Persist next_cursor; immediately drain has_more, otherwise poll every 15 seconds with exponential error backoff.", "AgentUpdates", "200", nil)
	updateFeed["parameters"] = []any{ticketDocQuery("cursor", map[string]any{"type": "integer", "minimum": 0, "default": 0}), ticketDocQuery("limit", map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "default": 50})}
	publicDocs := func(description, contentType string) map[string]any {
		return map[string]any{"summary": description, "security": []any{}, "responses": map[string]any{"200": map[string]any{"description": description, "content": map[string]any{contentType: map[string]any{"schema": map[string]any{"type": "string"}}}}, "429": map[string]any{"description": "Public document IP rate limit (60/minute)."}}}
	}
	return map[string]any{
		"openapi": "3.1.0", "jsonSchemaDialect": "https://json-schema.org/draft/2020-12/schema",
		"info":     map[string]any{"title": "Ticket Agent API", "version": "1.0.0", "description": "Support tickets using scoped lxt_ keys. All JSON operations use the standard {code,message,data} envelope. Protected routes share a pre-authentication one-minute request quota per resolved public unicast IP (agent_ip_requests_per_minute: default 600, range 0..100000, 0 disables). All credentials consume it; private, loopback, link-local, unspecified and non-unicast IPs skip it. Uses the same security-client-IP resolution as authentication failures. Exceeding it returns 429 AGENT_RATE_LIMITED before key lookup; Redis errors fail closed with 503. Settings are cached for up to one minute across server instances. Public docs have a separate 60/minute IP limit. IP failure blocks only affect authentication failures: valid usable keys remain accepted, subject to IP request quota and key/owner QPS. Missing or malformed keys return 401 without a database lookup when the IP quota permits."},
		"servers":  []map[string]any{{"url": "/api/v1/agent"}},
		"security": []map[string]any{{"bearerAuth": []string{}}, {"agentKeyHeader": []string{}}},
		"components": map[string]any{
			"securitySchemes": map[string]any{"bearerAuth": map[string]any{"type": "http", "scheme": "bearer", "description": "Full lxt_ agent key."}, "agentKeyHeader": map[string]any{"type": "apiKey", "in": "header", "name": "X-Agent-Key"}},
			"schemas": map[string]any{"TicketSummary": summary, "AdminTicketSummary": adminSummary, "TicketMessage": message, "TicketDetail": detail, "TicketAgentKey": key, "TicketPage": page, "CreateTicketRequest": create, "TicketMessageRequest": messageInput, "UpdateTicketRequest": patch, "AgentMe": me, "AgentUpdates": updates,
				"Error": map[string]any{"type": "object", "required": []string{"code", "message"}, "properties": map[string]any{"code": integer(), "message": str(), "reason": str(), "metadata": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}}}},
			},
		},
		"paths": map[string]any{
			"/me":                    map[string]any{"get": ticketAgentOperation("getAgentMe", "Inspect the current key and owner", "Returns live owner role and effective limits. Does not reveal the key secret or hash.", "AgentMe", "200", nil)},
			"/tickets":               map[string]any{"get": list, "post": createOp},
			"/tickets/{id}":          map[string]any{"parameters": []any{ticketDocPathID()}, "get": get, "patch": updateOp},
			"/tickets/{id}/messages": map[string]any{"parameters": []any{ticketDocPathID()}, "post": messageOp},
			"/tickets/{id}/close":    map[string]any{"parameters": []any{ticketDocPathID()}, "post": ticketAgentOperation("closeTicket", "Close a ticket (user scope)", "Sets closed status and closed_at.", "TicketSummary", "200", nil)},
			"/tickets/{id}/reopen":   map[string]any{"parameters": []any{ticketDocPathID()}, "post": ticketAgentOperation("reopenTicket", "Reopen a ticket (user scope)", "Only resolved or closed tickets can reopen; sets pending_admin and clears closed_at.", "TicketSummary", "200", nil)},
			"/updates":               map[string]any{"get": updateFeed},
			"/docs.md":               map[string]any{"get": publicDocs("Plain Markdown integration guide", "text/markdown")},
			"/openapi.json":          map[string]any{"get": publicDocs("This OpenAPI 3.1 document", "application/json")},
		},
	}
}

func ticketDocObject(properties map[string]any) map[string]any {
	keys := make([]string, 0, len(properties))
	for name := range properties {
		keys = append(keys, name)
	}
	slices.Sort(keys)
	return map[string]any{"type": "object", "required": keys, "properties": properties}
}

func ticketDocEnum(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}

func ticketDocRef(name string) map[string]any {
	return map[string]any{"$ref": "#/components/schemas/" + name}
}

func ticketDocArray(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

func ticketDocPathID() map[string]any {
	return map[string]any{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "integer", "format": "int64", "minimum": 1}}
}

func ticketDocQuery(name string, schema map[string]any) map[string]any {
	return map[string]any{"name": name, "in": "query", "required": false, "schema": schema}
}

func ticketAgentOperation(id, summary, description, result, status string, request map[string]any) map[string]any {
	headers := map[string]any{
		"X-RateLimit-Limit":     map[string]any{"description": "Key requests per one-second fixed window; pre-authentication IP quota rejections report the IP minute limit instead.", "schema": map[string]any{"type": "integer"}},
		"X-RateLimit-Remaining": map[string]any{"description": "Remaining requests in the current window.", "schema": map[string]any{"type": "integer"}},
	}
	errorResponse := func(description string) map[string]any {
		return map[string]any{"description": description, "content": map[string]any{"application/json": map[string]any{"schema": ticketDocRef("Error")}}}
	}
	rateLimited := errorResponse("AGENT_RATE_LIMITED includes the pre-authentication public unicast IP minute quota (agent_ip_requests_per_minute: default 600, 0 disables), plus key/owner QPS. Every protected request consumes the IP quota before any key lookup; private/loopback/link-local/non-unicast IPs skip it. Also TICKET_OPEN_LIMIT, TICKET_CREATE_RATE_LIMITED, TICKET_MESSAGE_RATE_LIMITED. AGENT_AUTH_BLOCKED only affects authentication failures: well-formed invalid keys on blocked IPs receive 429 and increment failures; valid usable keys are not blocked by IP failures, but remain subject to the separate IP request quota.")
	rateLimited["headers"] = map[string]any{"X-RateLimit-Limit": headers["X-RateLimit-Limit"], "X-RateLimit-Remaining": headers["X-RateLimit-Remaining"], "Retry-After": map[string]any{"description": "Wait at least this number of seconds before retrying; pre-authentication IP quotas use the remaining minute-window time, rounded up.", "schema": map[string]any{"type": "integer", "minimum": 1}}}
	op := map[string]any{
		"operationId": id, "summary": summary, "description": description,
		"responses": map[string]any{
			status: map[string]any{"description": "Success", "headers": headers, "content": map[string]any{"application/json": map[string]any{"schema": ticketDocObject(map[string]any{"code": map[string]any{"type": "integer", "const": 0}, "message": map[string]any{"type": "string"}, "data": ticketDocRef(result)})}}},
			"400":  errorResponse("TICKET_INVALID_INPUT."), "401": errorResponse("AGENT_KEY_MISSING, AGENT_KEY_INVALID, AGENT_KEY_DISABLED, AGENT_KEY_EXPIRED."),
			"403": errorResponse("TICKET_SYSTEM_DISABLED, AGENT_API_DISABLED, AGENT_SCOPE_FORBIDDEN."), "404": errorResponse("TICKET_NOT_FOUND."),
			"409": errorResponse("TICKET_CLOSED, TICKET_INVALID_TRANSITION."), "429": rateLimited, "503": errorResponse("AGENT_RATE_LIMIT_UNAVAILABLE: Redis is unavailable; authentication fails closed."),
		},
	}
	if request != nil {
		op["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": request}}}
	}
	return op
}
