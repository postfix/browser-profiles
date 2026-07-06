---
author: engineer
phase: 09
status: done
---

# Phase 09: Test Hardening — Summary

## Goal

Raise root-package test coverage, harden golden fixtures, confirm Chrome-free unit tests,
keep integration tests hermetic, and establish a skip-safe detector-oracle baseline.

## Coverage (HARD-02)

- **Baseline:** 77.6% statements
- **Final:** 85.1% statements (`go test . -cover`)
- Target ≥ 85% met.

### Highest-impact coverage additions

| Function | Before | After | Test(s) added |
|---|---|---|---|
| `LaunchByName` / `LaunchByIdOrName` / `Launch` | 0–80% | 83–100% | `TestLaunchByNameNotFound`, `TestLaunchByIdOrNameNotFound`, `TestLaunchMissingProfile`, `TestLaunchByNameSuccessSkipsWithoutChrome` |
| `BrowserError.Error` / `Unwrap` | 0% | 100% | `TestBrowserErrorError`, `TestBrowserErrorUnwrap` |
| `Port.UnmarshalJSON` error path | 66.7% | 77.8% | `TestPortUnmarshalInvalidString` |
| `buildLauncher` | 60.5% | 92.1% | `TestBuildLauncherOptions` (args, headless, extensions) |
| `dirExists` / `terminateProcess` / `firstNonEmpty` | 0–66.7% | 100% | `TestDirExists`, `TestTerminateProcessNoPanic`, `TestFirstNonEmpty` |
| `parseWSPort` | 75% | 100% | `TestParseWSPortInvalid` |
| `systemTimezone` localtime branch | 83.3% | 83.3% | `TestSystemTimezoneFromLocaltime` (added `localtimeReadlink` seam) |
| `readLockFile` | 85.7% | 100% | `TestReadLockFileMalformed` |
| `applyUpdate` | 61.1% | 72.2% | `TestApplyUpdateClearNotes`, `TestApplyUpdateEmptyStartUrls`, `TestApplyUpdateClearGroupIDAndTags` |
| `httpBackend.dial` / `plain`, `socks5Backend.plain` | 0–66.7% | 67–90.9% | `TestForwardProxyHTTPPlainErrorDial`, `TestForwardProxyHTTPBackendConnectRejected`, `TestForwardProxySOCKS5Plain`, `TestSOCKS5BackendPlainError` |
| `forwardProxy.Close` | 66.7% | 87.5% | `TestForwardProxyClose` (made idempotent with `closed`/`mu`) |
| `AutoDetectTimezone` | 50% | 100% | `TestAutoDetectTimezoneWithProxy` |
| `Close` (launch orchestration) | 0% | 100% | `TestLaunchOrchestrationClose` |

### Known gaps (require real Chrome)

The following functions still have uncovered branches because they only execute when
real Chrome is launched. When Chrome is absent they are skipped, so they do not count
toward the browser-free coverage metric:

- `LaunchChrome` (61.2%)
- `LaunchByName` / `LaunchByIdOrName` success path (real launch)
- `WithProfile` (60.0%)
- `attachSession` (63.2%)
- `CreateSession` success path (80.0%)
- `defaultPage` (66.7%)
- `installProtections` / `applyAntiDetect` (covered by real-Chrome integration tests but skipped when Chrome is absent)

## Golden Fixtures (HARD-03)

- Every public anti-detect builder in `fingerprint/scripts.go` is guarded by a golden
  fixture in `fingerprint/testdata/`:
  - `consts/*.js` → `WebRTCProtectionScript`, `CanvasProtectionScript`, `WebGLProtectionScript`, `AudioProtectionScript`, `AutomationBypassScript`
  - `navigator/*.js` → `CreateNavigatorScript`
  - `screen/*.js` → `CreateScreenScript`
  - `clienthints/*.js` → `CreateClientHintsScript`
  - `webgl/*.js` → `CreateWebGLScript`
  - `all/*.js` → `GetAllProtectionScripts`
  - `fpscripts/*.js` → `GetFingerprintScripts`
- `TestGoldenFixtureChecksums` passes and enforces that the set of `*.js` fixtures on
  disk exactly matches the pinned set in `fingerprint/testdata/checksums.txt`.
- Added fixture-update discipline comment to `fingerprint/checksums_test.go`.

## Chrome-free Unit Tests (HARD-04)

Confirmed existing and added tests cover:

- **Timezone helpers:** `TestDetectTimezoneFromIP`, `TestResolveEnvTimezone`, `TestSystemTimezoneFromEnv`, `TestSystemTimezoneFromLocaltime`, `TestAutoDetectTimezoneNilProxy`, `TestAutoDetectTimezoneWithProxy`.
- **Session-reuse probe:** `TestTryConnectExisting`, `TestCreateSessionPersistentNotImplemented`, `TestPatchPageScriptSubset`, `TestPatchPageScriptWebGLDead`, `TestPatchPageScriptWebRTCOff`.
- **CLI happy paths:** `TestCLIVersion`, `TestCLIListEmptyJSON`, `TestCLIListNonEmpty`, `TestCLIInfoJSON`, `TestCLIInfoNonJSON`, `TestCLIPath`, `TestCLICreateParsesProxyAndLists`, `TestCLICreateDefaultProxyPort`, `TestCLICreateWithFlags`, `TestCLIDeleteForce`, `TestCLIDeleteByName`.
- **PatchPage subset:** `TestPatchPageInjectionOnExternalPage` plus the Chrome-free `TestPatchPageScript*` tests.

## Hermetic Integration Tests (HARD-05)

- All geo-IP and proxy tests use `httptest` servers or local `net.Listen`; no external
  network calls are made in tests. Added a hermetic comment to the top of `proxy_test.go`.
- Added `TestChromeSkipWhenMissing` to pin the graceful-skip contract when Chrome is absent.
- Added `TestCrossContextInjection` (real Chrome, skip-guarded) to verify that spoofed
  `navigator.hardwareConcurrency` is injected into a newly created iframe.

## Detector-Oracle Baseline (HARD-06)

File: `.planning/data/09-detector-baseline.json`

| Detector | Status | Value / Note |
|---|---|---|
| ThumbmarkJS | passed | `ee576765960905473b035fe7e1389493 / 5e7dca1cdd50a19c5b16414017118520` |
| CreepJS | passed | `unstable` (trust-score element not present on page; fallback recorded) |
| BrowserLeaks | passed | `AEDC0EE5ABD8A25C06629F31DB38A650` (Canvas hash) |

*Recorded with `BROWSER_PROFILES_RUN_NETWORK_ORACLES=1` on 2026-07-06.*

When Chrome or network is unavailable, or when `BROWSER_PROFILES_RUN_NETWORK_ORACLES`
is not set to `1`, each oracle test skips cleanly and records `status: skipped` with the
reason. The harness does not fail the build.

## Verification

- `go build ./...` passes.
- `go vet ./...` passes.
- `go test ./... -count=1` passes.
- `go test . -cover` reports 85.1% coverage.

## Gaps

1. The CreepJS oracle currently records `unstable` because the score DOM selector is not
   reliably extracted on the public CreepJS demo page under headless automation. The
   harness is in place and will record a stable score if the selector/page structure
   becomes extractable.
2. Real-Chrome launch paths are exercised only when Chrome is installed; CI without
   Chrome will see them as skipped. This is by design and documented in the coverage gap
   list above.
3. `.planning/data/09-detector-baseline.json` and this SUMMARY live under the normally
   gitignored `.planning/` directory; they were force-committed as explicit phase
   deliverables.
