exports_files(["nix-bazel-resolve", "nix-bazel-fetch", "nix-bazel-generate"])

sh_test(
    name = "git_test",
    srcs = ["test_hello.sh"],
    data = ["@nix_deps//:git"],
)

