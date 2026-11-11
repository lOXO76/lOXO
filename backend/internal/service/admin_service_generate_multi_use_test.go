//go:build unit

package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type generateRedeemRepoStub struct {
	RedeemCodeRepository
	created []RedeemCode
}

func (r *generateRedeemRepoStub) Create(_ context.Context, code *RedeemCode) error {
	code.ID = int64(len(r.created) + 1)
	r.created = append(r.created, *code)
	return nil
}

func TestGenerateRedeemCodesMultiUse(t *testing.T) {
	repo := &generateRedeemRepoStub{}
	svc := &adminServiceImpl{redeemCodeRepo: repo}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count:          2,
		Type:           RedeemTypeMultiUse,
		BenefitType:    RedeemTypeBalance,
		Value:          5,
		MaxUsers:       100,
		MaxUsesPerUser: 3,
	})

	require.NoError(t, err)
	require.Len(t, codes, 2)
	require.Len(t, repo.created, 2)
	require.NotEqual(t, codes[0].Code, codes[1].Code)
	for _, code := range repo.created {
		require.Equal(t, RedeemTypeMultiUse, code.Type)
		require.Equal(t, RedeemTypeBalance, code.BenefitType)
		require.Equal(t, 5.0, code.Value)
		require.Equal(t, StatusUnused, code.Status)
		require.Equal(t, 100, code.MaxUsers)
		require.Equal(t, 3, code.MaxUsesPerUser)
		require.Nil(t, code.GroupID)
	}
}

func TestGenerateRedeemCodesMultiUseDefaultsLimits(t *testing.T) {
	repo := &generateRedeemRepoStub{}
	svc := &adminServiceImpl{redeemCodeRepo: repo}

	_, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count:       1,
		Type:        RedeemTypeMultiUse,
		BenefitType: RedeemTypeConcurrency,
		Value:       2,
	})

	require.NoError(t, err)
	require.Len(t, repo.created, 1)
	require.Equal(t, 1, repo.created[0].MaxUsers)
	require.Equal(t, 1, repo.created[0].MaxUsesPerUser)
}

func TestGenerateRedeemCodesMultiUseRejectsInvalidInput(t *testing.T) {
	valid := func() *GenerateRedeemCodesInput {
		return &GenerateRedeemCodesInput{
			Count: 1, Type: RedeemTypeMultiUse, BenefitType: RedeemTypeBalance, Value: 5, MaxUsers: 10, MaxUsesPerUser: 2,
		}
	}
	tests := []struct {
		name       string
		mutate     func(*GenerateRedeemCodesInput)
		wantReason string
	}{
		{"missing benefit type", func(in *GenerateRedeemCodesInput) { in.BenefitType = "" }, "REDEEM_CODE_BENEFIT_TYPE_INVALID"},
		{"invitation benefit", func(in *GenerateRedeemCodesInput) { in.BenefitType = RedeemTypeInvitation }, "REDEEM_CODE_BENEFIT_TYPE_INVALID"},
		{"nested multi-use benefit", func(in *GenerateRedeemCodesInput) { in.BenefitType = RedeemTypeMultiUse }, "REDEEM_CODE_BENEFIT_TYPE_INVALID"},
		{"zero balance", func(in *GenerateRedeemCodesInput) { in.Value = 0 }, "REDEEM_CODE_VALUE_INVALID"},
		{"negative balance", func(in *GenerateRedeemCodesInput) { in.Value = -5 }, "REDEEM_CODE_VALUE_INVALID"},
		{
			"fractional concurrency",
			func(in *GenerateRedeemCodesInput) { in.BenefitType, in.Value = RedeemTypeConcurrency, 1.5 },
			"REDEEM_CODE_VALUE_INVALID",
		},
		{
			"negative concurrency",
			func(in *GenerateRedeemCodesInput) { in.BenefitType, in.Value = RedeemTypeConcurrency, -1 },
			"REDEEM_CODE_VALUE_INVALID",
		},
		{"negative max users", func(in *GenerateRedeemCodesInput) { in.MaxUsers = -1 }, "REDEEM_CODE_MAX_USERS_INVALID"},
		{
			"too many users",
			func(in *GenerateRedeemCodesInput) { in.MaxUsers = RedeemMultiUseMaxUsers + 1 },
			"REDEEM_CODE_MAX_USERS_INVALID",
		},
		{"negative uses per user", func(in *GenerateRedeemCodesInput) { in.MaxUsesPerUser = -1 }, "REDEEM_CODE_MAX_USES_PER_USER_INVALID"},
		{
			"too many uses per user",
			func(in *GenerateRedeemCodesInput) { in.MaxUsesPerUser = RedeemMultiUseMaxUsesPerUser + 1 },
			"REDEEM_CODE_MAX_USES_PER_USER_INVALID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &generateRedeemRepoStub{}
			svc := &adminServiceImpl{redeemCodeRepo: repo}
			input := valid()
			tt.mutate(input)

			codes, err := svc.GenerateRedeemCodes(context.Background(), input)

			require.Nil(t, codes)
			require.Error(t, err)
			require.True(t, infraerrors.IsBadRequest(err))
			require.Equal(t, tt.wantReason, infraerrors.Reason(err))
			require.Empty(t, repo.created)
		})
	}
}

// 发放订阅的多次兑换码与订阅兑换码一样必须指定分组。
func TestGenerateRedeemCodesMultiUseSubscriptionRequiresGroup(t *testing.T) {
	repo := &generateRedeemRepoStub{}
	svc := &adminServiceImpl{redeemCodeRepo: repo}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count: 1, Type: RedeemTypeMultiUse, BenefitType: RedeemTypeSubscription, MaxUsers: 10, MaxUsesPerUser: 1,
	})

	require.Nil(t, codes)
	require.ErrorContains(t, err, "group_id is required")
	require.Empty(t, repo.created)
}

// 一次性兑换码的生成不受影响：不带多次兑换码字段，负面值（退款扣减）仍然允许。
func TestGenerateRedeemCodesSingleUseUnchanged(t *testing.T) {
	repo := &generateRedeemRepoStub{}
	svc := &adminServiceImpl{redeemCodeRepo: repo}

	_, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count: 1, Type: RedeemTypeBalance, Value: -5,
		// 非多次兑换码类型会忽略这些字段。
		BenefitType: RedeemTypeConcurrency, MaxUsers: 50, MaxUsesPerUser: 5,
	})

	require.NoError(t, err)
	require.Len(t, repo.created, 1)
	require.Equal(t, RedeemTypeBalance, repo.created[0].Type)
	require.Equal(t, -5.0, repo.created[0].Value)
	require.Empty(t, repo.created[0].BenefitType)
	require.Zero(t, repo.created[0].MaxUsers)
	require.Zero(t, repo.created[0].MaxUsesPerUser)
}
