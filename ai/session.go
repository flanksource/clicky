package ai

import (
	"sync"
)

// Session tracks costs across multiple agent calls
type Session struct {
	ID          string
	ProjectName string
	Costs       Costs
	mutex       sync.RWMutex
}

// NewSession creates a new session for tracking costs
func NewSession(id, projectName string) *Session {
	return &Session{
		ID:          id,
		ProjectName: projectName,
		Costs:       Costs{},
	}
}

// AddCost adds a cost entry to the session in a thread-safe manner
func (s *Session) AddCost(cost Cost) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.Costs = append(s.Costs, cost)
}

// GetTotalCost returns the aggregated cost across all entries
func (s *Session) GetTotalCost() Cost {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.Costs.Sum()
}

// GetCostsByModel returns costs grouped by model
func (s *Session) GetCostsByModel() map[string]Cost {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.Costs.GetCostsByModel()
}
