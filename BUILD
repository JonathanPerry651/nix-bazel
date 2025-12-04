exports_files(["nix-bazel-resolve", "nix-bazel-fetch", "nix-bazel-generate"])

sh_test(
    name = "git_test",
    srcs = ["test_hello.sh"],
    data = ["@nix_deps//:git"],
    args = ["$(location @nix_deps//:git)"],
)

sh_test(
    name = "stress_test",
    srcs = ["stress_test.sh"],
    args = [
        "$(location @nix_deps//:python3)",
        "$(location @nix_deps//:curl)",
        "$(location @nix_deps//:imagemagick)",
    ],
    data = [
        "@nix_deps//:python3",
        "@nix_deps//:curl",
        "@nix_deps//:imagemagick",
    ],
)

