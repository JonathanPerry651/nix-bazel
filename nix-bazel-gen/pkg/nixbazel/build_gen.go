package nixbazel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (f *Fetcher) GenerateBuildFiles(lockFile, channel string) error {
	data, err := os.ReadFile(lockFile)
	if err != nil {
		return fmt.Errorf("failed to read lockfile: %w", err)
	}
	var lock Lockfile
	if err := json.Unmarshal(data, &lock); err != nil {
		return fmt.Errorf("failed to parse lockfile: %w", err)
	}

	// Collect all unique store paths
	uniquePaths := make(map[string]*NarInfo)

	for storePath, node := range lock.Packages {
		uniquePaths[storePath] = &NarInfo{
			URL:         node.URL,
			StorePath:   storePath,
			References:  node.References,
			Compression: "xz",
			FileHash:    node.FileHash,
		}
	}

	return f.generateBuildFiles(lock, uniquePaths, channel)
}

func (f *Fetcher) generateBuildFiles(lock Lockfile, uniquePaths map[string]*NarInfo, channel string) error {
	// 1. Generate per-package BUILD files
	for storePath := range uniquePaths {
		storeName := filepath.Base(storePath)
		packageDir := filepath.Join(f.outDir, storeName)

		// Ensure directory exists (it should, from unpacking)
		if err := os.MkdirAll(packageDir, 0755); err != nil {
			return err
		}

		buildFilePath := filepath.Join(packageDir, "BUILD.bazel")
		file, err := os.Create(buildFilePath)
		if err != nil {
			return err
		}

		fmt.Fprintf(file, "load(\"@nix_deps//:nix_unpack.bzl\", \"nix_unpack\")\n")
		fmt.Fprintf(file, "load(\"@nix_deps//:nix_root.bzl\", \"nix_root\")\n")
		fmt.Fprintf(file, "load(\"@nix_deps//:nix_bwrap.bzl\", \"nix_bwrap_run\")\n\n")
		fmt.Fprintf(file, "package(default_visibility = [\"//visibility:public\"])\n\n")

		// Calculate dependencies (transitive closure)
		var deps []string
		var storePaths []string

		transitiveDeps := getTransitiveClosure(storePath, lock.Packages)

		// Dependencies for the filegroup (other packages)
		for _, ref := range transitiveDeps {
			refName := filepath.Base(ref)
			deps = append(deps, fmt.Sprintf("\"//%s:%s\"", refName, refName))
		}

		// Store paths for bwrap (self + transitive)
		// Self
		storePaths = append(storePaths, fmt.Sprintf("\"%s\"", storePath))
		// Transitive
		for _, ref := range transitiveDeps {
			storePaths = append(storePaths, fmt.Sprintf("\"%s\"", ref))
		}

		// nix_unpack target
		// We assume the NAR file is available as a file in the repo rule
		// The repo rule should expose it.
		// Let's assume the repo rule downloads it to "nar/<hash>.nar.xz" or similar.
		// But wait, the repo rule is generating THIS file.
		// The repo rule has the NAR file.
		// We can refer to it if the repo rule exposes it.
		// Let's assume the repo rule puts it in `downloads/<hash>` and exposes it via exports_files or similar.
		// Or better, we can assume the repo rule generates a filegroup for it?

		// Actually, if we are inside the repo rule, we can refer to the file by its path relative to the BUILD file.
		// The BUILD file is in `repo_root/<store_name>/BUILD.bazel`.
		// The NAR file is likely in `repo_root/downloads/<hash>`.
		// So path is `../downloads/<hash>`.

		// narPath := fmt.Sprintf("../downloads/%s", uniquePaths[storePath].URL)
		// We need the file hash or whatever we used to save it.
		// In nix_package.bzl we used `node["fileHash"]`.
		// We don't have fileHash in NarInfo struct here yet.
		// We need to pass it.

		// For now, let's assume we can get it.
		// Wait, uniquePaths is map[string]*NarInfo. NarInfo has URL.
		// In `nix_package.bzl`, we save to `downloads/fileHash`.
		// We need to pass fileHash to `generateBuildFiles`.

		fmt.Fprintf(file, "nix_unpack(\n")
		fmt.Fprintf(file, "    name = \"%s\",\n", storeName)
		fmt.Fprintf(file, "    nar_file = \"//:downloads/%s\",\n", uniquePaths[storePath].FileHash)
		fmt.Fprintf(file, "    store_name = \"%s\",\n", storeName)
		fmt.Fprintf(file, ")\n\n")

		// Binaries
		// This is the catch.
		// If we use nix_unpack, we don't know what binaries are inside until build time.
		// But we need to generate targets for them.

		// Solution: We rely on the `Entrypoint` from the lockfile (if available) or we just expose the whole thing?
		// Binaries
		// We assume standard bin/ directory structure in the unpacked package
		// Since we haven't unpacked yet, we can't scan the directory.
		// However, the user asked for "top level entrypoints".
		// If we can't scan, we can't know what binaries exist.
		// BUT, we can generate a nix_root target for the package itself?
		// Or we can rely on the entrypoint from lockfile if present.

		// Wait, the user said "make a rule for the top level entrypoints".
		// If I can't scan, I can't generate targets for arbitrary binaries.
		// But I CAN generate a single `nix_root` target for the package, which includes its closure.
		// Then the user can run something inside it.

		// Let's generate a `nix_root` target named `root` (or `storeName_root`) for every package.
		// This target will build the symlink forest for that package's closure.

		fmt.Fprintf(file, "nix_root(\n")
		fmt.Fprintf(file, "    name = \"root\",\n")
		// Deps: self + transitive deps
		// We need to refer to the nix_unpack targets.
		// Self: :storeName
		// Transitive: //refName:refName

		var rootDeps []string
		rootDeps = append(rootDeps, fmt.Sprintf("\":%s\"", storeName))
		for _, ref := range transitiveDeps {
			refName := filepath.Base(ref)
			rootDeps = append(rootDeps, fmt.Sprintf("\"//%s:%s\"", refName, refName))
		}

		fmt.Fprintf(file, "    deps = [%s],\n", strings.Join(rootDeps, ", "))
		fmt.Fprintf(file, ")\n\n")

		file.Close()
	}

	// 2. Generate root BUILD file with aliases
	rootBuildPath := filepath.Join(f.outDir, "BUILD.bazel")
	fmt.Printf("Generating root %s...\n", rootBuildPath)

	file, err := os.Create(rootBuildPath)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "package(default_visibility = [\"//visibility:public\"])\n\n")

	// Explicitly export downloaded NAR files
	var downloadedFiles []string
	for _, info := range uniquePaths {
		downloadedFiles = append(downloadedFiles, fmt.Sprintf("\"downloads/%s\"", info.FileHash))
	}
	// Also export the fetch tool
	downloadedFiles = append(downloadedFiles, "\"nix-bazel-fetch\"")

	fmt.Fprintf(file, "exports_files([%s])\n\n", strings.Join(downloadedFiles, ", "))

	for repoName, repoLock := range lock.Repositories {
		storePath := repoLock.StorePath
		storeName := filepath.Base(storePath)

		// Alias for the nix_root target
		target := fmt.Sprintf("//%s:root", storeName)

		fmt.Fprintf(file, "alias(\n")
		fmt.Fprintf(file, "    name = \"%s\",\n", repoName)
		fmt.Fprintf(file, "    actual = \"%s\",\n", target)
		fmt.Fprintf(file, ")\n\n")
	}

	// 3. Generate update_nix_lock target
	fmt.Fprintf(file, "sh_binary(\n")
	fmt.Fprintf(file, "    name = \"update_nix_lock\",\n")
	fmt.Fprintf(file, "    srcs = [\"update_nix_lock.sh\"],\n")
	fmt.Fprintf(file, "    data = [\"packages.json\"],\n")
	fmt.Fprintf(file, ")\n\n")

	// Generate the shell script
	scriptPath := filepath.Join(f.outDir, "update_nix_lock.sh")
	scriptContent := `#!/bin/bash
set -e

# Find packages.json in runfiles
PACKAGES_JSON=$(find -L .. -name packages.json -type f | head -n 1)

if [ -z "$PACKAGES_JSON" ]; then
  echo "Error: packages.json not found"
  exit 1
fi

if [ -z "$BUILD_WORKSPACE_DIRECTORY" ]; then
  echo "Error: BUILD_WORKSPACE_DIRECTORY not set. Run with 'bazel run @nix_deps//:update_nix_lock'"
  exit 1
fi

# Assume the tool is built in the workspace
TOOL="$BUILD_WORKSPACE_DIRECTORY/nix-bazel-gen/nix-bazel-resolve"

if [ ! -f "$TOOL" ]; then
    echo "Building tool..."
    (cd "$BUILD_WORKSPACE_DIRECTORY/nix-bazel-gen" && go build -o nix-bazel-resolve ./cmd/nix-bazel-resolve)
fi

echo "Updating lockfile in $BUILD_WORKSPACE_DIRECTORY..."
CHANNEL_ARG=""
if [ -n "%s" ]; then
  CHANNEL_ARG="--channel %s"
fi
"$TOOL" --config "$PACKAGES_JSON" --lockfile "$BUILD_WORKSPACE_DIRECTORY/nix_deps.lock.json" $CHANNEL_ARG
`
	scriptContent = fmt.Sprintf(scriptContent, channel, channel)
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return err
	}

	return nil
}

func getTransitiveClosure(root string, packages map[string]ClosureNode) []string {
	closure := make(map[string]bool)
	var traverse func(string)
	traverse = func(path string) {
		if closure[path] {
			return
		}
		closure[path] = true
		node, ok := packages[path]
		if !ok {
			return
		}
		for _, ref := range node.References {
			fullRef := "/nix/store/" + ref
			traverse(fullRef)
		}
	}
	traverse(root)

	var result []string
	for path := range closure {
		if path != root {
			result = append(result, path)
		}
	}
	return result
}

func (f *Fetcher) generateBuildFile(info *NarInfo) error {
	storeName := filepath.Base(info.StorePath)
	// destDir is where we unpacked: repo_root/<store_name>
	// We want to generate BUILD.bazel in repo_root
	buildFilePath := filepath.Join(f.outDir, "BUILD.bazel")

	fmt.Printf("Generating %s...\n", buildFilePath)

	file, err := os.Create(buildFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "load(\"@nix_deps//:patchelf.bzl\", \"nix_patchelf\")\n\n")
	fmt.Fprintf(file, "package(default_visibility = [\"//visibility:public\"])\n\n")

	// Expose all files
	fmt.Fprintf(file, "filegroup(\n    name = \"all_files\",\n    srcs = glob([\"**\"]),\n)\n\n")

	// Find binaries in bin/
	binDir := filepath.Join(f.outDir, storeName, "bin")
	entries, err := os.ReadDir(binDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				binName := entry.Name()
				// Generate sh_binary for each executable
				// path relative to repo root: <store_name>/bin/<name>
				relPath := filepath.Join(storeName, "bin", binName)

				fmt.Fprintf(file, "nix_patchelf(\n")
				fmt.Fprintf(file, "    name = \"%s\",\n", binName)
				fmt.Fprintf(file, "    src = \"%s\",\n", relPath)
				// We need to include all files as data so it can find libs/resources
				fmt.Fprintf(file, "    data = [\":all_files\"],\n")
				fmt.Fprintf(file, ")\n\n")
			}
		}
	}

	return nil
}
