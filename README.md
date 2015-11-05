# apt-mirror-go

A tool mirroring debian packages (almost) compatible with `apt-mirror`.

## We already have apt-mirror

Yes, and `apt-mirror` does well in most situations. But this project does not aim to __replace__ `apt-mirror`. It is just a practice.

`apt-mirror-go` also has a few of new features:

* Better multithread downloading. You would never wait the last thread to finish for hours.
* Transfer rate limiting. In case you have to share internet connection with your coworkers.
* Support `Contents` and i18n files. No need to write custom post-mirror script if you need `apt-file`.

There are also some bad news:

* You don't need Perl and `wget`, but you need `gzip`, `xz` and `bzip2`.
* Memory footprint is much bigger than `apt-mirror`, ate ~200mb memory with ~24000 package files.
* It's way much slower cleaning out-dated files with current implementation.

## Usage

```sh
apt-mirror-go [-n] [/path/to/mirror.list]
```

You can use `-n` to disable package downloading and file cleaning, but info files (`Sources`, `Contents`, `Packages`, `Release` and i18n files) will be downloaded.

## Configuration

### Variables

Use `set` keyword to declare variables.

```
set base_path /var/spool/apt-mirror
set skel_path $base_path/skel
```

`apt-mirror-go` will use these variables:

- `skel_path`: path to place temporary files.
- `mirror_path`: path to put mirrored files.
- `defaultarch`: default architecture.
- `nthreads`: spawn this number of goroutines for file downloading, must be an integer.
- `ratelimit`: limit transfer rate (kb) for http, must be an integer. 
- `translations`: i18n files to download, space delimited.

Variables are parsed line by line, so `skel_path` will be `/a/b` and `mirror_path` will be `/c/d` in following example:

```
set base_path /a
set skel_path $base_path/b
set base_path /c
set mirror_path $base_path/d
```

### Repositories

The way specify what to download should be exactly same as you did in `apt-mirror`:

```
deb http://ftp.debian.org/debian stable main contrib non-free
deb-i386 http://ftp.debian.org/debian stable main contrib non-free
deb-src http://ftp.debian.org/debian stable main contrib non-free
```

`apt-mirror-go` supports only `http` at this time.

### Files to be cleaned

To be compitable with `apt-mirror`, you have to specify directories in URL format:

```
clean http://ftp.debian.org
clean http://other.server/subdir/pool
```

### Comments

Every line starts with `#` will be treat as comment. Inline comments are not supported.

## TODO

1. Write comments to describe every component and program work flow.
2. Support post-mirror script like `apt-mirror` does.
3. Support https and ftp.
4. Optimize memory usage by changing how and what info to be cached.
5. Optimize the algorithm to clean out-dated files.
6. Extract gzip, xz and bzip2 without external programs.
7. Add command line option or configuration variable to enable checksum validating.

## License

You can choose any version of GPL, or GPLv3 if you don't want to choose one.
