CREATE TABLE IF NOT EXISTS degradation_probes (
    id            BIGSERIAL PRIMARY KEY,
    name          VARCHAR(100) NOT NULL,
    prompt        TEXT NOT NULL,
    expect_regex  TEXT NOT NULL,
    regex_flags   VARCHAR(8) NOT NULL DEFAULT '',
    negate        BOOLEAN NOT NULL DEFAULT FALSE,
    enabled       BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order    INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS degradation_targets (
    id                BIGSERIAL PRIMARY KEY,
    group_id          BIGINT NOT NULL,
    model             VARCHAR(200) NOT NULL,
    probe_ids         JSONB NOT NULL DEFAULT '[]'::JSONB,
    interval_seconds  INT NOT NULL DEFAULT 1800,
    enabled           BOOLEAN NOT NULL DEFAULT TRUE,
    last_run_at       TIMESTAMPTZ NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_degradation_targets_group_model
    ON degradation_targets (group_id, model)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS degradation_runs (
    id             BIGSERIAL PRIMARY KEY,
    target_id      BIGINT NOT NULL,
    group_id       BIGINT NOT NULL,
    model          VARCHAR(200) NOT NULL,
    probe_id       BIGINT NOT NULL,
    probe_name     VARCHAR(100) NOT NULL,
    status         VARCHAR(16) NOT NULL,
    answer_text    TEXT NOT NULL DEFAULT '',
    error_message  TEXT NOT NULL DEFAULT '',
    latency_ms     INT NOT NULL DEFAULT 0,
    account_id     BIGINT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_degradation_runs_group_created
    ON degradation_runs (group_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_degradation_runs_target_created
    ON degradation_runs (target_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_degradation_runs_created
    ON degradation_runs (created_at);
