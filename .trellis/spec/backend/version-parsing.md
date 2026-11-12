# Version String Parsing Guideline

> Single source of truth for version-string semantics across providers.
> Born from the v0.16.x `jdk8u512-b09` / `8u504+1` routing bugs: version
> parsing was re-implemented per provider with drifting semantics, and the
> first dot-less version format broke all copies at once.

## Contract

All version-string judgments live in `internal/app/version.go`. Providers
MUST NOT implement their own extraction or classification:

| Helper | Semantics | Use for |
|---|---|---|
| `IsPureMajor(s)` | all digits, positive ("8"/"21") | routing: the ONLY input form meaning "latest of this major" |
| `MajorOfVersion(s)` | lenient extract: strip optional `jdk-`/`jdk`/`JDK-`/`JDK` prefix, then leading digits; 0 on failure | getting a major from any version form |
| `ParseMajorOfVersion(s)` | `MajorOfVersion` + unified「无效的版本号」error | provider `Resolve` pre-step |
| `ParseMajorVersion(s)` (pre-existing) | strict validation, errors on anything non-integer | commands that REQUIRE a major (`update`/`uninstall --all`) |

## Provider Resolve routing rule

Every `Resolve` must be a two-way branch — never enumerate version-string
formats:

```go
if app.IsPureMajor(v) {
    // latest-of-major path
} else {
    // exact-version path via the provider's ResolveReleaseName
}
```

A new distro or a new upstream naming format only requires that distro's
`ResolveReleaseName` to recognize the form. If you find yourself adding a
`strings.ContainsAny(v, ".")`-style condition to routing, stop and extend
`ResolveReleaseName` instead.

## Known real-world version forms (regression registry)

`internal/app/version_test.go` is the form registry. Verified against live
upstream APIs — extend it, not provider-local tests:

- Temurin: `21.0.5+11`, `25.0.4.1+1` (4-segment), `jdk-21.0.12+8`, **`jdk8u512-b09`** (JDK 8, dot-less)
- Corretto: `8.452.09.1`, `21.0.12.8.1`, `26.0.2.10.1` (dotted, variable length)
- Zulu: `8.0.452+9` (assembled from `java_version` array)
- Liberica: `21.0.5+11`, **`8u504+1`** (JDK 8, dot-less)
- Semeru: `21.0.13.0` (new 4-segment since 2026-04), `21.0.5+11_openj9-0.48.0` (legacy, `_openj9` tail omittable in Resolve), `8u402-b06_openj9-0.43.0` (JDK 8 legacy), `8.0.492.0`
- Dragonwell: `21.0.12.0.12+8`, `25.0.4.0.4+7`, `11.0.32.28` (no build segment), `8.30.29`
- Kona: `21.0.13.0.3` (5-segment since 2026-10), `17.0.4.1`, `11.0.33`, `8.0.28`
- Microsoft: LTS only, dotted; GraalVM: 21/25, dotted; temurin-ea: `28+14-ea-beta`

## Gotchas

- "No dot ⇒ pure major" is a **wrong** predicate (liberica `8u504+1` proved
  it: exact versions silently resolved to latest-of-major). Always all-digits.
- Loaded DLLs block both delete AND rename of a version dir (see uninstall
  messaging); unrelated to parsing but the same user-facing trap class.
- `CompareVersions`/`versionSegments` split on non-digits, so dot-less forms
  compare correctly already — never add provider-local comparison either.
- Directory names (`{distro}-{version}`) go through `junction.SplitDistro`
  first; `MajorOfVersion` does not know distro prefixes (`junction.MajorOf`
  composes the two).
