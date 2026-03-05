package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"bentos-backend/domain"
)

type pullRequestFile struct {
	Filename string `json:"filename"`
	Patch    string `json:"patch"`
}

func parsePullRequestFiles(raw []byte) ([]pullRequestFile, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	files := make([]pullRequestFile, 0)
	for {
		var payload json.RawMessage
		if err := decoder.Decode(&payload); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		var singlePage []pullRequestFile
		if err := json.Unmarshal(payload, &singlePage); err == nil {
			files = append(files, singlePage...)
			continue
		}

		var slurpedPages [][]pullRequestFile
		if err := json.Unmarshal(payload, &slurpedPages); err == nil {
			for _, page := range slurpedPages {
				files = append(files, page...)
			}
			continue
		}

		return nil, fmt.Errorf("unexpected pull request files payload")
	}

	return files, nil
}

func mapPullRequestFilesToChangedFiles(files []pullRequestFile) []domain.ChangedFile {
	result := make([]domain.ChangedFile, 0)
	for _, item := range files {
		patch := strings.TrimSpace(item.Patch)
		if patch == "" {
			continue
		}
		result = append(result, domain.ChangedFile{
			Path:        item.Filename,
			Content:     patch,
			DiffSnippet: patch,
		})
	}
	return result
}
