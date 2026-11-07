CREATE TABLE tickets (
    id BIGSERIAL PRIMARY KEY,
    ticket_no VARCHAR(16) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subject TEXT NOT NULL,
    category VARCHAR(20) NOT NULL DEFAULT 'general' CHECK (category IN ('general', 'billing', 'technical', 'account', 'feature', 'other')),
    priority VARCHAR(20) NOT NULL DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    status VARCHAR(20) NOT NULL DEFAULT 'pending_admin' CHECK (status IN ('pending_admin', 'pending_user', 'resolved', 'closed')),
    assignee_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    user_unread BOOLEAN NOT NULL DEFAULT FALSE,
    admin_unread BOOLEAN NOT NULL DEFAULT TRUE,
    message_count INT NOT NULL DEFAULT 0 CHECK (message_count >= 0),
    last_message_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_message_by VARCHAR(20) NOT NULL DEFAULT 'user' CHECK (last_message_by IN ('user', 'admin', 'system')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ
);
CREATE INDEX tickets_user_updated ON tickets(user_id, updated_at DESC);
CREATE INDEX tickets_status_updated ON tickets(status, updated_at DESC);
CREATE INDEX tickets_assignee ON tickets(assignee_id);

CREATE TABLE ticket_agent_keys (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    key_prefix VARCHAR(12) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    scope VARCHAR(20) NOT NULL DEFAULT 'user' CHECK (scope IN ('user', 'admin')),
    qps_limit INT NOT NULL DEFAULT 2 CHECK (qps_limit BETWEEN 1 AND 1000),
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    last_used_at TIMESTAMPTZ,
    last_used_ip VARCHAR(64),
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX ticket_agent_keys_owner ON ticket_agent_keys(user_id, deleted_at);

CREATE TABLE ticket_messages (
    id BIGSERIAL PRIMARY KEY,
    ticket_id BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    author_type VARCHAR(20) NOT NULL CHECK (author_type IN ('user', 'admin', 'system')),
    author_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    via VARCHAR(20) NOT NULL DEFAULT 'web' CHECK (via IN ('web', 'agent', 'system')),
    agent_key_id BIGINT REFERENCES ticket_agent_keys(id) ON DELETE SET NULL,
    internal BOOLEAN NOT NULL DEFAULT FALSE,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (NOT internal OR author_type = 'admin')
);
CREATE INDEX ticket_messages_ticket_id ON ticket_messages(ticket_id, id);

CREATE TABLE ticket_notify_preferences (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    email_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    feishu_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    feishu_webhook_url_encrypted TEXT NOT NULL DEFAULT '',
    feishu_secret_encrypted TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
