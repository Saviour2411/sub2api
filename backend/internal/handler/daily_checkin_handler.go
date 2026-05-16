package handler

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type DailyCheckinHandler struct {
	service *service.DailyCheckinService
}

func NewDailyCheckinHandler(service *service.DailyCheckinService) *DailyCheckinHandler {
	return &DailyCheckinHandler{service: service}
}

type DailyCheckinStatusResponse struct {
	Enabled        bool     `json:"enabled"`
	CheckedInToday bool     `json:"checked_in_today"`
	RewardMode     string   `json:"reward_mode"`
	RewardAmount   float64  `json:"reward_amount"`
	RewardMin      float64  `json:"reward_min"`
	RewardMax      float64  `json:"reward_max"`
	TodayReward    *float64 `json:"today_reward,omitempty"`
	CheckedInAt    *string  `json:"checked_in_at,omitempty"`
}

type DailyCheckinResponse struct {
	RewardAmount float64 `json:"reward_amount"`
	NewBalance   float64 `json:"new_balance"`
	CheckedInAt  string  `json:"checked_in_at"`
}

func (h *DailyCheckinHandler) GetStatus(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	status, err := h.service.GetStatus(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var checkedInAt *string
	if status.CheckedInAt != nil {
		value := status.CheckedInAt.Format(time.RFC3339)
		checkedInAt = &value
	}
	response.Success(c, DailyCheckinStatusResponse{
		Enabled:        status.Enabled,
		CheckedInToday: status.CheckedInToday,
		RewardMode:     status.RewardMode,
		RewardAmount:   status.RewardAmount,
		RewardMin:      status.RewardMin,
		RewardMax:      status.RewardMax,
		TodayReward:    status.TodayReward,
		CheckedInAt:    checkedInAt,
	})
}

func (h *DailyCheckinHandler) Checkin(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	result, err := h.service.Checkin(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, DailyCheckinResponse{
		RewardAmount: result.RewardAmount,
		NewBalance:   result.NewBalance,
		CheckedInAt:  result.CheckedInAt.Format(time.RFC3339),
	})
}
