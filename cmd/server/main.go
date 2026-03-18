package main

import (
	"log"
	"net/http"

	"bentos-backend/config"
	"bentos-backend/wiring"
)

// main bootstraps webhook handlers for GitHub and GitLab.
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
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
