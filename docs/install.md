# Installing git-account

git-account is one program, `git-account`. Put it anywhere on your `PATH` and run `git-account`.

- [What you need](#what-you-need)
- [Download a release](#download-a-release)
- [Install with Go](#install-with-go)
- [Build from source](#build-from-source)
- [Check that it works](#check-that-it-works)
- [Shell completion](#shell-completion)
- [Manual pages](#manual-pages)
- [Updating](#updating)
- [Uninstalling](#uninstalling)
- [Coming from the old git-account.sh script](#coming-from-the-old-git-accountsh-script)

## What you need

|                   | Check with      | If it's missing                                        |
| ----------------- | --------------- | ------------------------------------------------------ |
| git 2.13 or newer | `git --version` | [git-scm.com/downloads](https://git-scm.com/downloads) |
| OpenSSH client    | `ssh -V`        | see below                                              |

**OpenSSH on Windows.** Windows 10 (1809 and later) and Windows 11 include it. If `ssh -V` fails, open Settings > System > Optional features > View features, and add **OpenSSH Client**. Or, in PowerShell as administrator:

```powershell
Add-WindowsCapability -Online -Name OpenSSH.Client~~~~0.0.1.0
```

git-account uses this Windows OpenSSH, not the ssh that comes with Git for Windows, because only the Windows one can use the Windows ssh-agent service.

**OpenSSH on macOS** is built in.

**OpenSSH on Linux:**

```sh
sudo apt install openssh-client      # Debian, Ubuntu
sudo dnf install openssh-clients     # Fedora, RHEL
sudo pacman -S openssh               # Arch
```

### Optional

- **ssh-agent**, if you want keys with a passphrase without typing it on every push.
  - Windows: the "OpenSSH Authentication Agent" service. It's often disabled. `git-account check` finds that and offers to turn it on (it asks for administrator rights). To do it yourself, in PowerShell as administrator:
    ```powershell
    Set-Service ssh-agent -StartupType Automatic
    Start-Service ssh-agent
    ```
  - macOS: always running. git-account also saves passphrases in Keychain.
  - Linux: most desktops start one. If `ssh-add -l` says it can't connect, add `eval "$(ssh-agent -s)"` to your shell profile.
- **A clipboard tool on Linux**, for "copy key": `xclip`, `xsel` or `wl-clipboard` (Wayland). Without one, git-account prints the key instead. WSL uses Windows' clipboard.

## Download a release

Once a version is published, each release on GitHub has an archive for every system:

| System                              | File                                                   |
| ----------------------------------- | ------------------------------------------------------ |
| Windows (most PCs)                  | `git-account_<version>_windows_amd64.zip`              |
| Windows on ARM                      | `git-account_<version>_windows_arm64.zip`              |
| macOS, Apple silicon (M1 and later) | `git-account_<version>_darwin_arm64.tar.gz`            |
| macOS, Intel                        | `git-account_<version>_darwin_amd64.tar.gz`            |
| Linux                               | `git-account_<version>_linux_amd64.tar.gz` or `_arm64` |
| Debian, Ubuntu                      | the `.deb` file                                        |
| Fedora, RHEL                        | the `.rpm` file                                        |

`checksums.txt` lists the SHA-256 of every file.

### Windows

In PowerShell, from the folder you downloaded the zip to:

```powershell
$dir = "$env:LOCALAPPDATA\Programs\git-account"
New-Item -ItemType Directory -Force $dir | Out-Null
Expand-Archive .\git-account_*_windows_amd64.zip -DestinationPath $dir -Force

# Add the folder to your PATH, for your user only (no admin needed).
# This edits the registry directly so entries like %USERPROFILE%\go\bin
# stay as variables; [Environment]::SetEnvironmentVariable would expand them.
$path = (Get-Item HKCU:\Environment).GetValue("Path", "", "DoNotExpandEnvironmentNames")
if ($path -notlike "*$dir*") {
  Set-ItemProperty HKCU:\Environment -Name Path -Type ExpandString -Value "$($path.TrimEnd(';'));$dir"
  # Tell Windows the environment changed, so new windows pick it up.
  [Environment]::SetEnvironmentVariable("GIT_ACCOUNT_SETUP", "1", "User")
  [Environment]::SetEnvironmentVariable("GIT_ACCOUNT_SETUP", $null, "User")
}
```

Or add it by hand: Start menu > "Edit environment variables for your account" > Path > Edit > New, then paste `%LOCALAPPDATA%\Programs\git-account`.

Terminals that were already open keep their old `PATH`. Close Windows Terminal or VS Code completely and reopen it, or refresh the current window:

```powershell
$env:Path = [Environment]::GetEnvironmentVariable("Path","Machine") + ";" + [Environment]::GetEnvironmentVariable("Path","User")
```

To check the download first:

```powershell
(Get-FileHash .\git-account_*_windows_amd64.zip -Algorithm SHA256).Hash.ToLower()
```

and compare it with the line in `checksums.txt`.

### macOS and Linux

```sh
tar -xzf git-account_*_linux_amd64.tar.gz      # or the darwin file on a Mac
mkdir -p ~/.local/bin
mv git-account ~/.local/bin/
```

`~/.local/bin` is on the `PATH` on most Linux systems. If `git-account` isn't found, add this line to `~/.bashrc` or `~/.zshrc` and open a new terminal:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

On macOS a file downloaded with a browser is quarantined, and the first run is blocked. Clear the flag once:

```sh
xattr -d com.apple.quarantine ~/.local/bin/git-account
```

To check the download first:

```sh
sha256sum --ignore-missing -c checksums.txt          # Linux
shasum -a 256 git-account_*_darwin_arm64.tar.gz      # macOS: compare with checksums.txt
```

### Debian, Ubuntu, Fedora

The packages put the program in `/usr/bin` and install the manual pages:

```sh
sudo apt install ./git-account_*.deb    # Debian, Ubuntu
sudo dnf install ./git-account_*.rpm    # Fedora, RHEL
```

## Install with Go

With Go 1.26 or newer:

```sh
go install github.com/kartik-dhawan-dev/git-account/cmd/git-account@latest
```

This works once the code is published on GitHub. The program goes into Go's `bin` folder: `~/go/bin` on macOS and Linux, `%USERPROFILE%\go\bin` on Windows. Make sure that folder is on your `PATH`.

## Build from source

With Go 1.26 or newer and git:

```sh
git clone https://github.com/kartik-dhawan-dev/git-account.git
cd git-account
go build -ldflags "-X main.version=$(git describe --tags --always)" -o git-account ./cmd/git-account
```

On Windows, in PowerShell:

```powershell
go build -ldflags "-X main.version=$(git describe --tags --always)" -o git-account.exe ./cmd/git-account
```

Then move the program onto your `PATH` as in [Download a release](#download-a-release). To run the tests first: `go test ./...`.

## Check that it works

```sh
git-account version     # prints the version
git-account -h          # lists every command
git-account check       # checks git, OpenSSH, ssh-agent and your setup
git-account             # opens the app
```

If PowerShell says `The term 'git-account' is not recognized`, or another shell says `git-account: command not found`, the terminal can't find the program. Usually it was opened before the `PATH` changed: close it completely and open a new one, or refresh it with the `$env:Path = ...` line from the Windows steps above. If a new terminal still can't find it, check that the folder with `git-account` is really on your `PATH`.

## Shell completion

**bash**

```sh
mkdir -p ~/.local/share/bash-completion/completions
git-account completion bash > ~/.local/share/bash-completion/completions/git-account
```

**zsh**

```sh
mkdir -p ~/.zsh/completions
git-account completion zsh > ~/.zsh/completions/_git-account
# in ~/.zshrc, before compinit:
fpath=(~/.zsh/completions $fpath)
```

**fish**

```sh
git-account completion fish > ~/.config/fish/completions/git-account.fish
```

**PowerShell**

```powershell
git-account completion powershell | Out-String | Invoke-Expression
# to keep it, add that line to your profile:
notepad $PROFILE
```

Completion fills in command names, flags and your account names.

## Manual pages

You don't need manual pages for help: `git-account --help` and `git-account <command> --help` always work. They're there for people who prefer `man`.

The Debian and Fedora packages install them. When you install another way on Linux or macOS, you can add them from a source checkout:

```sh
go run ./tools/man ./man
sudo mkdir -p /usr/local/share/man/man1
sudo cp man/*.1 /usr/local/share/man/man1/
```

After that, `man git-account` and `man git-account-use` work.

## Updating

Install the new version the same way, over the old one. Your accounts, keys and settings aren't stored next to the program, so they stay as they are. A newer git-account reads settings written by an older one.

## Uninstalling

1. Remove what git-account added to your git and SSH config:

   ```sh
   git-account uninstall
   ```

   This removes git-account's block from `~/.gitconfig` and `~/.ssh/config` and deletes the files it generated. Your accounts stay in `config.toml` and your keys stay in `~/.ssh/git-account`, so reinstalling brings everything back. Add `--purge` to also delete `config.toml` and move the keys git-account created to its trash folder.

2. Delete the program: the file you put on your `PATH`, or `sudo apt remove git-account` / `sudo dnf remove git-account`.

3. Optionally delete git-account's folders:

   |                   | Linux and macOS              | Windows                          |
   | ----------------- | ---------------------------- | -------------------------------- |
   | Settings          | `~/.config/git-account`      | `%APPDATA%\git-account`          |
   | Backups and trash | `~/.local/state/git-account` | `%LOCALAPPDATA%\git-account`     |
   | Keys              | `~/.ssh/git-account`         | `%USERPROFILE%\.ssh\git-account` |

   Delete the keys folder only if you've removed those keys from GitHub, GitLab and so on, or no longer need them.

## Coming from the old git-account.sh script

The old zsh script was installed under the same name, usually `/usr/local/bin/git-account`. Remove it first so the new program is the one that runs:

```sh
which -a git-account           # lists every git-account on your PATH
sudo rm /usr/local/bin/git-account
```

Your old keys in `~/.ssh/git-accounts/` still work. When adding an account in the app, choose **Use a key I already have** and pick one, or on the command line:

```sh
git-account add work --key ~/.ssh/git-accounts/id_rsa_work --email you@acme.com --username you-acme
```

The old script set a global `core.sshCommand` that forces one key for every repo. `git-account check` finds it and removes it with `--fix`. You can then delete `~/.git-accounts` and the `Host github.com-<name>` blocks it added to `~/.ssh/config`.
