load(":nix_providers.bzl", "NixStorePathInfo")

def _nix_root_impl(ctx):
    # We create a single output directory containing all unpacked packages
    out_dir = ctx.actions.declare_directory(ctx.label.name)
    
    fetch_tool = ctx.executable.fetch_tool
    
    inputs = [fetch_tool]
    args = []
    
    # Script to unpack and rewrite
    # We generate a script that calls fetch_tool for each NAR
    # and then rewrites symlinks.
    
    script_lines = [
        "set -e",
        "OUT_DIR=$1",
        "FETCH_TOOL=$2",
        "mkdir -p \"$OUT_DIR\"",
    ]
    
    for dep in ctx.attr.deps:
        if NixStorePathInfo in dep:
            info = dep[NixStorePathInfo]
            inputs.append(info.nar_file)
            
            store_name = info.store_path.split("/")[-1]
            
            script_lines.append(
                "\"$FETCH_TOOL\" --archive \"%s\" --out \"$OUT_DIR\" --store-path \"%s\"" % (info.nar_file.path, info.store_path)
            )
            
    # Symlink rewriting logic
    # Convert absolute /nix/store/... symlinks to relative ../...
    script_lines.append("""
    find "$OUT_DIR" -type l -print0 | while IFS= read -r -d '' LINK; do
      TARGET=$(readlink "$LINK")
      if [[ "$TARGET" == /nix/store/* ]]; then
        # Absolute symlink to store
        # Convert to relative using pure bash string manipulation to avoid realpath issues
        
        # 1. Get directory of the link relative to OUT_DIR
        # LINK is $OUT_DIR/path/to/link
        REL_LINK=${LINK#$OUT_DIR/}
        REL_LINK_DIR=$(dirname "$REL_LINK")
        
        # 2. Construct traversal up to root
        DOTS=""
        if [ "$REL_LINK_DIR" != "." ]; then
            # Replace each component with ..
            # Use sed with escaped backslash for Starlark
            DOTS=$(echo "$REL_LINK_DIR" | sed 's|[^/][^/]*|..|g')
            DOTS="$DOTS/"
        fi
        
        # 3. Construct new target
        # Target relative to OUT_DIR (stripping /nix/store prefix)
        REL_TARGET=${TARGET#/nix/store/}
        NEW_TARGET="${DOTS}${REL_TARGET}"
        
        rm "$LINK"
        ln -s "$NEW_TARGET" "$LINK"
      fi
    done
    """)

    
    ctx.actions.run_shell(
        outputs = [out_dir],
        inputs = inputs,
        command = "\n".join(script_lines),
        arguments = [out_dir.path, fetch_tool.path],
        mnemonic = "NixRootUnpack",
        progress_message = "Unpacking Nix Root %s" % ctx.label.name,
        use_default_shell_env = True,
    )
            
    return [
        DefaultInfo(
            files = depset([out_dir]),
            runfiles = ctx.runfiles(files = [out_dir]),
        )
    ]

nix_root = rule(
    implementation = _nix_root_impl,
    attrs = {
        "deps": attr.label_list(providers = [NixStorePathInfo]),
        "fetch_tool": attr.label(
            default = Label("//:nix-bazel-fetch"),
            executable = True,
            cfg = "exec",
            allow_files = True,
        ),
    },
)
