package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"bentos-backend/usecase"
)

// OverviewPublisher writes overview output to an output stream.
type OverviewPublisher struct {
	writer io.Writer
}

// NewOverviewPublisher creates a CLI overview publisher.
func NewOverviewPublisher(writer io.Writer) *OverviewPublisher {
	return &OverviewPublisher{writer: writer}
}

// PublishOverview prints overview sections for CLI mode.
func (p *OverviewPublisher) PublishOverview(_ context.Context, req usecase.OverviewPublishRequest) error {
	if _, err := fmt.Fprintln(p.writer, "Overview"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(p.writer, ""); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(p.writer, "Summary"); err != nil {
		return err
	}
	if len(req.Overview.Categories) == 0 {
		if _, err := fmt.Fprintln(p.writer, "- No significant high-level changes identified."); err != nil {
			return err
		}
	} else {
		for _, category := range req.Overview.Categories {
			if _, err := fmt.Fprintf(p.writer, "- %s: %s\n", category.Category, category.Summary); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintln(p.writer, ""); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(p.writer, "Walkthroughs"); err != nil {
		return err
	}
	if len(req.Overview.Walkthroughs) == 0 {
		if _, err := fmt.Fprintln(p.writer, "- No grouped walkthroughs generated."); err != nil {
			return err
		}
		return nil
	}

	for _, walkthrough := range req.Overview.Walkthroughs {
		if _, err := fmt.Fprintf(p.writer, "- %s\n", walkthrough.GroupName); err != nil {
			return err
		}
		if len(walkthrough.Files) > 0 {
			if _, err := fmt.Fprintf(p.writer, "  Files: %s\n", strings.Join(walkthrough.Files, ", ")); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(p.writer, "  %s\n", walkthrough.Summary); err != nil {
			return err
		}
	}
	return nil
}
