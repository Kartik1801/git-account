# git-account

Use several Git accounts on one computer without mixing them up. Works the same on Windows, macOS and Linux.

Run `git-account` and you get a terminal app where you add each account once: a name, an email and an SSH key. After that git picks the right one by itself:

- **Automatic folders:** every repo inside `~/work` uses your work account, now and later, in the terminal and in your IDE.
- **Use here:** pin an account to one repo.
- **Default:** everything else uses your default account.

## Install

Download the archive for your system from the releases page, unpack it, and put `git-account` on your `PATH`. With Go 1.26 or newer you can also run:

```sh
go install github.com/kartik-dhawan-dev/git-account/cmd/git-account@latest
```

You need git 2.13 or newer and OpenSSH. On Windows, git-account uses the built-in OpenSSH client (Settings > System > Optional features > OpenSSH Client).

**[Full install guide](docs/install.md)**: step by step for Windows, macOS and Linux, the `.deb` and `.rpm` packages, building from source, shell completion, manual pages, updating, uninstalling, and moving from the old `git-account.sh` script.

## Quick start

```sh
git-account
```

Press `a` to add an account. The wizard asks four short questions:

1. **You:** a short name (like `work`), your name and email. These are filled in from your git config.
2. **Host:** GitHub, GitLab, Bitbucket or your own server, and your username there.
3. **Key:** create a new ed25519 key (recommended), or use one you already have.
4. **Connect:** the public key is copied to your clipboard and the host's SSH key page opens. Paste it there. The app waits until the host accepts the key and checks that it signed in as the username you gave. That catches keys added while you were signed in to the wrong account.

At the end you can pick a folder where the account should apply automatically.

## Keys

| Key     | Home screen                                                                                                 |
| ------- | ----------------------------------------------------------------------------------------------------------- |
| `↑` `↓` | choose an account                                                                                           |
| `enter` | use it in this repo                                                                                         |
| `→`     | account actions: copy key, open key page, test, load into agent, folders, edit, rename, replace key, remove |
| `y`     | copy the public key                                                                                         |
| `a`     | add an account                                                                                              |
| `c`     | check everything and fix problems                                                                           |
| `s`     | pick a theme                                                                                                |
| `u`     | undo the last change                                                                                        |
| `?`     | every key                                                                                                   |

## Commands

You don't have to open the app. Everything it does is also a command, for daily use, scripts and dotfiles. Most accept `--json`.

```sh
git-account status                 # which account applies here, and why
git-account list
git-account use work               # pin work to this repo
git-account use personal --global  # make personal the default
git-account folder add work ~/work # automatic in a folder
git-account copy-key work          # public key to the clipboard
git-account show-key work          # print it
git-account open work              # open the host's SSH key page
git-account test                   # check each key signs in as the right user
git-account check --fix            # find and fix problems
git-account clone work git@github.com:acme/api.git
git-account replace-key work       # new key; switches once the host accepts it
git-account add work --name "Kartik Dhawan" --email kartik@acme.com --username kartik-acme --folder ~/work
git-account undo
git-account history
git-account theme dracula
git-account uninstall
```

For help, run `git-account --help` for every command or `git-account <command> --help` for one, with examples.

**[Command guide](docs/commands.md)**: every command and flag by task, what happens outside a terminal, exit codes and the JSON formats.

## How it works

Each account is a small git config file with `user.name`, `user.email` and a `core.sshCommand` that uses only that account's key (`IdentitiesOnly=yes`, so ssh-agent can't offer another account's key first).

git-account adds one marked block to the end of your global `~/.gitconfig`:

```ini
# >>> git-account (managed, do not edit) >>>
[include]
	path = "/home/you/.config/git-account/gitconfig"
# <<< git-account <<<
```

That file includes the default account and one `includeIf "gitdir:…"` rule per automatic folder, so git itself picks the account. **Use here** adds an `include.path` to the repo's own `.git/config`.

Being last in `~/.gitconfig` lets the default account win over a `[user]` section you already had. Nothing else in your files is changed.

### Files

|                                                   | Linux and macOS                 | Windows                                     |
| ------------------------------------------------- | ------------------------------- | ------------------------------------------- |
| Settings (`config.toml`), generated files, themes | `~/.config/git-account`         | `%APPDATA%\git-account`                     |
| Backups, trash                                    | `~/.local/state/git-account`    | `%LOCALAPPDATA%\git-account`                |
| Keys                                              | `~/.ssh/git-account/<account>/` | `%USERPROFILE%\.ssh\git-account\<account>\` |

`config.toml` is the source of truth. You can edit it by hand and run `git-account sync`.

## Safety

- **Undo:** every change backs up the files it touches first. `u` in the app or `git-account undo` reverts the last change; `git-account history` lists them.
- **No half-written files:** files are written to a temporary file and renamed into place. A change that fails partway puts every file back. A lock stops two windows from changing things at once.
- **Keys aren't deleted:** removing an account or replacing a key moves keys git-account created to the trash for 30 days. Keys you brought yourself stay where they are.
- **Private keys stay private:** new keys are readable only by you (mode 600, or an owner-only access list on Windows). Check finds and fixes keys other users can read.
- **Clean uninstall:** `git-account uninstall` removes exactly the lines and files git-account added, and `git-account undo` reverses it.

## Check and fix

Press `c` (or run `git-account check`) to test:

- git and OpenSSH are installed, and ssh-agent is running (on Windows, whether the agent service is running or disabled)
- `~/.gitconfig` includes git-account, and nothing after it overrides your default account
- nothing overrides each account's key: a global `core.sshCommand`, the `GIT_SSH_COMMAND` variable, or a `Host github.com` block with its own `IdentityFile`
- each key exists, is readable only by you, is loaded in the agent if it has a passphrase, and isn't more than a year old
- each account connects and signs in as the right username
- whether port 22 is blocked; if port 443 works, one key press switches that host to SSH over 443

Most problems have a fix you run with `enter`.

## Themes

Press `s` to pick one, with a live preview, or run `git-account theme <name>`. There are 23:

- **Dark:** git-account, Catppuccin Mocha, Dracula, Nord, Gruvbox, Tokyo Night, One Dark, Monokai, Solarized Dark, Everforest, Kanagawa, Rosé Pine, Ayu Dark, GitHub Dark, Synthwave '84
- **Light:** Rosé Pine Dawn, Solarized Light, GitHub Light, Catppuccin Latte, Gruvbox Light, One Light
- **Accessibility:** High contrast, No color

**Auto** (the default) follows your terminal's light or dark background and keeps its own background color; the others also paint their own background. Every theme passes a readability test: body text at 4.5:1 contrast or better, secondary text and status colors at 3:1, including on the selected row. A few light themes use a slightly darker shade of their official colors to get there.

Status always has a symbol (✓ ! ✗) as well as a color, so nothing depends on color alone. `NO_COLOR` is respected.

To add your own, save a file like this as `themes/<name>.toml` in the settings folder. Any color you leave out comes from the default theme.

```toml
name = "My theme"
kind = "dark"          # or "light"
bg = "#1e1e2e"
fg = "#cdd6f4"
dim = "#7f849c"
faint = "#45475a"
edge = "#313244"
accent = "#cba6f7"
ok = "#a6e3a1"
info = "#89b4fa"
warn = "#f9e2af"
err = "#f38ba8"
row_sel = "#313244"
strong = "#ffffff"
```

## Development

```sh
go test ./...
go run ./cmd/git-account
```

Tests run real git against a sealed temporary home folder, so they never touch your own config. CI runs them on Linux, macOS and Windows. Tagging `v*` builds release archives with GoReleaser.

To check the screens' layout without a terminal, `DUMP=<folder> go test ./internal/tui -run TestDump` writes each screen as plain text.

## License

MIT
