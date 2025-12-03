def _nix_package_impl(repository_ctx):
    # Tools
    resolve_tool = repository_ctx.path(Label("//:nix-bazel-resolve"))
    fetch_tool = repository_ctx.path(Label("//:nix-bazel-fetch"))
    generate_tool = repository_ctx.path(Label("//:nix-bazel-generate"))

    # Build tools if needed (assuming they are pre-built or built by user for now)
    # Ideally, we should build them here if they don't exist, but repository_rule has limits.
    # We will assume the user runs `go build` or we provide a wrapper.

    if repository_ctx.attr.lockfile:
        # 1. Read lockfile
        lockfile_path = repository_ctx.path(repository_ctx.attr.lockfile)
        lockfile_content = repository_ctx.read(lockfile_path)
        lock = json.decode(lockfile_content)

        # 2. Collect all unique store paths (already flat)
        unique_paths = lock.get("packages", {})

        # 3. Download and unpack each store path
        for store_path, node in unique_paths.items():
            # Download
            download_path = repository_ctx.path("downloads/" + node["fileHash"])
            repository_ctx.download(
                url = "https://cache.nixos.org/" + node["url"],
                output = download_path,
                sha256 = node["fileHash"],
            )

            # Unpack and patch
            # We need to pass references for RPATH
            refs_list = node.get("references") or []
            refs = ",".join(refs_list)
            
            args = [
                fetch_tool,
                "--archive", str(download_path),
                "--store-path", store_path,
                "--out", repository_ctx.path("."),
            ]
            if refs:
                args.extend(["--refs", refs])

            result = repository_ctx.execute(args)
            if result.return_code != 0:
                fail("Failed to unpack %s: \n%s\n%s" % (store_path, result.stdout, result.stderr))

        # 4. Generate BUILD files
        if repository_ctx.attr.packages_json:
            # Generate BUILD files
            args = [generate_tool, "--lockfile", lockfile_path, "--out", repository_ctx.path(".")]
            if repository_ctx.attr.channel:
                args.extend(["--channel", repository_ctx.attr.channel])
                
            result = repository_ctx.execute(args)
            if result.return_code != 0:
                fail("Failed to generate BUILD files: \n%s\n%s" % (result.stdout, result.stderr))

        # Copy patchelf.bzl
        repository_ctx.symlink(Label("//:patchelf.bzl"), "patchelf.bzl")

    else:
        fail("Lockfile is required for this mode")

nix_package = repository_rule(
    implementation = _nix_package_impl,
    attrs = {
        "package_id": attr.string(mandatory = False), # Optional if using lockfile
        "nix_store_hash": attr.string(mandatory = False),
        "lockfile": attr.label(mandatory = False), # Path to lockfile
        "repository_name": attr.string(mandatory = False), # Name in lockfile
        "packages_json": attr.string(mandatory = False), # JSON string of packages from MODULE.bazel
        "channel": attr.string(mandatory = False), # Nix channel (Hydra jobset)
    },
)

def _nix_extension_impl(module_ctx):
    # We only support one lockfile for now (or merge them?)
    # Let's assume one main lockfile passed to the first tag.
    lockfile = None
    channel = ""
    packages = {}

    for mod in module_ctx.modules:
        for tag in mod.tags.packages:
            if tag.lockfile:
                lockfile = tag.lockfile
            if tag.channel:
                channel = tag.channel
        
        for pkg in mod.tags.package:
            packages[pkg.name] = pkg.package

    # Serialize packages to JSON
    packages_json = json.encode({"repositories": packages})
    
    if lockfile:
        nix_package(
            name = "nix_deps",
            lockfile = lockfile,
            packages_json = packages_json,
            channel = channel,
        )

nix_extension = module_extension(
    implementation = _nix_extension_impl,
    tag_classes = {
        "packages": tag_class(
            attrs = {
                "lockfile": attr.label(mandatory = True),
                "channel": attr.string(mandatory = False),
            },
        ),
        "package": tag_class(
            attrs = {
                "name": attr.string(mandatory = True),
                "package": attr.string(mandatory = True),
            },
        ),
    },
)
