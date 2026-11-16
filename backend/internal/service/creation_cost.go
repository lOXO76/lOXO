package service

import (
	"context"
	"strings"
)

// CreationCostRepository reads settled usage only. Missing entries differ from
// recorded zero-cost requests and must remain absent from the response.
type CreationCostRepository interface {
	SumUserRequestCosts(context.Context, int64, []string) (map[string]float64, error)
	SumUserTaskCosts(context.Context, int64, []AsyncTaskRecord) (map[string]float64, error)
}

type CreationCost struct {
	RequestID  string  `json:"request_id"`
	ActualCost float64 `json:"actual_cost"`
}

type CreationCosts struct {
	Items []CreationCost `json:"items"`
}

type CreationCostService struct {
	repo CreationCostRepository
}

func NewCreationCostService(repo CreationCostRepository) *CreationCostService {
	return &CreationCostService{repo: repo}
}

// GetCosts accepts raw X-Client-Request-ID values. Both native Gemini image
// generation and OpenAI Images use resolveUsageBillingRequestID's client key.
func (s *CreationCostService) GetCosts(ctx context.Context, userID int64, requestIDs []string) (*CreationCosts, error) {
	billingIDs := make([]string, len(requestIDs))
	for i, id := range requestIDs {
		billingIDs[i] = "client:" + strings.TrimSpace(id)
	}
	costs, err := s.repo.SumUserRequestCosts(ctx, userID, billingIDs)
	if err != nil {
		return nil, err
	}
	result := &CreationCosts{Items: make([]CreationCost, 0, len(requestIDs))}
	for i, id := range requestIDs {
		if cost, ok := costs[billingIDs[i]]; ok {
			result.Items = append(result.Items, CreationCost{RequestID: id, ActualCost: cost})
		}
	}
	return result, nil
}

func (s *CreationCostService) populateTasks(ctx context.Context, userID int64, records []AsyncTaskRecord) error {
	if len(records) == 0 {
		return nil
	}
	costs, err := s.repo.SumUserTaskCosts(ctx, userID, records)
	if err != nil {
		return err
	}
	for i := range records {
		records[i].ActualCost = nil
		if cost, ok := costs[records[i].ID]; ok && records[i].UserID == userID {
			records[i].ActualCost = &cost
		}
	}
	return nil
}
