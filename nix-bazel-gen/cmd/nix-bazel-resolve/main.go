package main

import (
	"flag"
	"fmt"
	"os"

	"nix-bazel-gen/pkg/nixbazel"
)

func main() {
	configFile := flag.String("config", "nix_deps.json", "Config file for resolution")
	lockFile := flag.String("lockfile", "nix_deps.lock.json", "Lockfile output path")
	channel := flag.String("channel", "", "Nix channel (Hydra jobset) to use for resolution")

	flag.Parse()

	if err := nixbazel.RunResolve(*configFile, *lockFile, *channel); err != nil {
		fmt.Fprintf(os.Stderr, "Resolution failed: %v\n", err)
		os.Exit(1)
	}
}
