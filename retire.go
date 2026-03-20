package agent

import (
	"context"
	"fmt"

	egagent "github.com/lovyou-ai/eventgraph/go/pkg/agent"
	"github.com/lovyou-ai/eventgraph/go/pkg/event"
)

// Retire gracefully shuts down the agent.
// Follows the Retire composition: Introspect → Communicate (farewell) →
// Memory (archive) → Lifespan (end).
// Transitions: current state → Retiring → Retired.
func (a *Agent) Retire(ctx context.Context, reason string) error {
	if err := a.transitionTo(egagent.StateRetiring); err != nil {
		return fmt.Errorf("retire: %w", err)
	}

	// Introspect: final self-observation.
	introEv, _, err := a.runtime.Introspect(ctx, "Final introspection before retirement: "+reason)
	if err == nil {
		a.mu.Lock()
		a.lastEvent = introEv.ID()
		a.mu.Unlock()
	}

	// Communicate: farewell message.
	farewellEv, err := a.record(event.EventTypeAgentCommunicated.Value(), event.AgentCommunicatedContent{
		AgentID:   a.runtime.ID(),
		Recipient: a.runtime.ID(), // farewell to all — self-addressed
		Channel:   fmt.Sprintf("Agent %s (%s) retiring: %s", a.name, a.role, reason),
	})
	if err == nil {
		a.mu.Lock()
		a.lastEvent = farewellEv.ID()
		a.mu.Unlock()
	}

	// Lifespan: end.
	lifespanEv, err := a.record(event.EventTypeAgentLifespanEnded.Value(), event.AgentLifespanEndedContent{
		AgentID: a.runtime.ID(),
		Reason:  reason,
	})
	if err == nil {
		a.mu.Lock()
		a.lastEvent = lifespanEv.ID()
		a.mu.Unlock()
	}

	// Transition to Retired (terminal state).
	if err := a.transitionTo(egagent.StateRetired); err != nil {
		return fmt.Errorf("retire: final transition: %w", err)
	}

	// Update actor store: memorial.
	if !a.lastEvent.IsZero() {
		_, _ = a.graph.ActorStore().Memorial(a.runtime.ID(), a.lastEvent)
	}

	return nil
}
