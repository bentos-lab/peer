package router

import (
	"context"
	"fmt"

	"bentos-backend/usecase"
)

// AutogenPublisher routes autogen output to comment or print publishers.
type AutogenPublisher struct {
	Print   usecase.AutogenPublisher
	Comment usecase.AutogenPublisher
}

// NewAutogenPublisher creates a routing autogen publisher.
func NewAutogenPublisher(printPublisher usecase.AutogenPublisher, commentPublisher usecase.AutogenPublisher) *AutogenPublisher {
	return &AutogenPublisher{Print: printPublisher, Comment: commentPublisher}
}

// PublishAutogen routes to the comment publisher when Publish is true; otherwise print.
func (p *AutogenPublisher) PublishAutogen(ctx context.Context, req usecase.AutogenPublishRequest) error {
	if req.Publish {
		if p.Comment != nil {
			return p.Comment.PublishAutogen(ctx, req)
		}
		return fmt.Errorf("autogen comment publisher is not configured")
	}
	if p.Print != nil {
		return p.Print.PublishAutogen(ctx, req)
	}
	return fmt.Errorf("autogen print publisher is not configured")
}
