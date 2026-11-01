# Release Pipeline (GoReleaser 混合模式)

> Source of truth for the release pipeline introduced 2026-10-31 (task `10-31-goreleaser-migration`).
> Read this before touching `.goreleaser.yaml`, `.github/workflows/release.yml`, `Makefile` release targets, or adding release assets.

## Scenario: GoReleaser hybrid release pipeline

### 1. Scope / Trigger

- Trigger: any change to release assets, asset names, checksums, NSIS installer wiring, or the release workflow.
- Division of labor: **goreleaser** owns compile (amd64+arm64) / portable zip / `checksums.txt` / GitHub Release creation; **Makefile** owns the NSIS installer (`installer/jvm.nsi` custom logic goreleaser templates cannot cover). NSIS runs inside goreleaser `before.hooks` (`make installer ×2`), artifacts uploaded via `release.extra_files`.

### 2. Signatures (asset name contract — three consumers, do not break)

| Asset | Consumer(s) |
|---|---|
| `jvm-windows-{amd64,arm64}.zip` | `internal/upgrade/upgrade.go` `expectedAssetName()` exact match; `install.ps1` `releases/latest/download/` |
| `checksums.txt` | `upgrade.go` `parseChecksum` + `install.ps1` `Get-ExpectedChecksum`, both line-match by filename |
| `jvm-windows-{amd64,arm64}-setup.exe` / `install.ps1` | humans / one-line install |

- `checksums.txt` must contain exactly the two zips (GNU sha256sum text format). `extra_files` do NOT enter checksums (goreleaser default) — that is the contract, not an accident.
- Future 4th consumers (winget manifest / scoop manifest) must point at the same URLs (see parent task `10-31-release-automation`).

### 3. Contracts (goreleaser-specific, empirically verified on v2.19.0)

- `dist: release-dist` — goreleaser requires its own dist dir to be EMPTY after `before.hooks`. `jvm.nsi` hardcodes `..\dist\` and must not change → goreleaser gets its own dir. Makefile `clean` removes both.
- `{{ .Version }}` DOES expand inside `before.hooks` commands (verified: `make installer VERSION={{ .Version }}` receives the tag version). Design fallback (pre-step in workflow) not needed.
- `release.header` from-file loading (`from_file`) is **Pro-only**; OSS writes the path literally. Release body = CI post-step `gh release edit "$GITHUB_REF_NAME" --notes-file dist/release-header.md`, only when the file is non-empty (empty → keep goreleaser default changelog).
- `archives.files: [none*]` is the documented empty-glob trick to produce a zip containing ONLY `jvm.exe` (default glob pulls LICENSE/README/CHANGELOG in).
- checksum file name must be forced: `checksum.name_template: "checksums.txt"` (default is `<project>_checksums.txt`, invisible to consumers).
- Version pinning: workflow pins goreleaser CLI `version: v2.19.0` + action major `@v7` (v6 hits GitHub's Node 20 deprecation; v7 inputs unchanged, adds goreleaser binary checksum verification). Upgrade protocol: bump local goreleaser first, re-run snapshot, then bump CI.
- CHANGELOG body extraction (hard constraint in AGENTS.md): `scripts/gen-release-header.sh` awk-extracts the `## [<version>]` section; always emits the file (empty on miss, exit 0).

### 4. Validation & Error Matrix

- **CI rehearsal without tagging**: `gh workflow run release.yml --ref main` (workflow_dispatch) runs the full pipeline in snapshot mode (`--snapshot --skip=publish`) — all before hooks, both builds, archives, checksums — but creates no Release. Use this after any pipeline change; verified green 2026-11-01.
- `go vet`/`go test` red in before hooks → release aborts before creating the Release (the gate, unchanged from old pipeline).
- CHANGELOG section missing → empty header file → CI skips `gh release edit`, goreleaser default changelog body remains.
- `dist is not empty` error → someone made hooks write into goreleaser's dist dir again; keep the two dirs decoupled.

### 5. Good/Base/Bad Cases

- Good: `goreleaser release --snapshot --skip=publish --clean` from a clean tree → `release-dist/` has 2 zips + `checksums.txt` (exactly 2 lines), `dist/` has 2 setup.exe.
- Base: snapshot on a dev machine — every exe run must set `JVM_NO_BOOTSTRAP=1` (release-flavor binaries self-bootstrap: they append their own dir to user PATH + ensure shell integration).
- Bad: changing zip name_template / adding extra_files into checksums / running snapshot exes bare on a dev machine.

### 6. Tests Required

- Local snapshot run is the release gate before any `.goreleaser.yaml` change: matrix check (zip names, zip contains single root-level `jvm.exe`, checksums exactly 2 lines `sha256sum -c` OK, setup.exe ×2).
- Full end-to-end (tag → 5 assets → old `jvm upgrade` self-update → `install.ps1`) lives in the parent task integration checklist — only verifiable at a real release window.

### 7. Wrong vs Correct

#### Wrong

```yaml
# header from file (Pro-only; OSS writes the path literally into the body)
release:
  header: dist/release-header.md
```

#### Correct

```yaml
# CI post-step does: gh release edit "$GITHUB_REF_NAME" --notes-file dist/release-header.md
dist: release-dist          # own dir; Makefile/NSIS keep dist/
checksum:
  name_template: "checksums.txt"
```

## Common Mistake: verifying release-flavor binaries on a dev machine

**Symptom**: user PATH gains `<repo>\dist\<arch>` (HKCU\Environment), `$PROFILE`/`~/.bashrc` may gain/upgrade integration blocks.

**Cause**: release flavor keeps bootstrap ON (only `make build`/`make run` inject `Bootstrap=off`); any bare run self-bootstraps.

**Fix**: remove the entry via `internal/env.RemoveFromUserPath` (one-off `go run` main inside the repo — type-preserving registry write + broadcast). Do NOT `setx`/PowerShell-direct-write PATH (type truncation/downgrade risk).

**Prevention**: always run snapshot/dist binaries with `JVM_NO_BOOTSTRAP=1` (+ `JVM_HOME` pointing at a temp dir when you want full isolation). A single deliberate bare run is the only way to *verify* bootstrap-on semantics — plan its cleanup in the same step.
