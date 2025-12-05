package main

import (
	"flag"
	"fmt"
	"os"

	"nix-bazel-gen/pkg/nixbazel"
)

func main() {
	outDir := flag.String("out", ".", "Output directory")
	archivePath := flag.String("archive", "", "Path to NAR archive")
	storePathFlag := flag.String("store-path", "", "Store path (e.g. /nix/store/...)")

	flag.Parse()

	if *archivePath == "" || *storePathFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: --archive and --store-path are required")
		os.Exit(1)
	}

	fetcher := nixbazel.NewFetcher("", *outDir)
	if err := fetcher.Unpack(*archivePath, *storePathFlag); err != nil {
		fmt.Fprintf(os.Stderr, "Error unpacking: %v\n", err)
		os.Exit(1)
	}
}
