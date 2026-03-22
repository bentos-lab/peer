package codeagent

import "github.com/bentos-lab/peer/domain"

type issueAlignmentTaskTemplateData struct {
	Repository    string
	Base          string
	Head          string
	Title         string
	Description   string
	KeyIdeas      []string
	Issues        []issueAlignmentIssueData
	Files         []issueAlignmentFileData
	ExtraGuidance string
}

type issueAlignmentKeyIdeasTemplateData struct {
	Issues []issueAlignmentIssueData
}

type issueAlignmentIssueData struct {
	Repository string
	Number     int
	Title      string
	Body       string
	Comments   []domain.Comment
}

type issueAlignmentFileData struct {
	Path        string
	ChangedText string
}

const issueAlignmentSystemPrompt = "You are a senior engineer evaluating issue alignment."

const issueKeyIdeasSystemPrompt = "Extract the main, true requirements from the issue contents. Return only what is explicitly supported."
