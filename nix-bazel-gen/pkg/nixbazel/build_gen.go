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

		fmt.Fprintf(file, "load(\"@nix_deps//:patchelf.bzl\", \"nix_patchelf\")\n\n")
		fmt.Fprintf(file, "package(default_visibility = [\"//visibility:public\"])\n\n")

		// Calculate dependencies (transitive closure)
		var deps []string
		var depNames []string

		transitiveDeps := getTransitiveClosure(storePath, lock.Packages)

		for _, ref := range transitiveDeps {
			refName := filepath.Base(ref)
			// Dependency format: // <refName> : <refName>
			deps = append(deps, fmt.Sprintf("\"//%s:%s\"", refName, refName))
			depNames = append(depNames, fmt.Sprintf("\"%s\"", refName))
		}

		// Filegroup for the whole store path
		fmt.Fprintf(file, "filegroup(\n")
		fmt.Fprintf(file, "    name = \"%s\",\n", storeName)
		fmt.Fprintf(file, "    srcs = glob([\"**\"], exclude = [\"BUILD.bazel\"]),\n")
		if len(deps) > 0 {
			fmt.Fprintf(file, "    data = [%s],\n", strings.Join(deps, ", "))
		}
		fmt.Fprintf(file, ")\n\n")

		// Binaries
		binDir := filepath.Join(packageDir, "bin")
		entries, err := os.ReadDir(binDir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					binName := entry.Name()
					// Target name: binName (e.g. git)
					// It's safe to use binName because storeName is usually long and distinct

					fmt.Fprintf(file, "nix_patchelf(\n")
					fmt.Fprintf(file, "    name = \"%s\",\n", binName)
					fmt.Fprintf(file, "    src = \"bin/%s\",\n", binName)
					fmt.Fprintf(file, "    data = [\":%s\"],\n", storeName)
					if len(deps) > 0 {
						fmt.Fprintf(file, "    deps = [%s],\n", strings.Join(deps, ", "))
						fmt.Fprintf(file, "    nix_store_names = [%s],\n", strings.Join(depNames, ", "))
					}
					fmt.Fprintf(file, ")\n\n")
				}
			}
		}
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

	for repoName, repoLock := range lock.Repositories {
		storePath := repoLock.StorePath
		storeName := filepath.Base(storePath)

		// Alias for the filegroup
		// If the user asked for "git", they might expect the binary "git" if it exists,
		// or the filegroup if it's a library.

		target := fmt.Sprintf("//%s:%s", storeName, storeName) // Default to filegroup

		if repoLock.Entrypoint != "" {
			// User specified entrypoint, use it
			// We assume the entrypoint binary name matches the basename of the entrypoint path
			// e.g. bin/git -> git
			entrypointName := filepath.Base(repoLock.Entrypoint)
			target = fmt.Sprintf("//%s:%s", storeName, entrypointName)
		} else {
			// Let's check if a binary with the same name exists in that package.
			binPath := filepath.Join(f.outDir, storeName, "bin", repoName)
			if _, err := os.Stat(binPath); err == nil {
				// Binary exists, alias to it
				target = fmt.Sprintf("//%s:%s", storeName, repoName)
			}
		}

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
