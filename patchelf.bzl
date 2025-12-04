def _nix_patchelf_impl(ctx):
    # The output file will have the same name as the rule instance
    out = ctx.actions.declare_file(ctx.label.name)
    src = ctx.file.src

    # Find dynamic linker
    loader_store_name = ""
    loader_basename = ""
    for dep in ctx.attr.deps:
        for f in dep[DefaultInfo].files.to_list():
            if f.basename.startswith("ld-linux") and f.basename.endswith(".so.2"):
                 for name in ctx.attr.nix_store_names:
                    if name in f.path:
                         loader_store_name = name
                         loader_basename = f.basename
                         break
            if loader_store_name:
                break
        if loader_store_name:
            break

    # Declare the actual binary (patched)
    out_bin = ctx.actions.declare_file(ctx.label.name + ".bin")
    
    # Patch the binary
    # We DO NOT set the interpreter here, because we will invoke it explicitly.
    # We still set RPATH.
    # RPATH is relative to the binary location.
    # Binary is at .../package/git.bin
    # Deps are at .../dep/lib
    # So RPATH: $ORIGIN/../dep/lib
    
    rpaths = []
    rpaths.append("$ORIGIN/lib")
    rpaths.append("$ORIGIN/lib64")
    for dep_name in ctx.attr.nix_store_names:
        rpaths.append("$ORIGIN/../%s/lib" % dep_name)
        rpaths.append("$ORIGIN/../%s/lib64" % dep_name)
    rpath_str = ":".join(rpaths)

    command = """
    cp {src} {out_bin}
    chmod +w {out_bin}
    patchelf --set-rpath '{rpath}' {out_bin}
    """.format(
        src = src.path,
        out_bin = out_bin.path,
        rpath = rpath_str,
    )

    ctx.actions.run_shell(
        outputs = [out_bin],
        inputs = [src],
        command = command,
        mnemonic = "Patchelf",
        use_default_shell_env = True,
    )

    # Create wrapper script
    wrapper_content = """#!/bin/bash
set -e
DIR=$(dirname "$0")
# Resolve symlinks if any
if [ -L "$0" ]; then
  DIR=$(dirname "$(readlink -f "$0")")
fi

if [ -d "$0.runfiles" ]; then
  # Running from bazel-bin (bazel run)
  # Use absolute path in runfiles
  # Binary path: runfiles/workspace/package/name.bin
  BINARY="$0.runfiles/{workspace_name}/{package_name}/{name}.bin"
  LOADER="$0.runfiles/{workspace_name}/{loader_store_name}/lib/{loader_basename}"
else
  # Running from runfiles (bazel test)
  # Use relative path
  BINARY="$DIR/{name}.bin"
  LOADER="$DIR/../{loader_store_name}/lib/{loader_basename}"
fi

exec "$LOADER" "$BINARY" "$@"
""".format(
        name = ctx.label.name,
        workspace_name = ctx.label.workspace_name,
        package_name = ctx.label.package,
        loader_store_name = loader_store_name,
        loader_basename = loader_basename,
    )

    ctx.actions.write(
        output = out,
        content = wrapper_content,
        is_executable = True,
    )

    # Return the executable (wrapper) and runfiles
    runfiles = ctx.runfiles(files = [out, out_bin])
    for dep in ctx.attr.deps:
        runfiles = runfiles.merge(dep[DefaultInfo].default_runfiles)
    
    # Also include the data dependencies (the raw files of this package)
    for data in ctx.attr.data:
        runfiles = runfiles.merge(data[DefaultInfo].default_runfiles)
        runfiles = runfiles.merge(ctx.runfiles(transitive_files = data[DefaultInfo].files))

    return [DefaultInfo(
        executable = out,
        runfiles = runfiles,
    )]

nix_patchelf = rule(
    implementation = _nix_patchelf_impl,
    attrs = {
        "src": attr.label(allow_single_file = True, mandatory = True),
        "deps": attr.label_list(),
        "data": attr.label_list(),
        "nix_store_names": attr.string_list(),
    },
    executable = True,
)
