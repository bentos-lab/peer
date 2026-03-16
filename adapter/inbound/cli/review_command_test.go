package cli

import (
	"context"
	"fmt"
	"testing"
	"time"

	"bentos-backend/config"
	"bentos-backend/domain"
	"bentos-backend/usecase"

	"github.com/stretchr/testify/require"
)

func TestReviewCommandRunLogsPreUsecaseSnapshotWithoutSensitiveData(t *testing.T) {
	reviewUC := &fakeReviewUseCase{}
	githubClient := &fakeGitHubClient{
		resolvedRepository: "owner/repo",
		pullRequestInfo: domain.ChangeRequestInfo{
			Repository:  "owner/repo",
			BaseRef:     "main",
			HeadRef:     "feature/ref",
			Title:       "Super secret PR title",
			Description: "Confidential PR description",
		},
	}
	logger := &commandTestSpyLogger{}
	builder := func(_ string) (usecase.ReviewUseCase, error) {
		return reviewUC, nil
	}
	resolver := StaticVCSClients{GitHub: githubClient}
	command := NewReviewCommand(builder, resolver, &testCodeEnvironmentFactory{}, &testRecipeLoader{}, logger)

	err := command.Run(context.Background(), config.Config{}, ReviewParams{
		Repo:          "https://github.com/owner/repo.git",
		ChangeRequest: "9",
		Publish:       true,
		Suggest:       new(true),
	})

	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return containsLogEvent(logger.events, "info:Pre-usecase input snapshot source=\"cli\" repository=\"owner/repo\" changeRequestNumber=9 suggestions=true.")
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return containsLogEvent(logger.events, "debug:Pre-usecase input details source=\"cli\" action=\"\" base=\"main\" head=\"feature/ref\"")
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return containsLogEvent(logger.events, "repoURLPresent=true repoURLSafe=\"https://github.com/owner/repo.git\"")
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		return containsLogEvent(logger.events, "titleLength=21 descriptionLength=27")
	}, time.Second, 10*time.Millisecond)
	require.False(t, containsLogEvent(logger.events, "Super secret PR title"))
	require.False(t, containsLogEvent(logger.events, "Confidential PR description"))
}

func TestReviewCommandRespectsBaseWhenHeadEmpty(t *testing.T) {
	reviewUC := &fakeReviewUseCase{}
	githubClient := &fakeGitHubClient{resolvedRepository: "owner/repo"}
	builder := func(_ string) (usecase.ReviewUseCase, error) {
		return reviewUC, nil
	}
	resolver := StaticVCSClients{GitHub: githubClient}
	command := NewReviewCommand(builder, resolver, &testCodeEnvironmentFactory{}, &testRecipeLoader{}, nil)

	err := command.Run(context.Background(), config.Config{}, ReviewParams{
		Base: "main",
		Head: "",
	})

	require.NoError(t, err)
	require.Len(t, reviewUC.requests, 1)
	require.Equal(t, "main", reviewUC.requests[0].Input.Base)
	require.Equal(t, "@all", reviewUC.requests[0].Input.Head)
}

type fakeReviewUseCase struct {
	requests []usecase.ReviewRequest
}

func (f *fakeReviewUseCase) Execute(_ context.Context, request usecase.ReviewRequest) (usecase.ReviewExecutionResult, error) {
	f.requests = append(f.requests, request)
	return usecase.ReviewExecutionResult{}, nil
}

type commandTestSpyLogger struct {
	events []string
}

func (s *commandTestSpyLogger) Tracef(format string, args ...any) {
	s.events = append(s.events, "trace:"+fmt.Sprintf(format, args...))
}

func (s *commandTestSpyLogger) Debugf(format string, args ...any) {
	s.events = append(s.events, "debug:"+fmt.Sprintf(format, args...))
}

func (s *commandTestSpyLogger) Infof(format string, args ...any) {
	s.events = append(s.events, "info:"+fmt.Sprintf(format, args...))
}

func (s *commandTestSpyLogger) Warnf(format string, args ...any) {
	s.events = append(s.events, "warn:"+fmt.Sprintf(format, args...))
}

func (s *commandTestSpyLogger) Errorf(format string, args ...any) {
	s.events = append(s.events, "error:"+fmt.Sprintf(format, args...))
}
