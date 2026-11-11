//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type multiUseRedeemCall struct {
	id         int64
	userID     int64
	recordCode string
}

// multiUseRedeemRepoStub 在一次性兑换码的内存仓储之上模拟多次兑换码的占用：
// 成功时写入一条流水行，或返回注入的错误。
type multiUseRedeemRepoStub struct {
	paymentOrderLifecycleRedeemRepo
	multiUseErr   error
	multiUseCalls []multiUseRedeemCall
}

func (r *multiUseRedeemRepoStub) RedeemMultiUse(_ context.Context, id, userID int64, recordCode string) (*RedeemCode, error) {
	r.multiUseCalls = append(r.multiUseCalls, multiUseRedeemCall{id: id, userID: userID, recordCode: recordCode})
	if r.multiUseErr != nil {
		return nil, r.multiUseErr
	}
	var parent *RedeemCode
	for _, code := range r.codesByCode {
		if code.ID == id {
			parent = code
		}
	}
	if parent == nil {
		return nil, ErrRedeemCodeNotFound
	}
	now := time.Now().UTC()
	parentID := parent.ID
	record := &RedeemCode{
		ID:           9000 + int64(len(r.multiUseCalls)),
		Code:         recordCode,
		Type:         parent.BenefitType,
		Value:        parent.Value,
		Status:       StatusUsed,
		UsedBy:       &userID,
		UsedAt:       &now,
		GroupID:      parent.GroupID,
		ValidityDays: parent.ValidityDays,
		ParentID:     &parentID,
		ParentCode:   parent.Code,
	}
	r.codesByCode[recordCode] = record
	cloned := *record
	return &cloned, nil
}

func newMultiUseRedeemRepoStub(codes ...*RedeemCode) *multiUseRedeemRepoStub {
	byCode := make(map[string]*RedeemCode, len(codes))
	for _, code := range codes {
		byCode[code.Code] = code
	}
	return &multiUseRedeemRepoStub{
		paymentOrderLifecycleRedeemRepo: paymentOrderLifecycleRedeemRepo{codesByCode: byCode},
	}
}

func multiUseBalanceCode() *RedeemCode {
	return &RedeemCode{
		ID: 301, Code: "MULTI-BALANCE", Type: RedeemTypeMultiUse, BenefitType: RedeemTypeBalance,
		Value: 8, Status: StatusUnused, MaxUsers: 10, MaxUsesPerUser: 2,
	}
}

// redeemBusyLockCacheStub 模拟兑换码的分布式锁正被其他请求持有。
type redeemBusyLockCacheStub struct {
	redeemRateLimitCacheStub
}

func (c *redeemBusyLockCacheStub) AcquireRedeemLock(context.Context, string, time.Duration) (bool, error) {
	c.acquireCalls++
	return false, nil
}

// 多次兑换码按本次兑换的流水行发放权益，且不计邀请返利；
// 同样的环境下一次性余额兑换码仍走原有路径并计返利（作为对照）。
func TestRedeemGrantsBalanceForSingleAndMultiUseCodes(t *testing.T) {
	singleUse := &RedeemCode{ID: 300, Code: "SINGLE-BALANCE", Type: RedeemTypeBalance, Value: 8, Status: StatusUnused}
	tests := []struct {
		name              string
		code              *RedeemCode
		wantUseCalls      int
		wantMultiUseCalls int
		wantRebateCalls   int
		wantParentCode    string
	}{
		{name: "single-use", code: singleUse, wantUseCalls: 1, wantRebateCalls: 1},
		{name: "multi-use", code: multiUseBalanceCode(), wantMultiUseCalls: 1, wantParentCode: "MULTI-BALANCE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			userID := int64(42)
			inviterID := int64(9001)
			redeemRepo := newMultiUseRedeemRepoStub(tt.code)
			userRepo := &mockUserRepo{getByIDUser: &User{ID: userID}}
			var credited []float64
			userRepo.updateBalanceFn = func(_ context.Context, id int64, amount float64) error {
				require.Equal(t, userID, id)
				credited = append(credited, amount)
				return nil
			}
			cache := &redeemRateLimitCacheStub{}
			affiliateRepo := &paymentFulfillmentAffiliateRepoStub{
				inviteeSummary: &AffiliateSummary{
					UserID: userID, AffCode: "INVITEE", InviterID: &inviterID, CreatedAt: time.Now().Add(-time.Hour),
				},
				inviterSummary: &AffiliateSummary{
					UserID: inviterID, AffCode: "INVITER", CreatedAt: time.Now().Add(-2 * time.Hour),
				},
			}
			settingSvc := NewSettingService(&paymentFulfillmentSettingRepoStub{values: map[string]string{
				SettingKeyAffiliateEnabled:           "true",
				SettingKeyAffiliateRebateRate:        "10",
				SettingKeyAffiliateRebateFreezeHours: "0",
			}}, nil)
			affiliateSvc := NewAffiliateService(affiliateRepo, settingSvc, nil, nil)
			svc := NewRedeemService(redeemRepo, userRepo, nil, cache, nil, client, nil, affiliateSvc)

			result, err := svc.Redeem(ctx, userID, tt.code.Code)

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, RedeemTypeBalance, result.Type)
			require.Equal(t, 8.0, result.Value)
			require.Equal(t, StatusUsed, result.Status)
			require.Equal(t, tt.wantParentCode, result.ParentCode)
			require.Equal(t, []float64{8}, credited)
			require.Len(t, redeemRepo.useCalls, tt.wantUseCalls)
			require.Len(t, redeemRepo.multiUseCalls, tt.wantMultiUseCalls)
			require.Len(t, affiliateRepo.accrueCalls, tt.wantRebateCalls)
			require.Zero(t, cache.incrementCalls)
			require.Equal(t, 1, cache.releaseCalls)

			if tt.code.IsMultiUse() {
				call := redeemRepo.multiUseCalls[0]
				require.Equal(t, tt.code.ID, call.id)
				require.Equal(t, userID, call.userID)
				require.NotEmpty(t, call.recordCode)
				require.NotEqual(t, tt.code.Code, call.recordCode)
				// 多次兑换码自身保持可用，只有流水行是已使用状态。
				require.Equal(t, StatusUnused, redeemRepo.codesByCode[tt.code.Code].Status)
				require.Nil(t, redeemRepo.codesByCode[tt.code.Code].UsedBy)
			}
		})
	}
}

// 多次兑换码允许多个用户同时兑换：别的请求持有分布式锁时不应失败，也不能释放别人的锁。
// 一次性兑换码和不存在的码则维持原有的「处理中」行为。
func TestRedeemLockContention(t *testing.T) {
	singleUse := &RedeemCode{ID: 300, Code: "SINGLE-BALANCE", Type: RedeemTypeBalance, Value: 8, Status: StatusUnused}
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "multi-use proceeds", input: "MULTI-BALANCE"},
		{name: "single-use is rejected", input: "SINGLE-BALANCE", wantErr: ErrRedeemCodeLocked},
		{name: "missing code is rejected", input: "MISSING", wantErr: ErrRedeemCodeLocked},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newPaymentConfigServiceTestClient(t)
			redeemRepo := newMultiUseRedeemRepoStub(multiUseBalanceCode(), singleUse)
			userRepo := &mockUserRepo{getByIDUser: &User{ID: 42}}
			credits := 0
			userRepo.updateBalanceFn = func(context.Context, int64, float64) error {
				credits++
				return nil
			}
			cache := &redeemBusyLockCacheStub{}
			svc := NewRedeemService(redeemRepo, userRepo, nil, cache, nil, client, nil, nil)

			result, err := svc.Redeem(context.Background(), 42, tt.input)

			require.Equal(t, 1, cache.acquireCalls)
			require.Zero(t, cache.releaseCalls)
			require.Zero(t, cache.incrementCalls)
			require.Empty(t, redeemRepo.useCalls)
			if tt.wantErr != nil {
				require.Nil(t, result)
				require.ErrorIs(t, err, tt.wantErr)
				require.Empty(t, redeemRepo.multiUseCalls)
				require.Zero(t, credits)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Len(t, redeemRepo.multiUseCalls, 1)
			require.Equal(t, 1, credits)
		})
	}
}

// 占用名额失败时不发放任何权益，并保留可供前端识别的错误原因。
func TestRedeemMultiUsePropagatesClaimErrors(t *testing.T) {
	tests := []struct {
		name       string
		claimErr   error
		wantReason string
	}{
		{"user limit reached", ErrRedeemCodeUserLimitReached, "REDEEM_CODE_USER_LIMIT_REACHED"},
		{"user quota exhausted", ErrRedeemCodeUserQuotaExhausted, "REDEEM_CODE_USER_QUOTA_EXHAUSTED"},
		{"expired while redeeming", ErrRedeemCodeExpired, "REDEEM_CODE_EXPIRED"},
		{"used up while redeeming", ErrRedeemCodeUsed, "REDEEM_CODE_USED"},
		{"removed while redeeming", ErrRedeemCodeNotFound, "REDEEM_CODE_USED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newPaymentConfigServiceTestClient(t)
			redeemRepo := newMultiUseRedeemRepoStub(multiUseBalanceCode())
			redeemRepo.multiUseErr = tt.claimErr
			userRepo := &mockUserRepo{getByIDUser: &User{ID: 42}}
			credits := 0
			userRepo.updateBalanceFn = func(context.Context, int64, float64) error {
				credits++
				return nil
			}
			svc := NewRedeemService(redeemRepo, userRepo, nil, &redeemRateLimitCacheStub{}, nil, client, nil, nil)

			result, err := svc.Redeem(context.Background(), 42, "MULTI-BALANCE")

			require.Nil(t, result)
			require.Error(t, err)
			require.True(t, infraerrors.IsConflict(err))
			require.Equal(t, tt.wantReason, infraerrors.Reason(err))
			require.Len(t, redeemRepo.multiUseCalls, 1)
			require.Zero(t, credits)
		})
	}
}

// 兑换前的校验对多次兑换码同样生效，且在占用名额之前就拒绝。
func TestRedeemMultiUseRejectsBeforeClaim(t *testing.T) {
	past := time.Now().Add(-time.Minute)
	tests := []struct {
		name       string
		mutate     func(*RedeemCode)
		wantReason string
		wantCount  int // 是否计入兑换失败次数
	}{
		{"used up", func(c *RedeemCode) { c.Status = StatusUsed }, "REDEEM_CODE_USED", 1},
		{"disabled", func(c *RedeemCode) { c.Status = StatusDisabled }, "REDEEM_CODE_USED", 1},
		{"expired", func(c *RedeemCode) { c.ExpiresAt = &past }, "REDEEM_CODE_EXPIRED", 1},
		{"subscription without group", func(c *RedeemCode) { c.BenefitType = RedeemTypeSubscription }, "REDEEM_CODE_INVALID", 0},
		{"missing benefit type", func(c *RedeemCode) { c.BenefitType = "" }, "REDEEM_CODE_UNSUPPORTED_TYPE", 0},
		{"invitation benefit", func(c *RedeemCode) { c.BenefitType = RedeemTypeInvitation }, "REDEEM_CODE_UNSUPPORTED_TYPE", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := multiUseBalanceCode()
			tt.mutate(code)
			redeemRepo := newMultiUseRedeemRepoStub(code)
			cache := &redeemRateLimitCacheStub{}
			svc := &RedeemService{redeemRepo: redeemRepo, cache: cache}

			result, err := svc.Redeem(context.Background(), 42, code.Code)

			require.Nil(t, result)
			require.Error(t, err)
			require.Equal(t, tt.wantReason, infraerrors.Reason(err))
			require.Empty(t, redeemRepo.multiUseCalls)
			require.Empty(t, redeemRepo.useCalls)
			require.Equal(t, tt.wantCount, cache.incrementCalls)
		})
	}
}

func TestRedeemCodeEffectiveType(t *testing.T) {
	require.Equal(t, RedeemTypeBalance, (&RedeemCode{Type: RedeemTypeBalance}).EffectiveType())
	require.Equal(t, RedeemTypeSubscription, (&RedeemCode{Type: RedeemTypeSubscription, BenefitType: "ignored"}).EffectiveType())
	require.Equal(t, RedeemTypeConcurrency, (&RedeemCode{Type: RedeemTypeMultiUse, BenefitType: RedeemTypeConcurrency}).EffectiveType())
	require.True(t, (&RedeemCode{Type: RedeemTypeMultiUse}).IsMultiUse())
	require.False(t, (&RedeemCode{Type: RedeemTypeBalance}).IsMultiUse())
	require.False(t, (*RedeemCode)(nil).IsMultiUse())
}
