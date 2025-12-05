load(":nix_providers.bzl", "NixStorePathInfo")

def _nix_bwrap_run_impl(ctx):
    root_dir = ctx.attr.root[DefaultInfo].files.to_list()[0]
    
    entrypoint_info = ctx.attr.entrypoint[NixStorePathInfo]
    store_name = entrypoint_info.store_path.split("/")[-1]
    
    # Path to binary inside /nix/store
    # /nix/store/<store_name>/<bin_path>
    binary_path = "/nix/store/%s/%s" % (store_name, ctx.attr.bin_path)
    
    wrapper_content = """#!/bin/bash
set -e
DIR=$(dirname "$0")
# Resolve symlinks if any
if [ -L "$0" ]; then
  DIR=$(dirname "$(readlink -f "$0")")
fi

# The root directory (Tree Artifact) is in runfiles
# We need to find it.
# It should be at $DIR/../<workspace>/<package>/<root_name>
# But simpler: use rlocation if available, or find it relative to script.

# For now, let's assume standard runfiles layout
# root_dir.short_path gives path relative to runfiles root
ROOT_PATH="$DIR/{root_short_path}"

# Verify root exists
if [ ! -d "$ROOT_PATH" ]; then
    # Try finding it in .runfiles
    if [ -d "$0.runfiles/{workspace_name}/{root_short_path}" ]; then
        ROOT_PATH="$0.runfiles/{workspace_name}/{root_short_path}"
    elif [ -d "$DIR/{root_basename}" ]; then
        # Sometimes it's flattened?
        ROOT_PATH="$DIR/{root_basename}"
    else
        echo "Error: Could not locate Nix root at $ROOT_PATH" >&2
        find "$DIR" -maxdepth 3 >&2
        exit 1
    fi
fi

# Construct bwrap arguments
ARGS=(
    --ro-bind / /
    --dev /dev
    --proc /proc
    --bind /tmp /tmp
    --tmpfs /nix
    --dir /nix/store
    --ro-bind "$ROOT_PATH" /nix/store
)

exec bwrap "${{ARGS[@]}}" "{binary_path}" "$@"
""".format(
    root_short_path = root_dir.short_path,
    root_basename = root_dir.basename,
    workspace_name = ctx.workspace_name,
    binary_path = binary_path,
)

    output_script = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.write(output_script, wrapper_content, is_executable = True)

    runfiles = ctx.runfiles(files = [output_script])
    runfiles = runfiles.merge(ctx.attr.root[DefaultInfo].default_runfiles)
    
    return [DefaultInfo(
        executable = output_script,
        runfiles = runfiles,
    )]

nix_bwrap_run = rule(
    implementation = _nix_bwrap_run_impl,
    attrs = {
        "root": attr.label(mandatory = True),
        "entrypoint": attr.label(mandatory = True, providers = [NixStorePathInfo]),
        "bin_path": attr.string(default = "bin/git"), # Default for now, should be mandatory or inferred
    },
    executable = True,
)
