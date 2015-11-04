# apt-mirror-go

A tool mirroring debian packages (almost) compatible with `apt-mirror`.

## How it works

1. Read config file to build the list of repositories to mirror.
2. Fetch the `Release`, `Packages` and `Contents` files, including their variants like `Release.gpg`, `Packages.gz`.
3. Iterate and parse the `Pacakges` file, create a file list.
4. Comparing with local mirrored files, separate the files out-dated and files pending for downloading.
5. Download the files and verify their size and md5 checksum.
6. Clean-up the out-dated files, includes `Release`, `Packages` and `Contents` files.
7. Move downloaded files to mirror path.
