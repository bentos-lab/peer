package codingagent

import (
	"context"
	"fmt"
	"strings"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/usecase/contracts"
)

func (r *Reviewer) setupAgent(ctx context.Context, environment contracts.CodeEnvironment, head string) (contracts.CodingAgent, error) {
	agent, err := environment.SetupAgent(ctx, domain.CodingAgentSetupOptions{
		Agent: r.config.Agent,
		Ref:   strings.TrimSpace(head),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to setup coding agent: %w", err)
	}
	return agent, nil
}
