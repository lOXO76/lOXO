package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// CheckInHandler 管理端签到统计与签到明细。
type CheckInHandler struct {
	checkInService *service.CheckInService
}

func NewCheckInHandler(checkInService *service.CheckInService) *CheckInHandler {
	return &CheckInHandler{checkInService: checkInService}
}

// GetStats GET /admin/check-ins/stats?days=30
func (h *CheckInHandler) GetStats(c *gin.Context) {
	days, _ := strconv.Atoi(c.Query("days"))
	stats, err := h.checkInService.GetAdminStats(c.Request.Context(), days)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, stats)
}

// ListRecords GET /admin/check-ins/records?search=&start_date=&end_date=&page=&page_size=&sort_by=&sort_order=
func (h *CheckInHandler) ListRecords(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	filter := service.CheckInRecordFilter{
		Search:    c.Query("search"),
		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.Query("sort_by"),
		SortDesc:  c.Query("sort_order") != "asc",
	}
	items, total, err := h.checkInService.ListAdminRecords(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}
