# Command reference

English | [简体中文](commands.zh-CN.md)

Back to [README](../README.md)

## Version management

```bash
jvm install 21                  # latest Temurin JDK 21 (default distribution)
jvm install corretto@21         # Amazon Corretto 21
jvm install microsoft@21        # Microsoft Build of OpenJDK 21
jvm install zulu@21             # Azul Zulu 21
jvm install liberica@21         # BellSoft Liberica 21
jvm install graalvm@21          # Oracle GraalVM 21 (CPU LTS line)
jvm install temurin-ea@28       # Temurin 28 early access (pre-GA preview)
jvm install 21.0.12+8           # exact Temurin version, build number included
jvm install temurin@21.0.5+11 D:\jdk.zip   # from a local zip — no network at all

jvm use 21                      # switch to 21 (latest patch in that major)
jvm use corretto@21             # switch to Corretto 21
jvm use                         # no argument: read .jvmrc from the current directory

jvm pin corretto@21             # pin this directory to Corretto 21 (writes .jvmrc)
jvm pin                         # write the currently active version to .jvmrc

jvm uninstall 21                # asks for confirmation; -y skips it.
                                # Uninstalling the active version falls back to the newest remaining one
jvm uninstall temurin@21 --all -y   # remove every installed patch in that major group

jvm exec 17 -- mvn test         # run with JDK 17 without touching the global version
jvm update 21 -y                # install the newest patch, switch, clean up the old one
jvm update --all -y             # upgrade every outdated group, one confirmation, failures don't block the rest
```

## Query

```bash
jvm list                    # installed versions, → marks the active one
jvm available               # installable majors, LTS marked, cached for 10 minutes
jvm available corretto      # versions available for Corretto
jvm available -a            # every sub-version of every major
jvm available --major 21    # every sub-version of JDK 21
jvm available -r            # bypass the cache and query live
jvm current                 # active version (runs java -version for real)
jvm home                    # print JAVA_HOME, for scripts and IDE configuration
jvm outdated                # which installed versions have a newer patch
jvm cache                   # download cache with file dates
jvm cache clean             # empty the cache
jvm cache clean --older-than 30d   # only files modified more than 30 days ago
jvm doctor                  # 14 checks: PATH, junction, JAVA_HOME, integration, leftovers
jvm doctor --fix            # diagnose and repair what can be repaired (-y skips prompts)
```

## Machine-readable output

`list`, `available`, `outdated` and `doctor` accept `--json` (short form `-j`). Only the output format changes — exit codes and cache behaviour are identical. Errors still go to stderr with a non-zero exit; stdout is always pure JSON or empty.

```bash
jvm list --json             # {"installed":[{distro,version,major,dir,current}...]}
jvm available --json        # flat rows; -a / --major return groups with LTS flags
jvm outdated --json         # {"groups":[{distro,major,local,latest,upgradable,failed}...]}
jvm doctor --json           # {"ok":bool,"checks":[{name,ok,detail,fix}...]} (mutually exclusive with --fix)
```

```bash
jvm list --json | jq '.installed[].version'
jvm outdated --json | jq '.groups[] | select(.upgradable)'
jvm doctor --json | jq -e '.ok'
```

`doctor` exits 0 even when checks fail, so read the `ok` field rather than the exit code.

## Project-level versions

Put a `.jvmrc` in the project root. Anyone running `jvm use` with no argument in that directory — or any subdirectory — gets the pinned version.

```bash
jvm pin corretto@21         # write .jvmrc in the current directory

# or create it by hand, one line, same syntax as the CLI:
#   21                    # latest Temurin 21
#   corretto@21.0.12.8.1  # distribution plus full version
```

The format is `[distro@]version`, `#` starts a comment.

`jvm use` walks up from the current directory to the nearest `.jvmrc`, so monorepos work.

`jvm pin` only writes the file; run `jvm use` to apply it. The version must already be installed — `use` reports the install command rather than downloading silently.

## Compatibility with sdkman / asdf / mise

Your teammates may use other version managers, and the files they commit are in their own formats. jvm reads them directly:

| File | Tool | Example | Read as |
|---|---|---|---|
| `.java-version` | long-standing convention | `21.0.2` | `21.0.2` |
| `.tool-versions` | asdf / mise | `java temurin-21.0.2+11` | `temurin@21.0.2+11` |
| `.sdkmanrc` | sdkman | `java=21.0.2-tem` | `temurin@21.0.2` |

When several coexist, precedence is `.jvmrc` > `.java-version` > `.tool-versions` > `.sdkmanrc`. Files without a Java entry (a `.tool-versions` with only `nodejs`, say) are skipped.

sdkman vendor suffixes map automatically: `-tem` → temurin, `-amzn` → corretto, `-ms` → microsoft, `-zul` → zulu, `-librca` → liberica, `-graal` → graalvm.

These ecosystems usually omit the build number, so jvm falls back to the newest installed version within the same major when an exact match fails. `.jvmrc` and CLI arguments are unaffected — they stay strict.

`jvm pin` only ever writes `.jvmrc`; it does not touch other tools' files.

## Automatic switching on cd

With a current integration script (rewritten automatically when jvm updates), entering a directory that has a `.jvmrc` or one of the compatible files switches to that version, and leaving restores what you had selected by hand.

```bash
jvm use 21          # manual selection: 21
cd D:\proj\legacy   # that directory has .jvmrc = 8  → switches to 8
cd ~                # leaving the project → back to 21
```

- No cost when neither the directory nor the rc file changed (two-layer cache, the shell does not spawn jvm on every prompt)
- If the pinned version is not installed, one line suggests `jvm install` instead of spamming
- A manual `jvm use` always wins and becomes the new restore baseline
- Disable it with `autoswitch = false` in `~/.jvm/config.toml`, or `JVM_AUTOSWITCH=0` for one session

## Shell integration

jvm writes a wrapper into `$PROFILE` (PowerShell 5.x and 7+) and `~/.bashrc` on first run, so `jvm use` refreshes `JAVA_HOME` and `PATH` in the **current** session. Reopen the terminal once after installation; after that there is nothing to configure.

The injection is idempotent — checked silently on every start, added only if missing. To opt out, delete the block marked `jvm shell init` from your profile.

```bash
jvm init powershell             # print the integration script
jvm init powershell --install   # write it to $PROFILE
jvm init bash --install         # write it to ~/.bashrc
```

COMMAND PROMPT is not supported: `cmd.exe` has no equivalent of `$PROFILE`, and doskey macros are a poor fit. Open a new window after switching.

## Configuration

Optional, at `~/.jvm/config.toml`:

```toml
# Download mirror (default: Tsinghua TUNA; only Temurin uses it)
mirror = "https://mirrors.tuna.tsinghua.edu.cn/Adoptium"

# Target architecture, defaults to the architecture of the jvm binary itself.
# Also accepts amd64 / arm64.
arch = "aarch64"

# Automatic switching on cd (default true)
autoswitch = true

# Move JDK storage to another volume. The control plane (config.toml, current/,
# registry entries) stays in ~/.jvm. Existing versions must be moved by hand;
# jvm doctor will remind you.
# install_dir = "D:\\jdks"
```

Environment variables override the file, for one-off use:

```powershell
$env:JVM_MIRROR = "https://your.mirror/Adoptium"
$env:JVM_ARCH = "aarch64"
$env:JVM_AUTOSWITCH = "0"
$env:JVM_INSTALL_DIR = "D:\jdks"
jvm install 21
```

## Self-update

```bash
jvm upgrade     # check GitHub Releases and update jvm itself
jvm version
jvm help
```

`jvm upgrade` reads `githubRepo` from `internal/upgrade/upgrade.go` (`BaixuanZhu/JVM`) and pulls the portable zip matching the current binary architecture. Pushing a `v*` tag triggers the release workflow.
