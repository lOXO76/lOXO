package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
)

const maxCheckInTiers = 20

// CheckInTier is one reward range and its relative selection weight.
type CheckInTier struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Weight float64 `json:"weight"`
}

func checkInNumberValid(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// NormalizeCheckInTiers validates and copies tiers for the settings write path.
func NormalizeCheckInTiers(raw []CheckInTier) ([]CheckInTier, error) {
	if len(raw) > maxCheckInTiers {
		return nil, fmt.Errorf("check-in tiers exceed limit of %d", maxCheckInTiers)
	}
	if len(raw) == 0 {
		return []CheckInTier{}, nil
	}
	out := make([]CheckInTier, 0, len(raw))
	for i, tier := range raw {
		if !checkInNumberValid(tier.Min) || !checkInNumberValid(tier.Max) || tier.Min < 0 || tier.Max < tier.Min || tier.Max > 100000 {
			return nil, fmt.Errorf("check-in tier %d must satisfy 0 <= min <= max <= 100000", i)
		}
		if !checkInNumberValid(tier.Weight) || tier.Weight <= 0 {
			return nil, fmt.Errorf("check-in tier %d weight must be greater than 0", i)
		}
		out = append(out, tier)
	}
	return out, nil
}

// ValidateCheckInSettings validates both the tier list and the enabled-state
// invariant used by the admin settings write path and runtime reads.
func ValidateCheckInSettings(enabled bool, tiers []CheckInTier) ([]CheckInTier, error) {
	normalized, err := NormalizeCheckInTiers(tiers)
	if err != nil {
		return nil, err
	}
	if enabled && len(normalized) == 0 {
		return nil, fmt.Errorf("at least one check-in tier is required when check-in is enabled")
	}
	return normalized, nil
}

// parseCheckInTiers is deliberately tolerant on reads: a stale malformed setting
// must not prevent unrelated settings from loading. Runtime callers still reject
// an enabled setting when no valid tiers remain.
func parseCheckInTiers(raw string) []CheckInTier {
	out := make([]CheckInTier, 0)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out
	}
	var tiers []CheckInTier
	if err := json.Unmarshal([]byte(raw), &tiers); err != nil {
		slog.Warn("parseCheckInTiers: unmarshal failed", "error", err)
		return out
	}
	if len(tiers) > maxCheckInTiers {
		slog.Warn("parseCheckInTiers: too many tiers", "count", len(tiers))
		return out
	}
	for _, tier := range tiers {
		if _, err := NormalizeCheckInTiers([]CheckInTier{tier}); err != nil {
			continue
		}
		out = append(out, tier)
	}
	return out
}

func encodeCheckInTiers(tiers []CheckInTier) (string, error) {
	if len(tiers) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(tiers)
	if err != nil {
		return "", fmt.Errorf("marshal check-in tiers: %w", err)
	}
	return string(b), nil
}

// checkInRewardRange 返回所有档位合并后的奖励上下限，供用户端展示「今日可得区间」。
func checkInRewardRange(tiers []CheckInTier) (float64, float64) {
	if len(tiers) == 0 {
		return 0, 0
	}
	minReward, maxReward := tiers[0].Min, tiers[0].Max
	for _, tier := range tiers[1:] {
		minReward = math.Min(minReward, tier.Min)
		maxReward = math.Max(maxReward, tier.Max)
	}
	return minReward, maxReward
}

func checkInHasEnabledProvider(cfg CaptchaProviderConfig) bool {
	return cfg.TurnstileEnabled || cfg.Tencent.Enabled || cfg.Aliyun.Enabled
}
