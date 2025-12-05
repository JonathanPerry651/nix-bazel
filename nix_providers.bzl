NixStorePathInfo = provider(
    doc = "Information about a unpacked Nix store path",
    fields = {
        "nar_file": "The NAR archive file",
        "store_path": "The original /nix/store path (string)",
    },
)
