# oh-my-gossh

`gossh` is a terminal picker for the hosts in your ssh config: open a shell,
power a host off, or copy the paths you launched it with. Positional arguments
are files and directories to offer for transfer, typically passed by a file-manager
action (nemo's `%F`); with none, only the operations that need no selection are
shown.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/brohd11/oh-my-gossh/main/install.sh | sh
```

Installs the latest release to `~/.local/bin` (override with `BIN_DIR`) and
offers to put that directory on your PATH. Pin a release with
`VERSION=vX.Y.Z`, or skip the PATH prompt with `--no-modify-path` /
`--modify-path`.

## Update

```sh
gossh update
```

Checks for a newer release and, when one exists, installs it in place by
running the same `install.sh` against the directory the running binary lives
in.

## Development

```sh
make                # build into build/<os>-<arch>/
./install_unix.sh   # symlink the dev build into ~/.local/bin
go test ./...
```

Releases are built and published by the GitHub Actions workflow when a `v*`
tag is pushed.
