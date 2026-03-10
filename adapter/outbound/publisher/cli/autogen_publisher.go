package cli

import (
	"context"
	"fmt"
	"io"

	"bentos-backend/usecase"
)

// AutogenPublisher writes autogen output to an output stream.
type AutogenPublisher struct {
	writer io.Writer
}

// NewAutogenPublisher creates a CLI autogen publisher.
func NewAutogenPublisher(writer io.Writer) *AutogenPublisher {
	return &AutogenPublisher{writer: writer}
}

// PublishAutogen prints added content with file paths and line ranges.
func (p *AutogenPublisher) PublishAutogen(_ context.Context, req usecase.AutogenPublishRequest) error {
	for _, change := range req.Changes {
		if _, err := fmt.Fprintf(p.writer, "path: %s\n", change.FilePath); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(p.writer, "lines: %d-%d\n", change.StartLine, change.EndLine); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(p.writer, "%s\n\n", change.Content); err != nil {
			return err
		}
	}
	return nil
}
