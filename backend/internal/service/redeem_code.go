package service

import (
	"crypto/rand"
	"encoding/hex"
	"math"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 多次兑换码的限制上限。两者乘积即 used_count 的最大值，须落在 INT 列范围内。
const (
	RedeemMultiUseMaxUsers       = 1_000_000
	RedeemMultiUseMaxUsesPerUser = 1_000
)

type RedeemCode struct {
	ID        int64
	Code      string
	Type      string
	Value     float64
	Status    string
	UsedBy    *int64
	UsedAt    *time.Time
	Notes     string
	CreatedAt time.Time
	ExpiresAt *time.Time

	GroupID      *int64
	ValidityDays int

	// 多次兑换码（Type == RedeemTypeMultiUse）专用字段。
	// BenefitType 是每次兑换实际发放的权益类型；MaxUsers / MaxUsesPerUser 分别限制
	// 可兑换的用户数与每个用户可兑换的次数；UsedCount / UsedUserCount 为已用计数。
	BenefitType    string
	MaxUsers       int
	MaxUsesPerUser int
	UsedCount      int
	UsedUserCount  int

	// ParentID 非空表示这是某个多次兑换码的一次兑换流水；
	// ParentCode 为该多次兑换码的码值，仅在查询预加载时填充。
	ParentID   *int64
	ParentCode string

	User  *User
	Group *Group
}

// IsMultiUse 是否为多次兑换码（可被多个用户兑换、每个用户可兑换多次）。
func (r *RedeemCode) IsMultiUse() bool {
	return r != nil && r.Type == RedeemTypeMultiUse
}

// EffectiveType 返回兑换时实际发放的权益类型：多次兑换码取 BenefitType，其余即自身类型。
func (r *RedeemCode) EffectiveType() string {
	if r.IsMultiUse() {
		return r.BenefitType
	}
	return r.Type
}

// validateMultiUseRedeemCode 校验多次兑换码的权益与限制参数。
// 多次兑换码面向公开发放，只允许正向权益，不支持用负值做退款扣减。
func validateMultiUseRedeemCode(benefitType string, value float64, maxUsers, maxUsesPerUser int) error {
	switch benefitType {
	case RedeemTypeBalance:
		if !(value > 0) || math.IsInf(value, 0) {
			return infraerrors.BadRequest("REDEEM_CODE_VALUE_INVALID", "value must be greater than zero")
		}
	case RedeemTypeConcurrency:
		if !(value >= 1) || math.IsInf(value, 0) || value != math.Trunc(value) {
			return infraerrors.BadRequest("REDEEM_CODE_VALUE_INVALID", "value must be a positive integer for concurrency")
		}
	case RedeemTypeSubscription:
	default:
		return infraerrors.BadRequest("REDEEM_CODE_BENEFIT_TYPE_INVALID", "benefit_type must be balance, concurrency or subscription")
	}
	if maxUsers < 1 || maxUsers > RedeemMultiUseMaxUsers {
		return infraerrors.BadRequest("REDEEM_CODE_MAX_USERS_INVALID", "max_users must be between 1 and 1000000")
	}
	if maxUsesPerUser < 1 || maxUsesPerUser > RedeemMultiUseMaxUsesPerUser {
		return infraerrors.BadRequest("REDEEM_CODE_MAX_USES_PER_USER_INVALID", "max_uses_per_user must be between 1 and 1000")
	}
	return nil
}

func (r *RedeemCode) IsUsed() bool {
	return r.Status == StatusUsed
}

func (r *RedeemCode) IsExpired() bool {
	return r.IsExpiredAt(time.Now())
}

func (r *RedeemCode) IsExpiredAt(now time.Time) bool {
	if r == nil {
		return false
	}
	if r.Status == StatusExpired {
		return true
	}
	return r.Status == StatusUnused && r.ExpiresAt != nil && !r.ExpiresAt.After(now)
}

func (r *RedeemCode) CanUse() bool {
	return r.Status == StatusUnused && !r.IsExpired()
}

func GenerateRedeemCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
