package main

import (
	"flag"
	"log"
	"net/http"

	"bentos-backend/config"
	"bentos-backend/wiring"
)

// main bootstraps webhook handlers for GitHub and GitLab.
func main() {
	logLevel := flag.String("log-level", "", "log level override: trace|debug|info|warning|error|silence")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if *logLevel != "" {
		cfg.LogLevel = *logLevel
	}

	githubHandler, err := wiring.BuildGitHubHandler(cfg)
	if err != nil {
		log.Fatal(err)
	}
	gitlabHandler, err := wiring.BuildGitLabHandler(cfg)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/webhook/github", githubHandler)
	mux.Handle("/webhook/gitlab", gitlabHandler)

	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatal(err)
	}
}
