package router

import (
	"context"

	"bentos-backend/usecase"
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
	if result.Target.ChangeRequestNumber > 0 && p.Comment != nil {
		return p.Comment.Publish(ctx, result)
	}
	if p.Print != nil {
		return p.Print.Publish(ctx, result)
	}
	return nil
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
	if req.Target.ChangeRequestNumber > 0 && p.Comment != nil {
		return p.Comment.PublishOverview(ctx, req)
	}
	if p.Print != nil {
		return p.Print.PublishOverview(ctx, req)
	}
	return nil
}
