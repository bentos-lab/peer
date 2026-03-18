package main

import (
	"context"
	"errors"
	"log"
	"os"
)

var version = "dev"
var commit = "unknown"

// main bootstraps the peer CLI.
func main() {
	if err := runPeer(context.Background(), os.Args[1:], defaultPeerDeps(), version, commit); err != nil {
		if errors.Is(err, errCLIConfigLoad) {
			log.Printf("cli startup failed: %v", err)
		}
		if errors.Is(err, errWebhookConfigLoad) {
			log.Printf("webhook startup failed: %v", err)
		}
		os.Exit(1)
	}
}

func runPeer(ctx context.Context, args []string, deps peerDeps, version string, commit string) error {
	root := newRootCommand(ctx, deps, version, commit)
	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}
