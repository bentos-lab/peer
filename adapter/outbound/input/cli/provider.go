package cli

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"

	"bentos-backend/domain"
	"bentos-backend/usecase"
)

const (
	// MetadataKeyChangedFiles stores manually supplied file paths.
	MetadataKeyChangedFiles = "changed_files"
	// MetadataKeyAutoIncludeAll enables staged+unstaged auto discovery.
	MetadataKeyAutoIncludeAll = "auto_include_unstaged"
	// MetadataKeyAutoIncludeUntracked enables untracked file discovery.
	MetadataKeyAutoIncludeUntracked = "auto_include_untracked"
)

var errNoChangedFiles = errors.New("no changed files to review")

// ChangeDetector discovers changed paths from the local git workspace.
type ChangeDetector interface {
	// ListStaged returns staged changed file paths.
	ListStaged(ctx context.Context) ([]string, error)
	// ListUnstaged returns unstaged changed file paths.
	ListUnstaged(ctx context.Context) ([]string, error)
	// ListUntracked returns untracked file paths.
	ListUntracked(ctx context.Context) ([]string, error)
	// GetDiffForPath returns unified diff content for one file path.
	GetDiffForPath(ctx context.Context, path string) (string, error)
}

// Provider loads review input from local files for CLI mode.
type Provider struct {
	changeDetector ChangeDetector
}

// NewProvider creates a CLI review input provider.
func NewProvider(changeDetector ChangeDetector) *Provider {
	return &Provider{changeDetector: changeDetector}
}

// LoadChangeSnapshot loads changed content from metadata file paths.
func (p *Provider) LoadChangeSnapshot(ctx context.Context, request usecase.ChangeRequestRequest) (domain.ChangeSnapshot, error) {
	pathsCSV := strings.TrimSpace(request.Metadata[MetadataKeyChangedFiles])
	paths := splitCSV(pathsCSV)
	autoMode := pathsCSV == ""
	untrackedSet := map[string]struct{}{}
	if autoMode {
		autoPaths, err := p.detectAutoPaths(ctx, request.Metadata)
		if err != nil {
			return domain.ChangeSnapshot{}, err
		}
		paths = autoPaths
	}
	if p.changeDetector != nil {
		untrackedPaths, err := p.changeDetector.ListUntracked(ctx)
		if err != nil {
			return domain.ChangeSnapshot{}, err
		}
		untrackedSet = toSet(untrackedPaths)
	}

	files := make([]domain.ChangedFile, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			if autoMode && os.IsNotExist(err) {
				continue
			}
			return domain.ChangeSnapshot{}, err
		}

		diffSnippet := ""
		if p.changeDetector != nil {
			diffSnippet, err = p.changeDetector.GetDiffForPath(ctx, path)
			if err != nil {
				return domain.ChangeSnapshot{}, err
			}
		}
		if strings.TrimSpace(diffSnippet) == "" {
			if _, ok := untrackedSet[path]; ok {
				diffSnippet = buildSyntheticNewFileDiff(path, string(raw))
			}
		}
		files = append(files, domain.ChangedFile{
			Path:        path,
			Content:     string(raw),
			DiffSnippet: diffSnippet,
		})
	}
	if autoMode && len(files) == 0 {
		return domain.ChangeSnapshot{}, errNoChangedFiles
	}

	return domain.ChangeSnapshot{
		Context: domain.ChangeRequestContext{
			Repository:          request.Repository,
			ChangeRequestNumber: request.ChangeRequestNumber,
			Title:               request.Title,
			Description:         request.Description,
			Metadata:            request.Metadata,
		},
		ChangedFiles: files,
		Language:     "English",
	}, nil
}

func (p *Provider) detectAutoPaths(ctx context.Context, metadata map[string]string) ([]string, error) {
	if p.changeDetector == nil {
		return nil, errors.New("change detector is required")
	}

	paths, err := p.changeDetector.ListStaged(ctx)
	if err != nil {
		return nil, err
	}

	includeUnstaged, _ := strconv.ParseBool(metadata[MetadataKeyAutoIncludeAll])
	if includeUnstaged {
		unstaged, unstagedErr := p.changeDetector.ListUnstaged(ctx)
		if unstagedErr != nil {
			return nil, unstagedErr
		}
		paths = append(paths, unstaged...)
	}

	includeUntracked, _ := strconv.ParseBool(metadata[MetadataKeyAutoIncludeUntracked])
	if includeUntracked {
		untracked, untrackedErr := p.changeDetector.ListUntracked(ctx)
		if untrackedErr != nil {
			return nil, untrackedErr
		}
		paths = append(paths, untracked...)
	}

	return dedupe(paths), nil
}

func splitCSV(csv string) []string {
	rawItems := strings.Split(csv, ",")
	paths := make([]string, 0, len(rawItems))
	for _, item := range rawItems {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		paths = append(paths, item)
	}
	return paths
}

func dedupe(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	return result
}

func toSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result[trimmed] = struct{}{}
	}
	return result
}

func buildSyntheticNewFileDiff(path string, content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	lineCount := len(lines)
	if lineCount == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("@@ -0,0 +1,")
	builder.WriteString(strconv.Itoa(lineCount))
	builder.WriteString(" @@\n")
	for _, line := range lines {
		builder.WriteString("+")
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	return strings.TrimRight(builder.String(), "\n")
}
