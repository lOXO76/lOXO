package handler

import (
	"encoding/json"
	"io"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type RewardTaskOpenHandler struct {
	service *service.RewardTaskService
}

func NewRewardTaskOpenHandler(rewardTaskService *service.RewardTaskService) *RewardTaskOpenHandler {
	return &RewardTaskOpenHandler{service: rewardTaskService}
}

func readRewardOpenBody(c *gin.Context) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, service.RewardOpenMaxBodyBytes+1))
	if err != nil || len(body) > service.RewardOpenMaxBodyBytes {
		return body, infraerrors.BadRequest("REWARD_EVENT_INVALID", "invalid request body (maximum 64KB)")
	}
	return body, nil
}

func (h *RewardTaskOpenHandler) authenticate(c *gin.Context, body []byte) (*dbent.RewardTaskPartner, error) {
	return h.service.AuthenticateRewardOpen(c.Request.Context(), service.RewardOpenSignature{
		AppKey: c.GetHeader("X-Reward-App-Key"), Timestamp: c.GetHeader("X-Reward-Timestamp"),
		Nonce: c.GetHeader("X-Reward-Nonce"), Signature: c.GetHeader("X-Reward-Signature"),
	}, body)
}

func (h *RewardTaskOpenHandler) Ping(c *gin.Context) {
	body, err := readRewardOpenBody(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	partner, err := h.authenticate(c, body)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.service.OpenPing(c.Request.Context(), partner)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *RewardTaskOpenHandler) QueryTicket(c *gin.Context) {
	body, err := readRewardOpenBody(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	partner, err := h.authenticate(c, body)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var input struct {
		Ticket string `json:"ticket"`
	}
	if json.Unmarshal(body, &input) != nil || input.Ticket == "" || len(input.Ticket) > 64 {
		response.ErrorFrom(c, infraerrors.BadRequest("REWARD_EVENT_INVALID", "ticket is required"))
		return
	}
	result, err := h.service.QueryRewardOpenTicket(c.Request.Context(), partner.ID, input.Ticket)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *RewardTaskOpenHandler) Event(c *gin.Context) {
	body, err := readRewardOpenBody(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// 验签失败不落库：app_key 公开在合作方页面的 SDK 标签里，若记录未认证请求，
	// 任何人都能往事件表灌数据、伪造某个站点的回调日志。对方靠响应里的 reason 自助排查即可。
	partner, err := h.authenticate(c, body)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.service.ProcessRewardOpenEvent(c.Request.Context(), partner.ID, body, c.ClientIP())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *RewardTaskOpenHandler) Heartbeat(c *gin.Context) {
	body, err := readRewardOpenBody(c)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var input struct {
		AppKey string `json:"app_key"`
		Ticket string `json:"ticket"`
	}
	// text/plain 也按 JSON 处理，避免浏览器 SDK 发出不必要的预检。
	if json.Unmarshal(body, &input) != nil || input.AppKey == "" || len(input.AppKey) > 64 || input.Ticket == "" || len(input.Ticket) > 64 {
		response.ErrorFrom(c, infraerrors.BadRequest("REWARD_EVENT_INVALID", "app_key and ticket are required"))
		return
	}
	result, err := h.service.RewardSDKHeartbeat(c.Request.Context(), input.AppKey, input.Ticket, c.GetHeader("Origin"), c.GetHeader("Referer"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
