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
	for storePath, info := range uniquePaths {
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

		fmt.Fprintf(file, "package(default_visibility = [\"//visibility:public\"])\n\n")

		// Calculate dependencies (other store paths)
		var deps []string
		for _, ref := range info.References {
			refName := filepath.Base(ref)
			if refName == storeName {
				continue // Self-reference
			}
			// Dependency format: // <refName> : <refName>
			deps = append(deps, fmt.Sprintf("\"//%s:%s\"", refName, refName))
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

					fmt.Fprintf(file, "sh_binary(\n")
					fmt.Fprintf(file, "    name = \"%s\",\n", binName)
					fmt.Fprintf(file, "    srcs = [\"bin/%s\"],\n", binName)
					fmt.Fprintf(file, "    data = [\":%s\"],\n", storeName)
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

	for repoName, storePath := range lock.Repositories {
		storeName := filepath.Base(storePath)

		// Alias for the filegroup
		// If the user asked for "git", they might expect the binary "git" if it exists,
		// or the filegroup if it's a library.
		// Let's check if a binary with the same name exists in that package.

		target := fmt.Sprintf("//%s:%s", storeName, storeName) // Default to filegroup

		binPath := filepath.Join(f.outDir, storeName, "bin", repoName)
		if _, err := os.Stat(binPath); err == nil {
			// Binary exists, alias to it
			target = fmt.Sprintf("//%s:%s", storeName, repoName)
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

				fmt.Fprintf(file, "sh_binary(\n")
				fmt.Fprintf(file, "    name = \"%s\",\n", binName)
				fmt.Fprintf(file, "    srcs = [\"%s\"],\n", relPath)
				// We need to include all files as data so it can find libs/resources
				fmt.Fprintf(file, "    data = [\":all_files\"],\n")
				fmt.Fprintf(file, ")\n\n")
			}
		}
	}

	return nil
}
