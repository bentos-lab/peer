package router

import (
	"context"

	"github.com/bentos-lab/peer/domain"
	"github.com/bentos-lab/peer/usecase"
)

// ReviewPublisher routes review output to comment or print publishers.
type ReviewPublisher struct {
	Print   usecase.ReviewResultPublisher
	Comment usecase.ReviewResultPublisher
}

// NewReviewPublisher creates a routing review publisher.
func NewReviewPublisher(printPublisher usecase.ReviewResultPublisher, commentPublisher usecase.ReviewResultPublisher) *ReviewPublisher {
	return &ReviewPublisher{Print: printPublisher, Comment: commentPublisher}
}

// Publish routes to comment publisher when PR number is present; otherwise print.
func (p *ReviewPublisher) Publish(ctx context.Context, result usecase.ReviewPublishResult) error {
	publisher := selectPublisher(result.Target, p.Print, p.Comment)
	if publisher == nil {
		return nil
	}
	return publisher.Publish(ctx, result)
}

// OverviewPublisher routes overview output to comment or print publishers.
type OverviewPublisher struct {
	Print   usecase.OverviewPublisher
	Comment usecase.OverviewPublisher
}

// NewOverviewPublisher creates a routing overview publisher.
func NewOverviewPublisher(printPublisher usecase.OverviewPublisher, commentPublisher usecase.OverviewPublisher) *OverviewPublisher {
	return &OverviewPublisher{Print: printPublisher, Comment: commentPublisher}
}

// PublishOverview routes to comment publisher when PR number is present; otherwise print.
func (p *OverviewPublisher) PublishOverview(ctx context.Context, req usecase.OverviewPublishRequest) error {
	publisher := selectPublisher(req.Target, p.Print, p.Comment)
	if publisher == nil {
		return nil
	}
	return publisher.PublishOverview(ctx, req)
}

func selectPublisher[T any](target domain.ChangeRequestTarget, printPublisher T, commentPublisher T) T {
	var zero T
	if target.ChangeRequestNumber > 0 {
		if any(commentPublisher) != nil {
			return commentPublisher
		}
	}
	if any(printPublisher) != nil {
		return printPublisher
	}
	return zero
}
