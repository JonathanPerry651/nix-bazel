package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"nix-bazel-gen/pkg/nixbazel"
)

func main() {
	outDir := flag.String("out", ".", "Output directory")
	archivePath := flag.String("archive", "", "Path to NAR archive")
	storePathFlag := flag.String("store-path", "", "Store path (e.g. /nix/store/...)")
	refs := flag.String("refs", "", "Comma-separated references for RPATH")

	flag.Parse()

	if *archivePath == "" || *storePathFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: --archive and --store-path are required")
		os.Exit(1)
	}

	fetcher := nixbazel.NewFetcher("", *outDir)
	refList := []string{}
	if *refs != "" {
		refList = strings.Split(*refs, ",")
	}
	if err := fetcher.UnpackAndPatch(*archivePath, *storePathFlag, refList); err != nil {
		fmt.Fprintf(os.Stderr, "Error unpacking: %v\n", err)
		os.Exit(1)
	}
}
