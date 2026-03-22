package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/shared/logger/stdlogger"
)

// reviewUseCase is the concrete ReviewUseCase implementation.
type reviewUseCase struct {
	llmReviewer LLMReviewer
	publisher   ReviewResultPublisher
	logger      Logger
}

// NewReviewUseCase constructs a review-only usecase.
func NewReviewUseCase(
	llmReviewer LLMReviewer,
	publisher ReviewResultPublisher,
	logger Logger,
) (ReviewUseCase, error) {
	if llmReviewer == nil || publisher == nil {
		return nil, errors.New("review usecase dependencies must not be nil")
	}
	if logger == nil {
		logger = stdlogger.Nop()
	}
	u := &reviewUseCase{
		llmReviewer: llmReviewer,
		publisher:   publisher,
		logger:      logger,
	}
	return u, nil
}

// Execute runs the review flow and publishes review messages.
func (u *reviewUseCase) Execute(ctx context.Context, request ReviewRequest) (ReviewExecutionResult, error) {
	startedAt := time.Now()
	target := request.Input.Target
	logExecution(u.logger, "review", target, "start", startedAt, "")

	if request.Environment == nil {
		return ReviewExecutionResult{}, errors.New("code environment is required")
	}

	reviewDiffStartedAt := time.Now()
	llmResult, err := u.llmReviewer.Review(ctx, LLMReviewPayload{
		Input:         request.Input,
		Environment:   request.Environment,
		Suggestions:   request.Suggestions,
		CustomRuleset: strings.TrimSpace(request.Recipe.ReviewRuleset),
	})
	if err != nil {
		logStage(u.logger, "review", "review_diff", target, "failure", reviewDiffStartedAt, "%v", err)
		return ReviewExecutionResult{}, err
	}
	logStage(u.logger, "review", "review_diff", target, "success", reviewDiffStartedAt, "")
	u.logger.Debugf("The LLM review produced %d findings.", len(llmResult.Findings))

	filteredFindings := filterNonNitFindings(llmResult.Findings)
	if len(filteredFindings) == 0 {
		u.logger.Debugf("skipped_publish_no_findings: no non-NIT findings remained.")
		return ReviewExecutionResult{
			Messages: []domain.ReviewMessage{},
			Findings: []domain.Finding{},
			Summary:  "",
		}, nil
	}

	publishStartedAt := time.Now()
	messages := BuildMessages(filteredFindings, llmResult.Summary)
	publishInput := ReviewPublishResult{
		Target:         request.Input.Target,
		Messages:       messages,
		Findings:       filteredFindings,
		Summary:        llmResult.Summary,
		RecipeWarnings: request.Recipe.MissingPaths,
	}
	if err := u.publisher.Publish(ctx, publishInput); err != nil {
		logStage(u.logger, "review", "publish_review_result", target, "failure", publishStartedAt, "%v", err)
		return ReviewExecutionResult{}, err
	}
	logStage(u.logger, "review", "publish_review_result", target, "success", publishStartedAt, "")
	u.logger.Debugf("Published %d review messages.", len(messages))

	logExecution(
		u.logger,
		"review",
		target,
		"complete",
		startedAt,
		"Full execution took %d ms and produced %d findings with %d messages.",
		time.Since(startedAt).Milliseconds(),
		len(filteredFindings),
		len(messages),
	)

	return ReviewExecutionResult{
		Messages: messages,
		Findings: filteredFindings,
		Summary:  llmResult.Summary,
	}, nil
}

func filterNonNitFindings(findings []domain.Finding) []domain.Finding {
	filtered := make([]domain.Finding, 0, len(findings))
	for _, finding := range findings {
		if finding.Severity == domain.FindingSeverityNit {
			continue
		}
		filtered = append(filtered, finding)
	}
	return filtered
}
