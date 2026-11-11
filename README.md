# jvm — JDK version manager for Windows

English | [简体中文](README.zh-CN.md)

Install multiple JDKs, switch between them in milliseconds, no admin rights required. Temurin, Corretto, Microsoft, Zulu, Liberica, and GraalVM.

<!-- TODO: drop a 5s GIF here — `jvm use 21` then `java -version` changing in the same terminal.
     It converts better than everything else on this page combined. -->

## Why another one

Windows has no `sdkman`. Every alternative costs you something:

| Option | Where it breaks |
|---|---|
| Editing `JAVA_HOME` by hand | Restart the terminal and the IDE on every switch |
| sdkman | Bash only; JDKs inside WSL are invisible to Windows IDEs |
| jabba | Unmaintained |
| Chocolatey / Scoop | Installs fine, switching is still manual |
| `setx` for `PATH` | Silently truncates anything past 1024 characters |

jvm only fixes the switching half. `PATH` and `JAVA_HOME` are written once and point at a junction. Switching a version rewrites that junction — milliseconds, no elevation, and every new terminal picks it up.

## Install

```powershell
iwr -useb "https://raw.githubusercontent.com/BaixuanZhu/JVM/main/install.ps1" | iex
```

Reopen the terminal once afterwards. Windows 10+, no admin rights, no Go toolchain.

Prefer a GUI installer or a portable zip? Both are on [Releases](https://github.com/BaixuanZhu/JVM/releases).

## Usage

```bash
jvm install 21              # latest Temurin JDK 21
jvm use 21                  # switch — this terminal is current immediately
jvm install corretto@21     # or a different distribution
```

Drop a `.jvmrc` in a project root to pin its JDK. `jvm use` with no argument walks up from the current directory, so subdirectories inherit the setting.

It also reads `.java-version`, `.tool-versions` (asdf / mise) and `.sdkmanrc`. A team does not have to adopt jvm for you to use it.

Full command reference, `--json` output and configuration: [docs/commands.md](docs/commands.md)

## How it works

```
~/.jvm/
  versions/
    temurin-21.0.12+8/
    corretto-21.0.12.8.1/
  current/          ← junction pointing at the active version
  config.toml
```

`PATH` points at `~/.jvm/current/bin` and `JAVA_HOME` at `~/.jvm/current`, both written once at install time. Switching rebuilds the junction.

A junction is an NTFS reparse point and is not gated by `SeCreateSymbolicLinkPrivilege`, so it needs no elevation the way a symlink does. jvm creates it with `FSCTL_SET_REPARSE_POINT` directly rather than shelling out to `mklink`. The trade-off: a junction can only target a directory on a local volume, which is fine for JDKs.

The second half is harder. A child process cannot modify its parent shell's environment, so `jvm.exe` alone could never make `java -version` change in the terminal you just typed into. On first run jvm installs a wrapper into `$PROFILE` (PowerShell 5.x and 7+) and `~/.bashrc`; the wrapper runs the binary and, for `use`, refreshes `JAVA_HOME` and `PATH` in the current session.

## Supported distributions

| Distribution | Prefix | Windows ARM64 |
|---|---|---|
| Eclipse Temurin | `temurin` (default) | Partial |
| Temurin early access | `temurin-ea` | Partial |
| Amazon Corretto | `corretto` | — |
| Microsoft Build of OpenJDK | `microsoft` | Yes |
| Azul Zulu | `zulu` | Yes |
| BellSoft Liberica | `liberica` | Yes |
| Oracle GraalVM | `graalvm` | — |

Each vendor publishes a different API shape — indexmap JSON, two-step metadata queries, SHA1-only checksums, CDN probing. All of it is normalised behind a `Provider` interface in `internal/provider/`.

## Design decisions

| Problem | Choice | Why |
|---|---|---|
| Switch mechanism | Windows junction | No elevation, works in new terminals |
| Junction creation | Native `FSCTL_SET_REPARSE_POINT` | No `cmd.exe` in the path, no injection surface |
| Current-session effect | Shell wrapper injected on startup | A child process cannot touch the parent's environment |
| `PATH` injection | `HKCU\Environment` + `WM_SETTINGCHANGE` | `setx` truncates long `PATH` values |
| Download source | Tsinghua mirror first, official CDN fallback | Fast in mainland China, still correct if the mirror lags |
| Distribution | Six vendors plus Temurin EA | All ship plain zips; no MSI installers to unpackage |

## Build

Needs Go 1.26+ and GNU Make.

```bash
make build       # → dist/<arch>/jvm.exe (dev build, does not touch PATH or profile)
make build-dist  # release build, keeps startup self-bootstrap
make installer   # NSIS installer, requires scoop install nsis
```

Release artefacts are produced by GoReleaser. Pushing a `v*` tag triggers the workflow.

## Licence

[MIT](LICENSE)
