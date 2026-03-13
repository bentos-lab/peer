package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"bentos-backend/domain"
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
	if err := printIssueAlignment(p.writer, req.IssueAlignment); err != nil {
		return err
	}
	return nil
}

func printIssueAlignment(writer io.Writer, alignment *domain.IssueAlignmentResult) error {
	if alignment == nil || len(alignment.Requirements) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(writer, ""); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, "Issue Alignment"); err != nil {
		return err
	}
	if strings.TrimSpace(alignment.Issue.Title) == "" {
		if _, err := fmt.Fprintf(writer, "Linked issue: #%d\n", alignment.Issue.Number); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(writer, "Linked issue: #%d - %s\n", alignment.Issue.Number, alignment.Issue.Title); err != nil {
			return err
		}
	}
	if strings.TrimSpace(alignment.Issue.Repository) != "" {
		if _, err := fmt.Fprintf(writer, "Repository: %s\n", alignment.Issue.Repository); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(writer, "Requirement | Coverage"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, "--- | ---"); err != nil {
		return err
	}
	for _, row := range alignment.Requirements {
		if _, err := fmt.Fprintf(writer, "%s | %s\n", row.Requirement, row.Coverage); err != nil {
			return err
		}
	}
	return nil
}
