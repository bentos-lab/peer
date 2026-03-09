package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"bentos-backend/config"
	"bentos-backend/wiring"
)

type loadConfigFunc func() (config.Config, error)
type buildHandlerFunc func(config.Config) (http.Handler, error)
type listenAndServeFunc func(string, http.Handler) error

type serverDeps struct {
	loadConfig         loadConfigFunc
	buildGitHubHandler buildHandlerFunc
	listenAndServe     listenAndServeFunc
}

var errServerConfigLoad = errors.New("server config load failed")

type serverConfigLoadError struct {
	cause error
}

func (e serverConfigLoadError) Error() string {
	return fmt.Sprintf("load config: %v", e.cause)
}

func (e serverConfigLoadError) Unwrap() error {
	return e.cause
}

func (e serverConfigLoadError) Is(target error) bool {
	return target == errServerConfigLoad
}

// main bootstraps webhook handlers for GitHub.
func main() {
	logLevel := flag.String("log-level", "", "log level override: trace|debug|info|warning|error|silence")
	flag.Parse()

	if err := runServer(*logLevel, defaultServerDeps()); err != nil {
		if errors.Is(err, errServerConfigLoad) {
			log.Printf("server startup failed: %v", err)
		}
		os.Exit(1)
	}
}

func defaultServerDeps() serverDeps {
	return serverDeps{
		loadConfig: config.Load,
		buildGitHubHandler: func(cfg config.Config) (http.Handler, error) {
			return wiring.BuildGitHubHandler(cfg)
		},
		listenAndServe: http.ListenAndServe,
	}
}

func runServer(logLevelOverride string, deps serverDeps) error {
	cfg, err := deps.loadConfig()
	if err != nil {
		return serverConfigLoadError{cause: err}
	}
	if logLevelOverride != "" {
		cfg.LogLevel = logLevelOverride
	}
	startupLogger, err := wiring.BuildLogger(cfg, "")
	if err != nil {
		return err
	}
	startupLogger.Infof("server startup: config loaded; port=%q log-level=%q", cfg.Server.Port, cfg.LogLevel)
	effectiveOpenAIConfig, err := wiring.ResolveEffectiveOpenAIConfig(cfg, wiring.CLILLMOptions{})
	if err != nil {
		startupLogger.Errorf("server startup failed: resolve effective openai config: %v", err)
		return fmt.Errorf("resolve effective openai config: %w", err)
	}
	startupLogger.Infof(
		`server startup: llm_config base_url=%q model=%q`,
		effectiveOpenAIConfig.BaseURL,
		effectiveOpenAIConfig.Model,
	)

	startupLogger.Infof("server startup: wiring GitHub handler")
	githubHandler, err := deps.buildGitHubHandler(cfg)
	if err != nil {
		startupLogger.Errorf("server startup failed: build GitHub handler: %v", err)
		return fmt.Errorf("build GitHub handler: %w", err)
	}
	startupLogger.Infof("server startup: wired GitHub handler")

	mux := http.NewServeMux()
	mux.Handle("/webhook/github", githubHandler)
	startupLogger.Infof("server startup: route registered path=%q", "/webhook/github")

	addr := ":" + cfg.Server.Port
	startupLogger.Infof("server startup: listening on %q", addr)
	if err := deps.listenAndServe(addr, mux); err != nil {
		startupLogger.Errorf("server runtime failed: listen and serve: %v", err)
		return fmt.Errorf("listen and serve: %w", err)
	}

	return nil
}
