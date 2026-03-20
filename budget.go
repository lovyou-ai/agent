package agent

import (
	"fmt"

	"github.com/lovyou-ai/eventgraph/go/pkg/event"
)

// EmitBudgetAllocated records a budget allocation event on the graph.
func (a *Agent) EmitBudgetAllocated(maxTokens int, maxCostUSD float64) error {
	ev, err := a.record(event.EventTypeAgentBudgetAllocated.Value(), event.AgentBudgetAllocatedContent{
		AgentID:    a.runtime.ID(),
		TokenLimit: maxTokens,
		CostLimit:  maxCostUSD,
	})
	if err != nil {
		return fmt.Errorf("budget allocated: %w", err)
	}

	a.mu.Lock()
	a.lastEvent = ev.ID()
	a.mu.Unlock()

	return nil
}

// EmitBudgetExhausted records that a budget limit has been reached.
func (a *Agent) EmitBudgetExhausted(resource string) error {
	ev, err := a.record(event.EventTypeAgentBudgetExhausted.Value(), event.AgentBudgetExhaustedContent{
		AgentID:  a.runtime.ID(),
		Resource: resource,
	})
	if err != nil {
		return fmt.Errorf("budget exhausted: %w", err)
	}

	a.mu.Lock()
	a.lastEvent = ev.ID()
	a.mu.Unlock()

	return nil
}
