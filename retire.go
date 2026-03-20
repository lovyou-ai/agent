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
//
// All events are emitted via graph.Record() for bus visibility.
// Resolves any mid-operation state (Escalating, Refusing, Waiting) back
// to Idle before beginning the retirement sequence — atomically under
// a.mu to prevent TOCTOU races.
//
// Introspection records the reason directly (no LLM call) to prevent
// indefinite hangs in the terminal StateRetiring.
//
// Transitions: current state → [Idle →] Retiring → Retired.
func (a *Agent) Retire(ctx context.Context, reason string) error {
	if err := a.resolveAndRetire(); err != nil {
		return fmt.Errorf("retire: %w", err)
	}

	// Introspect: record the retirement reason directly. No LLM call —
	// an unresponsive provider would block the agent in StateRetiring
	// indefinitely, violating the DIGNITY invariant (graceful shutdown).
	_, _ = a.recordAndTrack(event.EventTypeAgentIntrospected.Value(), event.AgentIntrospectedContent{
		AgentID:     a.runtime.ID(),
		Observation: "Retiring: " + reason,
	})

	// Communicate: farewell on the "lifecycle" channel.
	_, _ = a.recordAndTrack(event.EventTypeAgentCommunicated.Value(), event.AgentCommunicatedContent{
		AgentID:   a.runtime.ID(),
		Recipient: a.runtime.ID(),
		Channel:   "lifecycle",
	})

	// Memory: archive — record the retirement in the agent's memory.
	_, _ = a.recordAndTrack(event.EventTypeAgentMemoryUpdated.Value(), event.AgentMemoryUpdatedContent{
		AgentID: a.runtime.ID(),
		Key:     "retirement",
		Action:  "archive",
	})

	// Lifespan: end.
	_, _ = a.recordAndTrack(event.EventTypeAgentLifespanEnded.Value(), event.AgentLifespanEndedContent{
		AgentID: a.runtime.ID(),
		Reason:  reason,
	})

	// Transition to Retired (terminal state).
	if err := a.transitionTo(egagent.StateRetired); err != nil {
		return fmt.Errorf("retire: final transition: %w", err)
	}

	// Update actor store: memorial.
	a.mu.Lock()
	lastID := a.lastEvent
	a.mu.Unlock()
	if !lastID.IsZero() {
		_, _ = a.graph.ActorStore().Memorial(a.runtime.ID(), lastID)
	}

	return nil
}

// resolveAndRetire atomically resolves mid-operation states and transitions
// to StateRetiring. Holds a.mu for the entire sequence to prevent TOCTOU
// races between state observation and transition.
func (a *Agent) resolveAndRetire() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.state == egagent.StateRetired {
		return fmt.Errorf("agent already retired")
	}

	// Resolve mid-operation states that can't transition directly to Retiring.
	switch a.state {
	case egagent.StateEscalating, egagent.StateRefusing, egagent.StateWaiting:
		if err := a.transitionLocked(egagent.StateIdle); err != nil {
			return fmt.Errorf("resolve %s: %w", a.state, err)
		}
	}

	return a.transitionLocked(egagent.StateRetiring)
}
