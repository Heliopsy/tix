# Shell completion

`tix completion SHELL` writes a completion script to standard output. bash, zsh and fish are supported; anything
else is exit 2.

Completion is not only for command and flag names. The scripts complete task references, project keys, tags and
statuses by querying the target the current directory resolves to, so `tix task mv <TAB>` offers real references
and `tix task ls --status <TAB>` offers the states your workflows actually define. A lookup that cannot reach the
target offers nothing rather than hanging.

## bash

Requires `bash-completion` to be installed and sourced by your shell.

System-wide:

```sh
tix completion bash | sudo tee /etc/bash_completion.d/tix > /dev/null
```

Per user:

```sh
mkdir -p ~/.local/share/bash-completion/completions
tix completion bash > ~/.local/share/bash-completion/completions/tix
```

On macOS with Homebrew:

```sh
tix completion bash > "$(brew --prefix)/etc/bash_completion.d/tix"
```

Open a new shell, or `source` the file.

## zsh

```sh
tix completion zsh > "${fpath[1]}/_tix"
```

If completion is not initialized yet, add this to `~/.zshrc` above the line that writes the file:

```sh
autoload -Uz compinit
compinit
```

Per user, without touching a system directory:

```sh
mkdir -p ~/.zsh/completions
tix completion zsh > ~/.zsh/completions/_tix
# in ~/.zshrc, before compinit:
fpath=(~/.zsh/completions $fpath)
```

zsh caches completions. After replacing the file, `rm -f ~/.zcompdump*` and start a new shell.

## fish

```sh
tix completion fish > ~/.config/fish/completions/tix.fish
```

fish loads it on the next prompt; no restart needed.

## Trying it without installing

```sh
source <(tix completion bash)        # bash
tix completion fish | source         # fish
```

Lasts for the current shell only, which is the quick way to check the script works before deciding where to put
it.

## Keeping it current

The script is generated from the command tree at the version that produced it, so regenerate it after an upgrade.
If completion starts offering flags that no longer exist, that is a stale script rather than a bug.

Dynamic completion runs the binary, so it resolves the same target your commands do. In a directory with a
`.tix.yaml` pinning a project, completion offers that project's references. See
[configuration.md](configuration.md).
