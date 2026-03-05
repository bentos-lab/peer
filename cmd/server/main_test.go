package main

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"strings"
	"testing"

	"bentos-backend/config"

	"github.com/stretchr/testify/require"
)

func TestRunServer_Scenarios(t *testing.T) {
	baseCfg := config.Config{
		LogLevel: "info",
		OpenAI: config.OpenAIConfig{
			BaseURL: "gemini",
			Model:   "gemini-2.5-flash-lite",
		},
		Server: config.ServerConfig{
			Port: "9090",
		},
	}

	testCases := []struct {
		name                    string
		logLevelOverride        string
		loadErr                 error
		githubErr               error
		gitlabErr               error
		listenErr               error
		expectedErrContains     string
		expectedGitHubCalls     int
		expectedGitLabCalls     int
		expectedListenCalls     int
		expectedLogPartsInOrder []string
	}{
		{
			name:                "success",
			expectedGitHubCalls: 1,
			expectedGitLabCalls: 1,
			expectedListenCalls: 1,
			expectedLogPartsInOrder: []string{
				`server startup: config loaded; port="9090" log-level="info"`,
				`server startup: llm_config base_url="https://generativelanguage.googleapis.com/v1beta/openai" model="gemini-2.5-flash-lite"`,
				`server startup: wiring GitHub handler`,
				`server startup: wired GitHub handler`,
				`server startup: wiring GitLab handler`,
				`server startup: wired GitLab handler`,
				`server startup: route registered path="/webhook/github"`,
				`server startup: route registered path="/webhook/gitlab"`,
				`server startup: listening on ":9090"`,
			},
		},
		{
			name:                    "config load failure",
			loadErr:                 errors.New("config boom"),
			expectedErrContains:     "load config: config boom",
			expectedGitHubCalls:     0,
			expectedGitLabCalls:     0,
			expectedListenCalls:     0,
			expectedLogPartsInOrder: nil,
		},
		{
			name:                    "github handler build failure",
			githubErr:               errors.New("github boom"),
			expectedErrContains:     "build GitHub handler: github boom",
			expectedGitHubCalls:     1,
			expectedGitLabCalls:     0,
			expectedListenCalls:     0,
			expectedLogPartsInOrder: []string{`server startup failed: build GitHub handler: github boom`},
		},
		{
			name:                    "gitlab handler build failure",
			gitlabErr:               errors.New("gitlab boom"),
			expectedErrContains:     "build GitLab handler: gitlab boom",
			expectedGitHubCalls:     1,
			expectedGitLabCalls:     1,
			expectedListenCalls:     0,
			expectedLogPartsInOrder: []string{`server startup failed: build GitLab handler: gitlab boom`},
		},
		{
			name:                    "listen failure",
			listenErr:               errors.New("listen boom"),
			expectedErrContains:     "listen and serve: listen boom",
			expectedGitHubCalls:     1,
			expectedGitLabCalls:     1,
			expectedListenCalls:     1,
			expectedLogPartsInOrder: []string{`server runtime failed: listen and serve: listen boom`},
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			var buffer bytes.Buffer
			restoreLogging := captureLogOutput(t, &buffer)
			defer restoreLogging()

			gitHubCalls := 0
			gitLabCalls := 0
			listenCalls := 0

			err := runServer(testCase.logLevelOverride, serverDeps{
				loadConfig: func() (config.Config, error) {
					if testCase.loadErr != nil {
						return config.Config{}, testCase.loadErr
					}
					return baseCfg, nil
				},
				buildGitHubHandler: func(config.Config) (http.Handler, error) {
					gitHubCalls++
					if testCase.githubErr != nil {
						return nil, testCase.githubErr
					}
					return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil
				},
				buildGitLabHandler: func(config.Config) (http.Handler, error) {
					gitLabCalls++
					if testCase.gitlabErr != nil {
						return nil, testCase.gitlabErr
					}
					return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil
				},
				listenAndServe: func(string, http.Handler) error {
					listenCalls++
					return testCase.listenErr
				},
			})

			if testCase.expectedErrContains == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), testCase.expectedErrContains)
			}

			require.Equal(t, testCase.expectedGitHubCalls, gitHubCalls)
			require.Equal(t, testCase.expectedGitLabCalls, gitLabCalls)
			require.Equal(t, testCase.expectedListenCalls, listenCalls)

			assertLogContainsInOrder(t, buffer.String(), testCase.expectedLogPartsInOrder)
			if testCase.loadErr != nil {
				require.Empty(t, buffer.String())
			}
		})
	}
}

func TestRunServerMarksConfigLoadErrors(t *testing.T) {
	err := runServer("", serverDeps{
		loadConfig: func() (config.Config, error) {
			return config.Config{}, errors.New("config boom")
		},
		buildGitHubHandler: func(config.Config) (http.Handler, error) {
			return nil, nil
		},
		buildGitLabHandler: func(config.Config) (http.Handler, error) {
			return nil, nil
		},
		listenAndServe: func(string, http.Handler) error {
			return nil
		},
	})

	require.Error(t, err)
	require.ErrorContains(t, err, "load config: config boom")
	require.ErrorIs(t, err, errServerConfigLoad)
}

func TestRunServer_LogLevelOverrideBehavior(t *testing.T) {
	testCases := []struct {
		name                 string
		override             string
		configLevel          string
		expectedHandlerLevel string
	}{
		{
			name:                 "uses config level when override is empty",
			override:             "",
			configLevel:          "warning",
			expectedHandlerLevel: "warning",
		},
		{
			name:                 "uses override level when provided",
			override:             "error",
			configLevel:          "warning",
			expectedHandlerLevel: "error",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			var buffer bytes.Buffer
			restoreLogging := captureLogOutput(t, &buffer)
			defer restoreLogging()

			var gitHubReceivedCfg config.Config

			err := runServer(testCase.override, serverDeps{
				loadConfig: func() (config.Config, error) {
					return config.Config{
						LogLevel: testCase.configLevel,
						OpenAI: config.OpenAIConfig{
							BaseURL: "gemini",
							Model:   "gemini-2.5-flash-lite",
						},
						Server: config.ServerConfig{
							Port: "8080",
						},
					}, nil
				},
				buildGitHubHandler: func(cfg config.Config) (http.Handler, error) {
					gitHubReceivedCfg = cfg
					return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil
				},
				buildGitLabHandler: func(config.Config) (http.Handler, error) {
					return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil
				},
				listenAndServe: func(string, http.Handler) error {
					return nil
				},
			})
			require.NoError(t, err)
			require.Equal(t, testCase.expectedHandlerLevel, gitHubReceivedCfg.LogLevel)
		})
	}
}

func TestRunServerLogsResolvedLLMConfig(t *testing.T) {
	testCases := []struct {
		name                 string
		openAIConfig         config.OpenAIConfig
		expectedLLMConfigLog string
	}{
		{
			name: "shortcut openai",
			openAIConfig: config.OpenAIConfig{
				BaseURL: "openai",
				Model:   "ignored-model",
			},
			expectedLLMConfigLog: `server startup: llm_config base_url="https://api.openai.com/v1" model="ignored-model"`,
		},
		{
			name: "full custom url",
			openAIConfig: config.OpenAIConfig{
				BaseURL: "https://example.com/v1",
				Model:   "custom-model",
			},
			expectedLLMConfigLog: `server startup: llm_config base_url="https://example.com/v1" model="custom-model"`,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			var buffer bytes.Buffer
			restoreLogging := captureLogOutput(t, &buffer)
			defer restoreLogging()

			err := runServer("", serverDeps{
				loadConfig: func() (config.Config, error) {
					return config.Config{
						LogLevel: "info",
						OpenAI:   testCase.openAIConfig,
						Server: config.ServerConfig{
							Port: "8080",
						},
					}, nil
				},
				buildGitHubHandler: func(config.Config) (http.Handler, error) {
					return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil
				},
				buildGitLabHandler: func(config.Config) (http.Handler, error) {
					return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), nil
				},
				listenAndServe: func(string, http.Handler) error {
					return nil
				},
			})
			require.NoError(t, err)
			require.Contains(t, buffer.String(), testCase.expectedLLMConfigLog)
		})
	}
}

func captureLogOutput(t *testing.T, output *bytes.Buffer) func() {
	t.Helper()

	originalWriter := log.Writer()
	originalFlags := log.Flags()
	originalPrefix := log.Prefix()

	log.SetOutput(output)
	log.SetFlags(0)
	log.SetPrefix("")

	return func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
		log.SetPrefix(originalPrefix)
	}
}

func assertLogContainsInOrder(t *testing.T, logs string, expectedParts []string) {
	t.Helper()

	currentIndex := 0
	for _, part := range expectedParts {
		relativeIndex := strings.Index(logs[currentIndex:], part)
		require.NotEqualf(t, -1, relativeIndex, "missing log part %q in logs: %s", part, logs)
		currentIndex += relativeIndex + len(part)
	}
}
