package main

import (
	"flag"
	"fmt"
	"os"

	"nix-bazel-gen/pkg/nixbazel"
)

func main() {
	outDir := flag.String("out", ".", "Output directory")
	lockFile := flag.String("lockfile", "nix_deps.lock.json", "Lockfile output path")
	channel := flag.String("channel", "", "Nix channel (Hydra jobset) to use for resolution")

	flag.Parse()

	if *lockFile == "" {
		fmt.Fprintln(os.Stderr, "Error: --lockfile is required")
		os.Exit(1)
	}

	fetcher := nixbazel.NewFetcher("", *outDir)
	if err := fetcher.GenerateBuildFiles(*lockFile, *channel); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating build files: %v\n", err)
		os.Exit(1)
	}
}
