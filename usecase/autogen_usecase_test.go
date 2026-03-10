package usecase

import (
	"context"
	"errors"
	"testing"

	"bentos-backend/domain"
	uccontracts "bentos-backend/usecase/contracts"
	"github.com/stretchr/testify/require"
)

type autogenUseCaseTestGenerator struct {
	callCount   int
	lastPayload AutogenPayload
	output      string
	err         error
}

func (g *autogenUseCaseTestGenerator) Generate(_ context.Context, payload AutogenPayload) (string, error) {
	g.callCount++
	g.lastPayload = payload
	if g.err != nil {
		return "", g.err
	}
	return g.output, nil
}

type autogenUseCaseTestPublisher struct {
	callCount int
	lastReq   AutogenPublishRequest
	err       error
}

func (p *autogenUseCaseTestPublisher) PublishAutogen(_ context.Context, req AutogenPublishRequest) error {
	p.callCount++
	p.lastReq = req
	return p.err
}

type autogenUseCaseTestEnvironment struct {
	files     []domain.ChangedFile
	loadCalls int
	loadOpts  domain.CodeEnvironmentLoadOptions
	loadErr   error
}

func (e *autogenUseCaseTestEnvironment) SetupAgent(_ context.Context, _ domain.CodingAgentSetupOptions) (uccontracts.CodingAgent, error) {
	return nil, nil
}

func (e *autogenUseCaseTestEnvironment) LoadChangedFiles(_ context.Context, opts domain.CodeEnvironmentLoadOptions) ([]domain.ChangedFile, error) {
	e.loadCalls++
	e.loadOpts = opts
	if e.loadErr != nil {
		return nil, e.loadErr
	}
	return e.files, nil
}

func (e *autogenUseCaseTestEnvironment) PushChanges(_ context.Context, _ domain.CodeEnvironmentPushOptions) (domain.CodeEnvironmentPushResult, error) {
	return domain.CodeEnvironmentPushResult{}, nil
}

func (e *autogenUseCaseTestEnvironment) Cleanup(_ context.Context) error {
	return nil
}

type autogenUseCaseTestEnvironmentFactory struct {
	environment *autogenUseCaseTestEnvironment
	err         error
	lastOpts    domain.CodeEnvironmentInitOptions
}

func (f *autogenUseCaseTestEnvironmentFactory) New(_ context.Context, opts domain.CodeEnvironmentInitOptions) (uccontracts.CodeEnvironment, error) {
	f.lastOpts = opts
	if f.err != nil {
		return nil, f.err
	}
	return f.environment, nil
}

func TestAutogenUseCaseRequiresFlags(t *testing.T) {
	generator := &autogenUseCaseTestGenerator{output: "report"}
	publisher := &autogenUseCaseTestPublisher{}
	factory := &autogenUseCaseTestEnvironmentFactory{environment: &autogenUseCaseTestEnvironment{}}
	useCase, err := NewAutogenUseCase(generator, publisher, factory, nil)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), AutogenRequest{
		Input: domain.ChangeRequestInput{
			Target: domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 1},
		},
	})
	require.ErrorContains(t, err, "--docs")
}

func TestAutogenUseCaseRequiresHeadBranchWhenPublishing(t *testing.T) {
	generator := &autogenUseCaseTestGenerator{output: "report"}
	publisher := &autogenUseCaseTestPublisher{}
	factory := &autogenUseCaseTestEnvironmentFactory{environment: &autogenUseCaseTestEnvironment{}}
	useCase, err := NewAutogenUseCase(generator, publisher, factory, nil)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), AutogenRequest{
		Input: domain.ChangeRequestInput{
			Target: domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 1},
		},
		Docs:    true,
		Publish: true,
	})
	require.ErrorContains(t, err, "head branch")
}

func TestAutogenUseCaseRequiresAgentOutputWhenPublishing(t *testing.T) {
	generator := &autogenUseCaseTestGenerator{output: ""}
	publisher := &autogenUseCaseTestPublisher{}
	factory := &autogenUseCaseTestEnvironmentFactory{environment: &autogenUseCaseTestEnvironment{}}
	useCase, err := NewAutogenUseCase(generator, publisher, factory, nil)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), AutogenRequest{
		Input: domain.ChangeRequestInput{
			Target: domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 1},
		},
		Docs:       true,
		Publish:    true,
		HeadBranch: "feature",
	})
	require.ErrorContains(t, err, "agent output")
}

func TestAutogenUseCaseBuildsSummaryAndPublishes(t *testing.T) {
	env := &autogenUseCaseTestEnvironment{files: []domain.ChangedFile{
		{Path: "foo.go", DiffSnippet: "@@ -1,1 +1,2 @@\n line1\n+// added comment\n", Content: "line1\n// added comment"},
		{Path: "bar_test.go", DiffSnippet: "@@ -1,1 +1,2 @@\n line1\n+line2\n", Content: "line1\nline2"},
		{Path: "docs/readme.md", DiffSnippet: "@@ -1,1 +1,2 @@\n line1\n+line2\n", Content: "line1\nline2"},
		{Path: "newfile.go", DiffSnippet: "", Content: "package main\n// comment"},
	}}
	factory := &autogenUseCaseTestEnvironmentFactory{environment: env}
	generator := &autogenUseCaseTestGenerator{output: "agent report"}
	publisher := &autogenUseCaseTestPublisher{}
	useCase, err := NewAutogenUseCase(generator, publisher, factory, nil)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), AutogenRequest{
		Input: domain.ChangeRequestInput{
			Target:  domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 0},
			RepoURL: "https://github.com/org/repo.git",
			Base:    "main",
			Head:    "feature",
		},
		Docs:  true,
		Tests: true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, generator.callCount)
	require.Equal(t, 1, env.loadCalls)
	require.Equal(t, "@all", env.loadOpts.Head)
	require.Equal(t, 1, publisher.callCount)
	require.Equal(t, "agent report", publisher.lastReq.AgentOutput)
	require.Contains(t, publisher.lastReq.Summary.Tests, "bar_test.go")
	require.Contains(t, publisher.lastReq.Summary.Docs, "docs/readme.md")
	require.Contains(t, publisher.lastReq.Summary.Comments, "foo.go")
	require.Contains(t, publisher.lastReq.Summary.Comments, "newfile.go")
}

func TestAutogenUseCasePropagatesGeneratorError(t *testing.T) {
	env := &autogenUseCaseTestEnvironment{}
	factory := &autogenUseCaseTestEnvironmentFactory{environment: env}
	generator := &autogenUseCaseTestGenerator{err: errors.New("generator failed")}
	publisher := &autogenUseCaseTestPublisher{}
	useCase, err := NewAutogenUseCase(generator, publisher, factory, nil)
	require.NoError(t, err)

	_, err = useCase.Execute(context.Background(), AutogenRequest{
		Input: domain.ChangeRequestInput{
			Target: domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 0},
		},
		Docs: true,
	})
	require.ErrorContains(t, err, "generator failed")
}

func TestAutogenUseCaseSkipsErrorWhenNoChangesDetected(t *testing.T) {
	env := &autogenUseCaseTestEnvironment{loadErr: domain.ErrNoCodeChanges}
	factory := &autogenUseCaseTestEnvironmentFactory{environment: env}
	generator := &autogenUseCaseTestGenerator{output: "agent report"}
	publisher := &autogenUseCaseTestPublisher{}
	useCase, err := NewAutogenUseCase(generator, publisher, factory, nil)
	require.NoError(t, err)

	result, err := useCase.Execute(context.Background(), AutogenRequest{
		Input: domain.ChangeRequestInput{
			Target: domain.ChangeRequestTarget{Repository: "org/repo", ChangeRequestNumber: 1},
		},
		Docs:       true,
		Publish:    true,
		HeadBranch: "feature",
	})
	require.NoError(t, err)
	require.Empty(t, result.Changes)
	require.Empty(t, result.Summary.Docs)
	require.Empty(t, result.Summary.Tests)
	require.Empty(t, result.Summary.Comments)
	require.Equal(t, 1, publisher.callCount)
	require.Empty(t, publisher.lastReq.Changes)
}
