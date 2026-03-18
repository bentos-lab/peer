package cli

import "github.com/bentos-lab/peer/domain"

type ChangeRequestParams struct {
	VCSProvider    string
	VCSHost        string
	Repo           string
	ChangeRequest  string
	Base           string
	Head           string
	Publish        bool
	IssueAlignment bool
}

type ChangeRequestResolution struct {
	Repository          string
	RepoURL             string
	ChangeRequestNumber int
	Title               string
	Description         string
	Base                string
	Head                string
	HeadRefName         string
	IssueCandidates     []domain.IssueContext
}
