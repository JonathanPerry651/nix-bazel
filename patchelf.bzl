def _nix_patchelf_impl(ctx):
    # The output file will have the same name as the rule instance
    out = ctx.actions.declare_file(ctx.label.name)
    src = ctx.file.src

    # Calculate RPATHs
    # We assume the layout:
    # .../storeName/bin/binary
    # .../depName/lib
    # So we need $ORIGIN/../../depName/lib
    
    rpaths = []
    
    # Add own lib dir
    # $ORIGIN/../lib
    rpaths.append("$ORIGIN/../lib")
    rpaths.append("$ORIGIN/../lib64")

    for dep_name in ctx.attr.nix_store_names:
        # $ORIGIN/../../depName/lib
        rpaths.append("$ORIGIN/../../%s/lib" % dep_name)
        rpaths.append("$ORIGIN/../../%s/lib64" % dep_name)

    rpath_str = ":".join(rpaths)

    # We need to copy the src to out, then patch it.
    # We use a shell command for this.
    # We assume 'patchelf' is available in the execution environment (e.g. Linux executor).
    
    command = """
    cp {src} {out}
    chmod +w {out}
    patchelf --set-rpath "{rpath}" {out}
    """.format(
        src = src.path,
        out = out.path,
        rpath = rpath_str,
    )

    ctx.actions.run_shell(
        outputs = [out],
        inputs = [src],
        command = command,
        mnemonic = "Patchelf",
        use_default_shell_env = True,
    )

    # Return the executable
    # We also need to return runfiles (the dependencies)
    runfiles = ctx.runfiles(files = [out])
    for dep in ctx.attr.deps:
        runfiles = runfiles.merge(dep[DefaultInfo].default_runfiles)
    
    # Also include the data dependencies (the raw files of this package)
    for data in ctx.attr.data:
        runfiles = runfiles.merge(data[DefaultInfo].default_runfiles)

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
