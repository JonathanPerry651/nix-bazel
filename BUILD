exports_files(["nix-bazel-resolve", "nix-bazel-fetch", "nix-bazel-generate"])

platform(
    name = "rbe_ubuntu2004",
    constraint_values = [
        "@platforms//os:linux",
        "@platforms//cpu:x86_64",
    ],
    exec_properties = {
        "OSFamily": "Linux",
        "container-image": "docker://gcr.io/cloud-marketplace/google/rbe-ubuntu20-04:latest",
    },
)

sh_test(
    name = "git_test",
    srcs = ["test_hello.sh"],
    data = ["@nix_deps//:git"],
)

