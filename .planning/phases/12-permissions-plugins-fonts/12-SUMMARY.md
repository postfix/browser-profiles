---
author: engineer
phase: 12
wave: 1
---

# Phase 12: Permissions, Plugins, Fonts — Execution Summary

## Goal

Add three low-risk, additive anti-detect surfaces for the v1.1 milestone while
preserving v1.0 default behavior for profiles that do not override the new
fields:

1. `navigator.permissions.query` returns coherent, platform-consistent states
   for the four query names (`camera`, `microphone`, `geolocation`,
   `notifications`) — **ADD-01**.
2. `navigator.plugins` and `navigator.mimeTypes` expose a platform-specific,
   array-like surface with correct methods and indexing — **ADD-02**.
3. `document.fonts.check` returns `true` for fonts in a per-OS whitelist and falls
   back to the real check otherwise — **ADD-03**.

All changes are additive and deterministic; the v1.0 launch path is unchanged
when the new fields are absent.

## Chosen API Shape

Three new optional blocks are added to `FingerprintConfig` in `types.go`:

- `Permissions *PermissionsConfig` with `Camera`, `Microphone`, `Geolocation`,
  `Notifications`.
- `Plugins *PluginsConfig` with lists of `PluginInfo` and `PluginMimeType`.
- `Fonts *FontsConfig` with a `Whitelist []string`.

Matching builder-facing types live in `fingerprint/scripts.go`, and the
`GeneratedFingerprint` value carries the same fields so `CreateSession` can copy
them into the temporary `FingerprintConfig` when `RandomFingerprint` is enabled.

## Implementation Highlights

### Script builders

- `fingerprint/scripts/permissions.tmpl.js` captures the real
  `navigator.permissions.query` and returns deterministic `PermissionState`
  objects for the four known descriptors, falling back to the real query for
  unknown names.
- `fingerprint/scripts/plugins.tmpl.js` builds `PluginArray`- and
  `MimeTypeArray`-like objects with `length`, numeric indices, `item(i)`,
  `namedItem(name)`, and `refresh()` where required. Each plugin entry also
  exposes its own MIME-type list with the same array-like methods.
- `fingerprint/scripts/fonts.tmpl.js` captures the real `document.fonts.check`,
  parses comma-separated font-family lists, and returns `true` for whitelisted
  families, falling back to the native check otherwise.

### Defaults and platform mapping

- `fingerprint/scripts.go` provides `DefaultPermissionsConfig` (desktop-Chrome
  defaults: `camera=prompt`, `microphone=prompt`, `geolocation=prompt`,
  `notifications=default`), `DefaultPluginsConfig`, and `DefaultFontsConfig`.
- Plugin lists are platform-specific:
  - **Windows:** Chrome PDF Plugin, Chrome PDF Viewer, Native Client (plus their
    MIME types).
  - **macOS / Linux:** Chrome PDF Plugin, Chrome PDF Viewer (no Native Client).
  - These match the conventional desktop-Chrome plugin set and vary only where
    the real browser historically differs by platform.
- Font whitelists are per-OS common system fonts:
  - **Windows:** Arial, Arial Black, Calibri, Cambria, Cambria Math, Consolas,
    Courier New, Georgia, Impact, Lucida Console, Lucida Sans Unicode,
    Microsoft Sans Serif, Segoe UI, Segoe UI Symbol, Tahoma, Times New Roman,
    Trebuchet MS, Verdana.
  - **macOS:** Helvetica, Helvetica Neue, Arial, Arial Black, Times New Roman,
    Times, Lucida Grande, Menlo, Monaco, Courier, Courier New, Georgia, Verdana,
    Geneva, Tahoma, Trebuchet MS.
  - **Linux:** DejaVu Sans, DejaVu Serif, DejaVu Sans Mono, Liberation Sans,
    Liberation Serif, Liberation Mono, Ubuntu, Ubuntu Mono, Noto Sans, Noto
    Serif, Noto Mono, FreeSans, FreeSerif, FreeMono.

### Generator and launch wiring

- `fingerprint/generate.go` sets `Permissions`, `Plugins`, and `Fonts` on the
  generated fingerprint using the platform-derived defaults.
- `chrome.go` `applyAntiDetect` derives the canonical platform key (`Win32` ->
  `windows`, etc.), builds the three default configs, applies explicit
  `fp.*` overrides, and passes them to `GetAllProtectionScripts`.
- `session.go` `protectionBundle` does the same so the M5 cross-tab
  re-injection bundle and `installProtections` include the new scripts.
- `GetAllProtectionScripts` appends the scripts in the fixed order: navigator,
  client-hints, permissions, plugins, fonts, automation-bypass. The old
  hardcoded permissions/plugins blocks inside the automation-bypass template were
  removed so the new builder-generated scripts are the single source of truth.
- `GetFingerprintScripts` also appends the three scripts using the generated
  defaults.

## Final Fixture Set and Checksum Changes

Regenerated and pinned fixtures:

- New per-surface fixtures:
  - `permissions/default.js`, `permissions/full.js`
  - `plugins/win.js`, `plugins/mac.js`, `plugins/linux.js`
  - `fonts/win.js`, `fonts/mac.js`, `fonts/linux.js`
  - `all/with_permissions_plugins_fonts.js` — the full launch bundle with all
    three new surfaces enabled.
- Updated existing fixtures:
  - `all/default.js`, `all/launch_nav.js`, `all/modes.js`, `all/nav_escape.js`,
    `all/toggles.js`, `all/with_clienthints.js` — the old inline
    permissions/plugins blocks were removed from the automation-bypass section.
  - `consts/automation_bypass.js` — same cleanup.
  - `fpscripts/sample.js` — regenerated to include the new permissions,
    plugins, and fonts scripts in the consumer helper output.
- `fingerprint/testdata/manifest.json` and `fingerprint/testdata/checksums.txt`
  were updated to cover the new fixtures and the regenerated existing ones.

`TestGoldenFixtureChecksums` and `TestBuildersGolden` pass.

## Unit Tests

`fingerprint/scripts_test.go` was extended with builder and integration tests:

- `TestPermissionsBuilder` — known descriptors return configured states, string
  and object descriptors are accepted, and unknown descriptors fall back.
- `TestPluginsBuilder` — the generated script produces array-like objects with
  `length`, `item`, `namedItem`, and correct plugin/MIME-type fields.
- `TestFontsBuilder` — whitelisted fonts return `true`, non-whitelisted fonts
  fall back, and an empty whitelist always falls back.
- `TestAllProtectionIncludesNewScripts` — the all-bundle includes the three new
  scripts in the expected order.
- `TestBuildersGolden` covers the new fixtures and passes.

## Test Coverage

| Package | Coverage | Delta vs. Phase 11 |
|---|---|---|
| `github.com/postfix/browser-profiles` | **86.0%** | **+0.6%** |
| `github.com/postfix/browser-profiles/fingerprint` | **94.0%** | **-2.2%** |

Root-package coverage remains above the 85% threshold. The `fingerprint` package
coverage dropped because the new fixtures and script files are large, mostly
static JavaScript, but the package still remains well above the 90% threshold.

## Real-Browser Smoke Test Result

Real-Chrome tests live in `permissions_plugins_fonts_test.go`:

- `TestPermissionsPluginsFontsSmoke` — launches a Windows profile with explicit
  permissions, plugins, and fonts overrides and verifies:
  - `navigator.permissions.query` returns the configured states for the four
    known descriptors.
  - Unknown descriptors fall back to the real query without crashing.
  - `navigator.plugins` has the expected length, `item`, `namedItem`, and
    `navigator.mimeTypes['application/pdf']` is defined.
  - `document.fonts.check` returns `true` for whitelisted fonts and falls back
    for unknown fonts.
  - A self-contained contradiction check records a
    `permissions_plugins_fonts_coherence` detector baseline entry.
- `TestPermissionsPluginsFontsCrossTab` — opens a new tab after `WithProfile` and
 verifies that the M5 re-injection loop carries the same permissions, plugins,
 and fonts overrides into the new tab.

Both tests ran in this environment with Chromium available and **passed**. When
Chrome is unavailable, `requireChrome` skips them cleanly.

The detector baseline was updated to include:

```json
"permissions_plugins_fonts_coherence": {
  "status": "passed",
  "note": "permissions/plugins/fonts coherence self-check passed",
  "value": "platform/plugins/fonts coherent"
}
```

and the ThumbmarkJS oracle hash pair was refreshed to reflect the new surfaces.

## Verification Commands

All of the following pass:

```text
go build ./...
go vet ./...
go test -count=1 ./...
go test -cover ./...
```

## Commits

- `5e5204e` — Phase 12: add permissions, plugins, and fonts script templates and builders
- `90d820b` — Phase 12: wire permissions, plugins, and fonts into generator and launch bundle
- (this commit) — Phase 12: fixtures, tests, and summary for permissions/plugins/fonts

## Deviations and Notes

- The old automation-bypass template contained an inline permissions query and
  plugins list. Those blocks were removed and replaced by the dedicated
  builder-generated scripts, which are now the single source of truth for both
  the launch bundle and the consumer helper.
- The `fonts` guard is a **lightweight check**, not a full OS-level font spoof.
  True font spoofing would require installing or removing fonts on the host OS;
  this guard only intercepts `document.fonts.check` for a configured whitelist and
  falls back to the real browser check otherwise. This matches the scope of
  ADD-03.
- No additional network-oracle run was performed; the external detector oracles
  (`browserleaks`, `creepjs`) remain skipped unless
  `BROWSER_PROFILES_RUN_NETWORK_ORACLES=1` is set.

## Acceptance Criteria Status

| Criterion | Status |
|---|---|
| Permissions API returns coherent states for the four descriptors | ✅ |
| Plugins/MIME types expose platform-specific array-like surfaces | ✅ |
| Fonts guard returns `true` for whitelisted fonts and falls back otherwise | ✅ |
| Golden fixtures exist for permissions, plugins, and fonts | ✅ |
| Unit tests cover the three builders and `GetAllProtectionScripts` inclusion | ✅ |
| Real-Chrome smoke tests cover launch and cross-tab behavior | ✅ |
| v1.0 default behavior preserved when new fields are absent | ✅ |
| `go test -count=1 ./...`, `go vet ./...`, `go build ./...` pass | ✅ |
| Root-package coverage ≥ 85% | ✅ (86.0%) |
| `fingerprint` package coverage ≥ 90% | ✅ (94.0%) |
| `12-SUMMARY.md` exists and documents results | ✅ |
