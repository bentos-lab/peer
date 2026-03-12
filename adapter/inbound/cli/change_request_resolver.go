package cli

type ChangeRequestParams struct {
	VCSProvider   string
	Repo          string
	ChangeRequest string
	Base          string
	Head          string
	Comment       bool
}

type ChangeRequestResolution struct {
	Repository          string
	RepoURL             string
	ChangeRequestNumber int
	Title               string
	Description         string
	Base                string
	Head                string
}
