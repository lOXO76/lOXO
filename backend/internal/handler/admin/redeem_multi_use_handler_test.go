package admin

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupRedeemMultiUseRouter() (*gin.Engine, *stubAdminService) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	adminSvc := newStubAdminService()

	h := NewRedeemHandler(adminSvc, nil)
	router.POST("/api/v1/admin/redeem-codes/generate", h.Generate)
	router.GET("/api/v1/admin/redeem-codes/export", h.Export)
	return router, adminSvc
}

func postGenerateRedeemCodes(t *testing.T, router *gin.Engine, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/redeem-codes/generate", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	return rec
}

// 一个多次兑换码及其一条兑换流水，用于检查管理端输出。
func multiUseRedeemFixtures() []service.RedeemCode {
	parentID := int64(7)
	usedBy := int64(42)
	return []service.RedeemCode{
		{
			ID: 7, Code: "MULTI-1", Type: service.RedeemTypeMultiUse, BenefitType: service.RedeemTypeBalance,
			Value: 5, Status: service.StatusUnused, MaxUsers: 100, MaxUsesPerUser: 3, UsedCount: 4, UsedUserCount: 2,
		},
		{
			ID: 8, Code: "REC-1", Type: service.RedeemTypeBalance, Value: 5, Status: service.StatusUsed,
			UsedBy: &usedBy, MaxUsers: 1, MaxUsesPerUser: 1, ParentID: &parentID, ParentCode: "MULTI-1",
		},
	}
}

func TestRedeemGeneratePassesMultiUseLimits(t *testing.T) {
	router, adminSvc := setupRedeemMultiUseRouter()
	adminSvc.redeems = multiUseRedeemFixtures()

	rec := postGenerateRedeemCodes(t, router, map[string]any{
		"count":             2,
		"type":              "multi_use",
		"value":             5,
		"benefit_type":      "balance",
		"max_users":         100,
		"max_uses_per_user": 3,
	})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	input := adminSvc.lastGenerateRedeemCodes
	require.NotNil(t, input)
	require.Equal(t, 2, input.Count)
	require.Equal(t, service.RedeemTypeMultiUse, input.Type)
	require.Equal(t, service.RedeemTypeBalance, input.BenefitType)
	require.Equal(t, 5.0, input.Value)
	require.Equal(t, 100, input.MaxUsers)
	require.Equal(t, 3, input.MaxUsesPerUser)

	var body struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Data, 2)

	parent := body.Data[0]
	require.Equal(t, "multi_use", parent["type"])
	require.Equal(t, "balance", parent["benefit_type"])
	require.EqualValues(t, 100, parent["max_users"])
	require.EqualValues(t, 3, parent["max_uses_per_user"])
	require.EqualValues(t, 4, parent["used_count"])
	require.EqualValues(t, 2, parent["used_user_count"])
	require.NotContains(t, parent, "parent_id")
	require.NotContains(t, parent, "parent_code")

	// 兑换流水是一条普通的已使用余额记录，只多出指向多次兑换码的引用。
	record := body.Data[1]
	require.Equal(t, "balance", record["type"])
	require.EqualValues(t, 7, record["parent_id"])
	require.Equal(t, "MULTI-1", record["parent_code"])
	for _, field := range []string{"benefit_type", "max_users", "max_uses_per_user", "used_count", "used_user_count"} {
		require.NotContains(t, record, field)
	}
}

func TestRedeemGenerateRejectsInvalidMultiUseFields(t *testing.T) {
	tests := []struct {
		name  string
		field string
		value any
	}{
		{"invitation benefit", "benefit_type", "invitation"},
		{"nested multi-use benefit", "benefit_type", "multi_use"},
		{"negative max users", "max_users", -1},
		{"too many users", "max_users", 1000001},
		{"negative uses per user", "max_uses_per_user", -1},
		{"too many uses per user", "max_uses_per_user", 1001},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, adminSvc := setupRedeemMultiUseRouter()
			body := map[string]any{
				"count":             1,
				"type":              "multi_use",
				"value":             5,
				"benefit_type":      "balance",
				"max_users":         10,
				"max_uses_per_user": 1,
			}
			body[tt.field] = tt.value

			rec := postGenerateRedeemCodes(t, router, body)

			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.Nil(t, adminSvc.lastGenerateRedeemCodes)
		})
	}
}

func TestRedeemGenerateSingleUseOmitsMultiUseFields(t *testing.T) {
	router, adminSvc := setupRedeemMultiUseRouter()

	rec := postGenerateRedeemCodes(t, router, map[string]any{"count": 1, "type": "balance", "value": 10})

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	input := adminSvc.lastGenerateRedeemCodes
	require.NotNil(t, input)
	require.Equal(t, service.RedeemTypeBalance, input.Type)
	require.Empty(t, input.BenefitType)
	require.Zero(t, input.MaxUsers)
	require.Zero(t, input.MaxUsesPerUser)
}

// 多次兑换码的列追加在 CSV 末尾，既有列的位置不变。
func TestRedeemExportAppendsMultiUseColumns(t *testing.T) {
	router, adminSvc := setupRedeemMultiUseRouter()
	adminSvc.redeems = multiUseRedeemFixtures()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/redeem-codes/export", nil)
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	rows, err := csv.NewReader(rec.Body).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 3)
	require.Equal(t, []string{
		"id", "code", "type", "value", "status", "used_by", "used_by_email", "used_at", "expires_at", "created_at",
		"benefit_type", "max_users", "max_uses_per_user", "used_count", "used_user_count", "parent_code",
	}, rows[0])

	require.Equal(t, []string{"7", "MULTI-1", "multi_use", "5.00", "unused"}, rows[1][:5])
	require.Equal(t, []string{"balance", "100", "3", "4", "2", ""}, rows[1][10:])

	require.Equal(t, []string{"8", "REC-1", "balance", "5.00", "used", "42"}, rows[2][:6])
	require.Equal(t, []string{"", "1", "1", "0", "0", "MULTI-1"}, rows[2][10:])
}
