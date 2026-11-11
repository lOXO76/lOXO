-- 多次兑换码（type = multi_use）：一个兑换码可被多个用户兑换，每个用户可兑换多次。
--
-- max_users / max_uses_per_user 为父码上的限制，used_count / used_user_count 为已用计数；
-- 既有的一次性兑换码取默认值（1 个用户、每人 1 次），语义不变。
-- benefit_type 决定每次兑换发放的权益（balance / concurrency / subscription）。
-- 每次兑换会落一条 parent_id 指向父码的流水行（type = benefit_type、status = used），
-- 父码被删除时流水行保留。

ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS benefit_type VARCHAR(20) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS max_users INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS max_uses_per_user INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS used_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS used_user_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS parent_id BIGINT REFERENCES redeem_codes(id) ON DELETE SET NULL;

-- 按「父码 + 用户」统计该用户已兑换次数；绝大多数行没有父码，使用部分索引。
CREATE INDEX IF NOT EXISTS idx_redeem_codes_parent_id_used_by
    ON redeem_codes (parent_id, used_by)
    WHERE parent_id IS NOT NULL;
