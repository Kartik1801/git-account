# Using git-account from the command line

You never have to open the app. Every action in it is also a command, and most commands can print JSON for scripts.

- [Getting help](#getting-help)
- [All commands](#all-commands)
- [Which account am I using?](#which-account-am-i-using)
- [Adding an account](#adding-an-account)
- [Choosing the account for a repo](#choosing-the-account-for-a-repo)
- [Keys and connections](#keys-and-connections)
- [Changing and removing accounts](#changing-and-removing-accounts)
- [Checking and fixing problems](#checking-and-fixing-problems)
- [Undo and history](#undo-and-history)
- [Themes](#themes)
- [Uninstalling](#uninstalling)
- [Scripting](#scripting)

## Getting help

```sh
git-account --help               # every command, grouped
git-account use --help           # one command, with examples
git-account help folder add      # the same, another way
```

`-h` is short for `--help`. Every command's help has examples.

If you get a command's arguments wrong, the error shows the right form, an example and your account names:

```text
$ git-account use
✗ Usage: git-account use <account>
  for example: git-account use work
  your accounts: personal, work
```

## All commands

| Command                                        | What it does                                                                  |
| ---------------------------------------------- | ----------------------------------------------------------------------------- |
| `git-account`                                  | Open the app. Outside a terminal, prints the status instead.                  |
| **Everyday**                                   |                                                                               |
| `git-account status`                           | Which account applies here, and why                                           |
| `git-account list`                             | Your accounts                                                                 |
| `git-account use <account>`                    | Use an account in this repo; with `--global`, make it the default             |
| `git-account unpin`                            | Stop pinning an account to this repo                                          |
| `git-account default <account>`                | Make an account the default                                                   |
| `git-account clone <account> <url> [folder]`   | Clone with an account's key and pin the new repo to it                        |
| **Accounts and folders**                       |                                                                               |
| `git-account add <account>`                    | Add an account                                                                |
| `git-account edit <account>`                   | Change its name, email or username                                            |
| `git-account rename <account> <new-name>`      | Rename it                                                                     |
| `git-account remove <account>`                 | Remove it (its key goes to the trash for 30 days)                             |
| `git-account folder add <account> <folder>`    | Use an account for every repo inside a folder                                 |
| `git-account folder remove <account> <folder>` | Stop a folder from choosing an account                                        |
| `git-account folder list`                      | List automatic folders                                                        |
| **Keys and connections**                       |                                                                               |
| `git-account show-key <account>`               | Print the public key                                                          |
| `git-account copy-key <account>`               | Copy the public key to the clipboard                                          |
| `git-account open <account>`                   | Open the host's SSH key page                                                  |
| `git-account test [account...]`                | Check each key signs in as the right user                                     |
| `git-account load-key <account>`               | Load a key with a passphrase into ssh-agent                                   |
| `git-account replace-key <account>`            | Create a new key and switch to it once the host accepts it                    |
| **Safety and setup**                           |                                                                               |
| `git-account check`                            | Find problems; `--fix` fixes them                                             |
| `git-account undo`                             | Undo the last change                                                          |
| `git-account history`                          | Changes you can undo                                                          |
| `git-account sync`                             | Rebuild git-account's files after editing `config.toml` by hand               |
| `git-account theme [name]`                     | List themes or pick one                                                       |
| `git-account uninstall`                        | Remove git-account's lines and files                                          |
| `git-account version`                          | Print the version                                                             |
| `git-account completion <shell>`               | Shell completion script (see [Shell completion](install.md#shell-completion)) |

Some commands have short aliases: `ls` for `list`, `current` and `whoami` for `status`, `switch` for `use`, `rm` for `remove`, `doctor` for `check`.

## Which account am I using?

```text
$ git-account status
work  automatic in ~/work
  Kartik Dhawan <kartik@acme.com> · github.com
```

The reason is one of:

- **set for this repo**: pinned with `git-account use`
- **automatic in ~/work**: the repo is inside one of the account's folders
- **your default**: nothing else applies

If the repo's email comes from somewhere git-account doesn't manage, such as the repo's own `user.email`, `status` says which file it comes from.

```text
$ git-account list
● work      kartik@acme.com      GitHub · auto ~/work
  personal  kartik@example.com   GitHub · default
```

The dot marks the account that applies in the current folder. Both commands take `--json`.

## Adding an account

```sh
git-account add work
```

In a terminal it asks for anything you didn't pass: your name and email (filled in from your git config), your username on the host, and a passphrase for the new key. Then:

1. A new ed25519 key is created in `~/.ssh/git-account/work/`.
2. The public key is printed and copied to the clipboard, and the host's SSH key page opens.
3. You paste the key there. The command waits until the host accepts it.
4. If you gave a username, it checks the key signs in as that user. A key added while you were signed in to the wrong account is caught here.

The account is saved only after that, so a failed or cancelled add leaves nothing behind.

With everything as flags, for scripts:

```sh
git-account add work \
  --name "Kartik Dhawan" \
  --email kartik@acme.com \
  --username kartik-acme \
  --folder ~/work
```

| Flag                 | Meaning                                                                              |
| -------------------- | ------------------------------------------------------------------------------------ |
| `--name`             | Your name for commits. Required outside a terminal.                                  |
| `--email`            | Your email for commits. Required outside a terminal.                                 |
| `--username`         | Your username on the host. Used to check the key lands on the right account.         |
| `--host`             | `github` (default), `gitlab`, `bitbucket` or `custom`                                |
| `--hostname`         | The server, for GitHub Enterprise, self-hosted GitLab or `custom`                    |
| `--port`             | SSH port, if it isn't 22                                                             |
| `--folder`           | Use the account automatically in this folder. Repeat it for more folders.            |
| `--key`              | Use an existing private key instead of creating one                                  |
| `--key-type`         | Type of new key: `ed25519` (default) or `rsa`, for servers that don't accept ed25519 |
| `--passphrase-stdin` | Read the new key's passphrase from standard input                                    |
| `--no-verify`        | Save without waiting for the host to accept the key                                  |
| `--no-browser`       | Don't open the host's SSH key page                                                   |
| `--wait`             | How long to wait for the host, like `5m` (default `10m`)                             |

More examples:

```sh
# GitLab.com
git-account add oss --host gitlab --email me@example.com --username me

# GitHub Enterprise
git-account add corp --host github --hostname github.corp.example --email me@corp.example --username me

# A self-hosted Gitea on port 2222
git-account add home --host custom --hostname git.home.example --port 2222 --email me@example.com

# A key you already have
git-account add old --key ~/.ssh/id_ed25519 --email me@example.com --username me

# In a script, with a passphrase and without waiting
printf '%s\n' "$PASSPHRASE" | git-account add ci --name CI --email ci@example.com --passphrase-stdin --no-verify
```

## Choosing the account for a repo

Three ways, from most to least specific:

**Pin an account to one repo.** Run it inside the repo:

```sh
git-account use work
```

If the repo had its own `user.name`, `user.email` or `core.sshCommand`, they're removed, because they'd win over the account. The command says so. `git-account unpin` removes the pin.

**Pick an account by folder.** Every repo inside the folder uses it, now and later:

```sh
git-account folder add work ~/work
git-account folder add client-x ~/clients/x
git-account folder list
git-account folder remove work ~/work
```

git itself applies folder rules, so they work in IDEs and GUI clients too. A deeper folder wins over the folder around it.

**Set a default** for everywhere else:

```sh
git-account use personal --global      # or: git-account default personal
```

**Cloning.** A repo that isn't cloned yet has no folder or pin, so a private repo outside your folders would be cloned with the default key. `clone` uses the right key from the start and pins the new repo:

```sh
git-account clone work git@github.com:acme/api.git
git-account clone work git@github.com:acme/api.git ~/code/api
```

## Keys and connections

```sh
git-account show-key work        # print the public key
git-account show-key work > work.pub
git-account copy-key work        # copy it to the clipboard
git-account open work            # open the host's SSH key page, and print the fingerprint to look for
git-account test                 # test every account
git-account test work            # test one
git-account load-key work        # load a key with a passphrase into ssh-agent
```

`test` connects with only that account's key and checks who the host says you are:

```text
$ git-account test
✓ work → github.com says Hi @kartik-acme
✗ personal: github.com didn't accept the key
  Add it on GitHub's SSH keys page. The fix copies the key and opens the page.
```

**Replacing a key**, for example once a year or after a laptop was lost:

```sh
git-account replace-key work
```

It creates a new key, copies it and opens the host's page. Once the host accepts the new key, the account switches to it and the old one goes to the trash. Until then nothing changes and the old key keeps working. It prints the old key's fingerprint so you can delete it on the host. It takes `--passphrase-stdin`, `--no-browser` and `--wait` like `add`.

## Changing and removing accounts

```sh
git-account edit work --email kartik@acme-corp.com
git-account edit work --name "K. Dhawan" --username kdhawan
git-account rename work acme
git-account remove old-client
git-account remove old-client --yes     # no question, for scripts
```

`edit` changes new commits only; commits you already made keep their author. Pinned repos and folders follow a rename. `remove` moves a key git-account created to the trash for 30 days and leaves a key you brought yourself where it is. `git-account undo` brings a removed account back.

## Checking and fixing problems

```sh
git-account check          # report
git-account check --fix    # fix everything that has a fix
git-account check --json
```

It checks git, OpenSSH and ssh-agent; that your global git config includes git-account and nothing after it overrides your default; settings that override account keys (a global `core.sshCommand`, `GIT_SSH_COMMAND`, a `Host` block with its own `IdentityFile`); each key's file, permissions, age and agent status; and each account's connection. If port 22 is blocked and port 443 works, the fix switches that host to SSH over port 443.

Some fixes need you at the terminal: loading a key asks for its passphrase, and starting the Windows ssh-agent service asks for administrator rights. Without a terminal those are skipped.

## Undo and history

```sh
git-account history
git-account undo
```

Every change backs up the files it touches first. `undo` puts them back; run it again to go further. The last 30 changes are kept.

If you edit `config.toml` by hand, run `git-account sync` to rebuild the files that come from it.

## Themes

```sh
git-account theme            # list, with the current one marked
git-account theme dracula
git-account theme auto       # follow the terminal's light or dark background
```

Themes only affect the app. Your own themes are `.toml` files in the `themes` folder next to `config.toml`; see [Themes](../README.md#themes).

## Uninstalling

```sh
git-account uninstall
git-account uninstall --purge --yes
```

`uninstall` removes git-account's block from your global git config and `~/.ssh/config`, and the files it generated. Accounts stay in `config.toml` and keys stay put. `--purge` also deletes `config.toml` and moves the keys git-account created to the trash. `git-account undo` reverses either. See [Uninstalling](install.md#uninstalling) for removing the program itself.

## Scripting

**No prompts outside a terminal.** When standard input or output isn't a terminal, nothing asks questions:

- `add` needs `--name` and `--email`
- `remove` and `uninstall` need `--yes`
- `git-account` with no command prints the status instead of opening the app
- fixes that need the terminal are skipped by `check --fix`

**Exit status** is 0 on success and 1 on an error. `test` and `check` also exit with 1 when something needs attention, so they work in CI:

```sh
git-account check --json > report.json || echo "git-account found problems"
```

**JSON output**

`git-account list --json`:

```json
[
  {
    "id": "work",
    "name": "Kartik Dhawan",
    "email": "kartik@acme.com",
    "host": "github",
    "domain": "github.com",
    "username": "kartik-acme",
    "key": "/home/kartik/.ssh/git-account/work/id_ed25519",
    "folders": ["~/work"],
    "default": false
  }
]
```

`git-account status --json`. `kind` is `repo`, `folder`, `default`, `other` or `none`:

```json
{
  "account": "work",
  "email": "kartik@acme.com",
  "file": "",
  "folder": "~/work",
  "kind": "folder",
  "repo": "/home/kartik/work/api"
}
```

`git-account check --json`. `level` is `ok`, `info`, `warn` or `problem`:

```json
[
  {
    "level": "problem",
    "title": "personal: github.com didn't accept the key",
    "detail": "Add it on GitHub's SSH keys page. The fix copies the key and opens the page.",
    "account": "personal",
    "fix": "Copy the key and open the page"
  }
]
```

**Environment variables**

| Variable            | Effect                                                                                 |
| ------------------- | -------------------------------------------------------------------------------------- |
| `NO_COLOR`          | No colors in the app or in command output                                              |
| `GIT_ACCOUNT_DIR`   | Keep git-account's settings and backups in this folder instead of the usual places     |
| `GIT_CONFIG_GLOBAL` | git's own setting; git-account writes its block to that file instead of `~/.gitconfig` |
