# Nix Fetcher for Bazel (Pure Go)

This repository contains a **Pure Go** implementation of a Nix package fetcher for Bazel. It allows Bazel projects to depend on Nix packages **without requiring Nix to be installed on the host machine or build nodes**.

## Key Features

*   **No Nix Dependency**: Runs entirely in user space using Go. Does not require the Nix daemon or `/nix/store` to be writable (unless configured to use it).
*   **Hermetic**: Fetches dependencies based on a lockfile (`nix_deps.lock.json`), ensuring reproducible builds.
*   **Deduplication**: Automatically deduplicates shared dependencies in the lockfile, keeping the dependency graph efficient.
*   **Hydra Resolution**: Can resolve package identifiers (e.g., `nixpkgs.git.aarch64-darwin`) to specific store paths by querying Hydra.
*   **Binary Patching**: Automatically patches ELF binaries (on Linux) using `patchelf` to make them relocatable and usable within the Bazel sandbox.
*   **Bzlmod Support**: Designed for modern Bazel with Bzlmod and module extensions.

## Prerequisites

*   **Go**: Required to build the fetcher tool.
*   **Bazel**: The build system.
*   **patchelf** (Linux only): Required for patching ELF binaries.
*   **xz**: Required for decompressing NAR archives.

## Usage (Bzlmod)

### 1. Configure `MODULE.bazel`

Add the `nix_deps` module extension to your `MODULE.bazel` file:

```python
nix = use_extension("//:nix_package.bzl", "nix_extension")
nix.packages(lockfile = "//:nix_deps.lock.json")

# Define your dependencies
nix.package(name = "git", package = "nixpkgs.git.aarch64-darwin")
nix.package(name = "hello", package = "nixpkgs.hello.x86_64-linux")

use_repo(nix, "nix_deps")
```

### 2. Generate/Update Lockfile

Run the update target to resolve dependencies and generate `nix_deps.lock.json`:

```bash
bazel run @nix_deps//:update_nix_lock
```

This will:
1.  Resolve the specified packages via Hydra.
2.  Recursively resolve their dependencies.
3.  Deduplicate the dependency graph.
4.  Write the lockfile to your workspace root.

### 3. Use in BUILD files

Dependencies are exposed as targets in the `@nix_deps` repository. You can use them in your `BUILD` files:

```python
sh_test(
    name = "git_test",
    srcs = ["test_git.sh"],
    data = ["@nix_deps//:git"], # Alias to the package
)
```

## How it Works

1.  **Resolution**: The `nix-bazel-resolve` tool queries Hydra to find the store path for a given package identifier. It then downloads the `.narinfo` for that path and recursively fetches `.narinfo` files for all dependencies.
2.  **Lockfile Generation**: It constructs a flattened dependency graph and writes it to `nix_deps.lock.json`.
3.  **Fetching**: During the build, the `nix_package` repository rule invokes `nix-bazel-fetch` (or `nix-bazel-generate` for build files) to download the NAR archives specified in the lockfile.
4.  **Unpacking & Patching**: The archives are unpacked into the Bazel external repository. On Linux, `patchelf` is used to set the `RPATH` of binaries to `$ORIGIN/...`, allowing them to find their libraries relative to themselves.
5.  **Build Generation**: A `BUILD.bazel` file is generated for each package, exposing its files and binaries.

## Directory Structure

*   `nix-bazel-gen/`: Source code for the Go tools.
    *   `cmd/`: Entry points for the CLI tools (`resolve`, `fetch`, `generate`).
    *   `pkg/nixbazel/`: Shared library code.
*   `nix_package.bzl`: Starlark implementation of the repository rule and module extension.
*   `nix_deps.lock.json`: The generated lockfile (do not edit manually).

## License

[Add License Here]
