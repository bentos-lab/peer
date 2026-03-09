package tracing

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"bentos-backend/usecase/contracts"
	"github.com/stretchr/testify/require"
)

type stubGenerator struct {
	generateResult     string
	generateErr        error
	generateJSONResult map[string]any
	generateJSONErr    error
}

func (s *stubGenerator) Generate(_ context.Context, _ contracts.GenerateParams) (string, error) {
	return s.generateResult, s.generateErr
}

func (s *stubGenerator) GenerateJSON(_ context.Context, _ contracts.GenerateParams) (map[string]any, error) {
	return s.generateJSONResult, s.generateJSONErr
}

type spyLogger struct {
	traceLogs []string
}

func (s *spyLogger) Tracef(format string, args ...any) {
	s.traceLogs = append(s.traceLogs, fmt.Sprintf(format, args...))
}

func (*spyLogger) Debugf(string, ...any) {}
func (*spyLogger) Infof(string, ...any)  {}
func (*spyLogger) Warnf(string, ...any)  {}
func (*spyLogger) Errorf(string, ...any) {}

func TestGeneratorGenerateLogsRequestAndResponse(t *testing.T) {
	base := &stubGenerator{generateResult: "hello"}
	logger := &spyLogger{}
	generator := NewGenerator(base, logger)

	output, err := generator.Generate(context.Background(), contracts.GenerateParams{
		SystemPrompt: "sys",
		Messages: []contracts.Message{
			{Role: "user", Content: "hi"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "hello", output)
	require.Len(t, logger.traceLogs, 1)
	require.Contains(t, logger.traceLogs[0], "llm response method=Generate")
	require.Contains(t, logger.traceLogs[0], `output="hello"`)
}

func TestGeneratorGenerateJSONLogsRequestAndResponse(t *testing.T) {
	base := &stubGenerator{
		generateJSONResult: map[string]any{
			"summary": "ok",
			"count":   1,
		},
	}
	logger := &spyLogger{}
	generator := NewGenerator(base, logger)

	output, err := generator.GenerateJSON(context.Background(), contracts.GenerateParams{
		SystemPrompt: "sys",
	})
	require.NoError(t, err)
	require.Equal(t, "ok", output["summary"])
	require.Len(t, logger.traceLogs, 1)
	require.Contains(t, logger.traceLogs[0], "llm response method=GenerateJSON")
	require.Contains(t, logger.traceLogs[0], `"summary":"ok"`)
}

func TestGeneratorGenerateLogsError(t *testing.T) {
	base := &stubGenerator{generateErr: errors.New("boom")}
	logger := &spyLogger{}
	generator := NewGenerator(base, logger)

	_, err := generator.Generate(context.Background(), contracts.GenerateParams{})
	require.Error(t, err)
	require.Len(t, logger.traceLogs, 1)
	require.Contains(t, logger.traceLogs[0], `error="boom"`)
}
