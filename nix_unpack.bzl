load(":nix_providers.bzl", "NixStorePathInfo")

def _nix_unpack_impl(ctx):
    nar_file = ctx.file.nar_file
    
    # Create a dummy output to satisfy Bazel rule requirements
    # We name it with a suffix to avoid conflicts
    dummy = ctx.actions.declare_file(ctx.label.name + ".marker")
    ctx.actions.write(dummy, "")
    
    return [
        DefaultInfo(files = depset([dummy])),
        NixStorePathInfo(
            nar_file = nar_file,
            store_path = "/nix/store/" + ctx.attr.store_name,
        )
    ]

nix_unpack = rule(
    implementation = _nix_unpack_impl,
    attrs = {
        "nar_file": attr.label(allow_single_file = True, mandatory = True),
        "store_name": attr.string(mandatory = True),
        # fetch_tool is no longer needed here, but we keep it optional to avoid breaking existing calls if any
        "fetch_tool": attr.label(
            default = Label("//:nix-bazel-fetch"),
            executable = True,
            cfg = "exec",
            allow_files = True,
        ),
    },
)
