package cli

import (
	"bytes"
	"context"
	"testing"

	"bentos-backend/domain"
	"bentos-backend/usecase"
	"github.com/stretchr/testify/require"
)

func TestOverviewPublisher_PublishOverview_PrintsReadableOutput(t *testing.T) {
	var buffer bytes.Buffer
	publisher := NewOverviewPublisher(&buffer)

	err := publisher.PublishOverview(context.Background(), usecase.OverviewPublishRequest{
		Overview: usecase.LLMOverviewResult{
			Categories: []domain.OverviewCategoryItem{
				{Category: domain.OverviewCategoryRefactoring, Summary: "Simplified service boundaries."},
			},
			Walkthroughs: []domain.OverviewWalkthrough{
				{GroupName: "Service", Files: []string{"service.go", "repo.go"}, Summary: "Split persistence and orchestration."},
			},
		},
		IssueAlignment: &domain.IssueAlignmentResult{
			Issue:    domain.IssueReference{Number: 12, Title: "Reduce coupling"},
			KeyIdeas: []string{"Decouple repo access"},
			Requirements: []domain.IssueAlignmentRequirement{
				{Requirement: "Decouple repo access", Coverage: "Partial - only read path updated"},
			},
		},
	})
	require.NoError(t, err)
	require.Contains(t, buffer.String(), "Overview")
	require.Contains(t, buffer.String(), "Summary")
	require.Contains(t, buffer.String(), "Walkthroughs")
	require.Contains(t, buffer.String(), "service.go")
	require.Contains(t, buffer.String(), "Issue Alignment")
	require.Contains(t, buffer.String(), "Linked issue: #12 - Reduce coupling")
	require.Contains(t, buffer.String(), "Requirement | Coverage")
}
