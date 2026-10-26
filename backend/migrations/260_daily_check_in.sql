-- 每日签到记录及按日期的并发幂等约束。
CREATE TABLE IF NOT EXISTS user_check_ins (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL,
    check_in_date DATE NOT NULL,
    reward        DECIMAL(20,8) NOT NULL DEFAULT 0,
    tier_index    INT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_check_ins_user_date
    ON user_check_ins (user_id, check_in_date);

CREATE INDEX IF NOT EXISTS idx_user_check_ins_created_at
    ON user_check_ins (created_at);
