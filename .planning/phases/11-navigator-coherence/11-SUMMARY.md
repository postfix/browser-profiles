---
author: engineer
phase: 11
wave: 1
---

# Phase 11: Navigator Coherence — Execution Summary

## Goal

Make every `navigator.*` value coherent with the generated profile persona, move
User-Agent Client Hints (`navigator.userAgentData` + `Sec-CH-UA*` headers) into
the core launch path, and make WebGL numeric capabilities stable per profile by
choosing them from a GPU-family-specific table. Preserve v1.0 default behavior
and backward compatibility for existing profiles that lack the new fields.

## Chosen API Shape

The data model is extended through new fields on the existing
`FingerprintConfig` and `WebGLConfig` types, plus matching
`fingerprint` package structs:

- `FingerprintConfig` gained `AppVersion`, `ProductSub`, `Vendor`,
  `MaxTouchPoints`, `Mobile`, `Connection`, and `ClientHints`.
- `WebGLConfig` gained `Caps *WebGLCaps` with realistic GPU-family parameters.
- `GeneratedFingerprint` and `WebGLInfo` mirror the new fields so
  `CreateSession` can copy them from the random generator.
- `fingerprint.NavigatorConfig`, `ClientHintsScriptConfig`, and
  `AllProtectionOptions` carry the same fields through to the script builders.

v1.0 defaults are preserved: when the new fields are absent, the injected bundle
is byte-identical to the pre-Phase-11 default output and runtime behavior is
unchanged.

## Implementation Highlights

### Generator and cap table

- `fingerprint/generate.go` derives coherent navigator values (`AppVersion`,
  `ProductSub`, `Vendor`, `MaxTouchPoints`, `Mobile`, `Connection`) from the
  chosen persona.
- `webGLCapsForFamily(family, seed)` returns a stable, deterministic WebGL
  parameter set per GPU family (`intel`, `nvidia`, `amd`, `apple`). Relaunching
  the same profile yields the same caps; different profiles vary within the
  family.

### Script builders

- `navigator.tmpl.js` installs `appVersion`, `productSub`, `maxTouchPoints`,
  `mobile`, and `connection` getters when the config provides them.
- `clienthints.tmpl.js` tokenizes the full version (`%%UA_FULL_VERSION%%`) and
  falls back to the same default when the token is unset.
- `webgl.tmpl.js` tokenizes numeric caps while keeping the per-profile GPU
  identity spoof and the default randomized behavior.
- `GetAllProtectionScripts` now appends the client-hints script when
  `ClientHints` is set, placed after the navigator script so the page sees a
  coherent `navigator.userAgentData`.

### Launch and session wiring

- `chrome.go`:
  - `applyAntiDetect` derives coherent `Network.setUserAgentOverride` metadata
    including the brand list and `fullVersionList`.
  - New helper `applyNetworkUserAgentOverride` encapsulates the
    `Network.enable` + `Network.setUserAgentOverride` call with the corrected
    `EmulationUserAgentMetadata` (including `FullVersionList`, which Chrome uses
    for `Sec-CH-UA-Full-Version-List`).
- `session.go`:
  - `protectionBundle` carries the new navigator, client-hints, and WebGL caps
    config into the cross-tab re-injection bundle.
  - `attachSession` now applies `applyNetworkUserAgentOverride` on the
    session's default page, ensuring the override survives the CDP reconnection
    performed by `WithProfile` and works in the session-reuse path where
    `applyAntiDetect` may not run.
  - `installProtections` applies the network override to every existing page and
    to each new page created via `TargetTargetCreated`, so Sec-CH-UA* headers
    are coherent across tabs (M5).
  - `patchPageScript` passes client-hints config through when a fingerprint is
    provided.

## Final Fixture Set and Checksum Changes

Regenerated and re-pinned fixtures:

- `navigator/launch.js`, `navigator/full_escape.js`, `navigator/empty.js`
- `navigator/coherence.js` (new, exercises all navigator coherence fields)
- `clienthints/default.js`, `clienthints/full.js`
- `clienthints/full_version.js` (new)
- `webgl/custom.js`, `webgl/escape.js`
- `webgl/caps.js` (new)
- `all/launch_nav.js`, `all/nav_escape.js`
- `all/with_clienthints.js` (new)
- `fpscripts/sample.js`
- `testdata/manifest.json` and `testdata/checksums.txt`

`TestGoldenFixtureChecksums` and `TestBuildersGolden` pass.

## Test Coverage

| Package | Coverage |
|---|---|
| `github.com/postfix/browser-profiles` | **85.4%** |
| `github.com/postfix/browser-profiles/fingerprint` | **96.2%** |

Root-package coverage remains above the 85% threshold.

## Real-Browser Smoke Test Result

Real-Chrome tests live in `navigator_coherence_test.go`:

- `TestNavigatorCoherenceSmoke` — verifies `navigator.userAgent`, `appVersion`,
  `productSub`, `vendor`, `maxTouchPoints`, `userAgentData.mobile`,
  `navigator.connection`, `userAgentData.platform`, `userAgentData.brands`,
  `getHighEntropyValues` for architecture and platform version, and WebGL
  vendor/renderer/caps.
- `TestSecCHUAHeaders` — verifies the network request carries coherent
  `Sec-CH-UA`, `Sec-CH-UA-Mobile`, `Sec-CH-UA-Platform`, and
  `Sec-CH-UA-Full-Version-List` headers. The test server delivers an
  `Accept-CH` header and the browser is navigated a second time, which is the
  standard Chrome client-hints handshake.
- `TestNavigatorCoherenceCrossTab` — verifies the protection bundle reaches a
  newly opened tab.
- `TestNavigatorCoherenceDetectorContradiction` — self-contained coherence
  check that records a `navigator_coherence` entry in the detector baseline.

All four tests ran in this environment with Chromium available and **passed**.
When Chrome is unavailable, `requireChrome` skips them gracefully.

## Verification Commands

All of the following pass:

```text
go build ./...
go vet ./...
go test -count=1 ./...
go test -cover ./...
```

## Commits

- `bfe9cc4` — Phase 11: navigator coherence, client hints wiring, and tests

## Deviations and Notes

- The original `Network.setUserAgentOverride` metadata only set the deprecated
  `FullVersion` field. Chrome emits `Sec-CH-UA-Full-Version-List` from
  `FullVersionList`, so the wiring was corrected to populate both fields. This
  is the root cause that made `TestSecCHUAHeaders` fail in the cancelled run.
- The `Sec-CH-UA-Platform` header is a structured-field string and arrives
  quoted (`"Windows"`), so the test normalizes the value before asserting.
- The `Accept-CH` + second-request dance is the pragmatic way to observe
  request-header client hints in a headless Chrome test; the alternative would
  be asserting only JavaScript-side `navigator.userAgentData`, which does not
  verify that the headers actually leave the browser.
- The temporary `cmd/regenfixtures` helper was removed before commit; it is
  not referenced by any test or build step.
- No additional detector-oracle run was performed beyond the embedded
  `TestNavigatorCoherenceDetectorContradiction` contradiction check; the
  detector baseline entry was recorded by that test.

## Acceptance Criteria Status

| Criterion | Status |
|---|---|
| Navigator coherence fields derived from persona and injected | ✅ |
| Client hints in core launch bundle and `navigator.userAgentData` | ✅ |
| WebGL caps stable per profile via GPU-family table | ✅ |
| v1.0 default behavior preserved when new fields are absent | ✅ |
| `Network.setUserAgentOverride` metadata includes `FullVersionList` | ✅ |
| Network override applied to session page and every new page | ✅ |
| Real-Chrome tests skip gracefully when Chrome is unavailable | ✅ |
| Golden fixtures and checksums consistent | ✅ |
| `go test -count=1 ./...`, `go vet ./...`, `go build ./...` pass | ✅ |
| Root-package coverage ≥ 85% | ✅ (85.4%) |
| `11-SUMMARY.md` exists and documents results | ✅ |
