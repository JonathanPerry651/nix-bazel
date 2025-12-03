load("//:nix_package.bzl", "nix_package")

# Example: Fetching GNU Hello (x86_64-linux)
# Hash taken from a known good store path for hello-2.12.1
# /nix/store/35q1qj0f8c35q1qj0f8c35q1qj0f8c35-hello-2.12.1 (fake hash for example, need a real one)

# Real hash for hello 2.12.1 on x86_64-linux from cache.nixos.org might be different.
# I'll use a placeholder and expect it to fail or I need to find a real one.
# Let's try to find a real hash via web search or just use a dummy and see the tool try to fetch it.
# If I use an invalid hash, the tool will fail gracefully (hopefully).

nix_package(
    name = "git",
    package_id = "nixpkgs.git.aarch64-darwin",
)
