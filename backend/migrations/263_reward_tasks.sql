CREATE TABLE reward_task_partners (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    app_key VARCHAR(64) NOT NULL UNIQUE,
    app_secret_encrypted TEXT NOT NULL,
    secret_hint VARCHAR(8) NOT NULL DEFAULT '',
    allowed_domains JSONB NOT NULL DEFAULT '[]',
    contact VARCHAR(200) NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_event_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE TABLE reward_tasks (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(100) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    task_type VARCHAR(20) NOT NULL,
    verify_mode VARCHAR(20) NOT NULL,
    partner_id BIGINT,
    target_url TEXT NOT NULL,
    icon_url TEXT NOT NULL DEFAULT '',
    reward_amount DECIMAL(20,8) NOT NULL,
    period VARCHAR(20) NOT NULL DEFAULT 'once',
    browse_seconds INT NOT NULL DEFAULT 0,
    idle_timeout_seconds INT NOT NULL DEFAULT 15,
    min_recharge_amount DECIMAL(20,2) NOT NULL DEFAULT 0,
    max_completions INT NOT NULL DEFAULT 0,
    completed_count INT NOT NULL DEFAULT 0,
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX reward_tasks_visible ON reward_tasks(enabled, deleted_at, sort_order);
CREATE TABLE reward_task_records (
    id BIGSERIAL PRIMARY KEY,
    task_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    period_key VARCHAR(20) NOT NULL,
    ticket VARCHAR(64) NOT NULL UNIQUE,
    status VARCHAR(20) NOT NULL,
    progress_ms BIGINT NOT NULL DEFAULT 0,
    recharge_total DECIMAL(20,2) NOT NULL DEFAULT 0,
    external_user_id VARCHAR(128) NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL,
    last_heartbeat_at TIMESTAMPTZ,
    ticket_expires_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    reward_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    task_title VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, task_id, period_key)
);
CREATE UNIQUE INDEX reward_task_records_external_user ON reward_task_records(task_id, period_key, external_user_id) WHERE external_user_id <> '';
CREATE INDEX reward_task_records_task_status ON reward_task_records(task_id, status);
CREATE INDEX reward_task_records_user_completed ON reward_task_records(user_id, completed_at DESC);
CREATE INDEX reward_task_records_completed ON reward_task_records(completed_at);
CREATE TABLE reward_task_events (
    id BIGSERIAL PRIMARY KEY,
    partner_id BIGINT NOT NULL,
    event_id VARCHAR(128) NOT NULL,
    event_type VARCHAR(20) NOT NULL DEFAULT '',
    ticket VARCHAR(64) NOT NULL DEFAULT '',
    task_id BIGINT,
    record_id BIGINT,
    user_id BIGINT,
    external_user_id VARCHAR(128) NOT NULL DEFAULT '',
    amount DECIMAL(20,2) NOT NULL DEFAULT 0,
    order_id VARCHAR(128) NOT NULL DEFAULT '',
    result VARCHAR(20) NOT NULL,
    error_code VARCHAR(64) NOT NULL DEFAULT '',
    remote_ip VARCHAR(64) NOT NULL DEFAULT '',
    payload JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX reward_task_events_idempotency ON reward_task_events(partner_id, event_id) WHERE result <> 'rejected';
CREATE INDEX reward_task_events_partner_created ON reward_task_events(partner_id, created_at DESC);
CREATE INDEX reward_task_events_created ON reward_task_events(created_at);
