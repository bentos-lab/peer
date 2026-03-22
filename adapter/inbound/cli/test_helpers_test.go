package cli

import (
	"context"
	"strings"

	"github.com/bentos-lab/peer/domain"
	uccontracts "github.com/bentos-lab/peer/usecase/contracts"
)

func containsLogEvent(events []string, needle string) bool {
	for _, event := range events {
		if strings.Contains(event, needle) {
			return true
		}
	}
	return false
}

type testCodeEnvironmentFactory struct{}

func (f *testCodeEnvironmentFactory) New(_ context.Context, _ domain.CodeEnvironmentInitOptions) (uccontracts.CodeEnvironment, error) {
	return &testCodeEnvironment{}, nil
}

type testCodeEnvironment struct{}

func (e *testCodeEnvironment) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (uccontracts.CodingAgent, error) {
	return &testCodingAgent{}, nil
}

func (e *testCodeEnvironment) LoadChangedFiles(_ context.Context, _ domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	return nil, nil
}

func (e *testCodeEnvironment) EnsureDiffContentAvailable(_ context.Context, _ domain.CodeEnvironmentLoadOptions) error {
	return nil
}

func (e *testCodeEnvironment) ReadFile(_ context.Context, _ string, _ string) (string, bool, error) {
	return "", false, nil
}

func (e *testCodeEnvironment) CommitChanges(_ context.Context, _ domain.CodeEnvironmentCommitOptions) (domain.CodeEnvironmentCommitResult, error) {
	return domain.CodeEnvironmentCommitResult{}, nil
}

func (e *testCodeEnvironment) PushChanges(_ context.Context, _ domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	return domain.CodeEnvironmentPushResult{}, nil
}

func (e *testCodeEnvironment) Cleanup(_ context.Context) error {
	return nil
}

type testCodingAgent struct{}

func (a *testCodingAgent) Run(_ context.Context, _ string, _ domain.CodingAgentRunOptions) (domain.CodingAgentRunResult, error) {
	return domain.CodingAgentRunResult{}, nil
}

type testRecipeLoader struct{}

func (t *testRecipeLoader) Load(_ context.Context, _ uccontracts.CodeEnvironment, _ string) (domain.CustomRecipe, error) {
	return domain.CustomRecipe{}, nil
}
