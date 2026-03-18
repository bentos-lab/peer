package main

import (
	"context"
	"fmt"
	"os"
)

var version = "dev"
var commit = "unknown"

// main bootstraps the peer CLI.
func main() {
	if err := runPeer(context.Background(), os.Args[1:], defaultPeerDeps(), version, commit); err != nil {
		os.Exit(1)
	}
}

func runPeer(ctx context.Context, args []string, deps peerDeps, version string, commit string) error {
	root := newRootCommand(ctx, deps, version, commit)
	root.SetArgs(args)
	_, err := root.ExecuteContextC(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	return err
}
