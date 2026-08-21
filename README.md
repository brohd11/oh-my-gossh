# oh-my-gossh

`gossh` is a terminal picker for the hosts in your ssh config: open a shell,
power a host off, or copy the paths you launched it with. Positional arguments
are files and directories to offer for transfer, typically passed by a file-manager
action (nemo's `%F`); with none, only the operations that need no selection are
shown.

## Install

Unix:
```sh
curl -fsSL https://raw.githubusercontent.com/brohd11/oh-my-gossh/main/install.sh | sh
```

Windows:

```powershell
irm https://raw.githubusercontent.com/brohd11/oh-my-gossh/main/install.ps1 | iex
```

To update:
```
gossh update
```

More install details (location, flags, etc): [shared install reference](https://github.com/brohd11/goutil/blob/main/docs/install.md).

<sub>macOS note: a binary downloaded **in a browser** gets quarantined by Gatekeeper — clear it
with `xattr -dr com.apple.quarantine path/to/binary`. This doesn't apply to the installer
above; the attribute is set by browsers, not by `curl`.</sub>