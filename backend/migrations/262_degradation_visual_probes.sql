ALTER TABLE degradation_probes ADD COLUMN IF NOT EXISTS probe_type VARCHAR(16) NOT NULL DEFAULT 'regex';
ALTER TABLE degradation_probes ADD COLUMN IF NOT EXISTS max_output_tokens INT NOT NULL DEFAULT 0;
ALTER TABLE degradation_probes ADD COLUMN IF NOT EXISTS timeout_seconds INT NOT NULL DEFAULT 0;
ALTER TABLE degradation_probes ADD COLUMN IF NOT EXISTS run_interval_seconds INT NOT NULL DEFAULT 0;

ALTER TABLE degradation_runs ADD COLUMN IF NOT EXISTS probe_type VARCHAR(16) NOT NULL DEFAULT 'regex';
ALTER TABLE degradation_runs ADD COLUMN IF NOT EXISTS artifact_kind VARCHAR(16) NOT NULL DEFAULT '';
ALTER TABLE degradation_runs ADD COLUMN IF NOT EXISTS artifact_content TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_degradation_runs_visual_latest
    ON degradation_runs (group_id, model, probe_id, created_at DESC)
    WHERE probe_type = 'visual' AND status = 'rendered';

CREATE INDEX IF NOT EXISTS idx_degradation_runs_target_probe_created
    ON degradation_runs (target_id, probe_id, created_at DESC);
