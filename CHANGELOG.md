# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.1.0] - 2026-07-06

### Changed

- **v1.0 Go-port cutover finalized (Phase 08).** The previously-uncommitted Go implementation was
  committed and the obsolete TypeScript `src/` tree (plus `package.json`/`tsconfig.json`/
  `tsup.config.ts`) was removed, leaving a clean working tree. See `[0.3.0-go]` below for the
  cutover details.

### Added

- **Test hardening (Phase 09).** Root-package coverage raised from a 77.6% baseline to 85.2%.
  Every anti-detect builder in `fingerprint/scripts.go` is now guarded by a golden fixture pinned
  in `fingerprint/testdata/checksums.txt` (`TestGoldenFixtureChecksums`). Added Chrome-free unit
  tests for timezone helpers, the session-reuse probe, CLI happy paths, and the `PatchPage` subset.
  Geo-IP/proxy integration tests are hermetic (`httptest`/local listeners, no external network
  calls). Established a detector-oracle baseline (`.planning/data/09-detector-baseline.json`):
  the local ThumbmarkJS oracle passes; the network oracles (CreepJS, BrowserLeaks) are skip-safe
  and opt-in via `BROWSER_PROFILES_RUN_NETWORK_ORACLES=1`.
- **Anti-detect mode fields (Phase 10).** `FingerprintConfig.WebRTC` (`"disable"` / `"fake"` /
  `"real"`, default `"fake"`), `Canvas` (`"noise"` / `"real"`, default `"noise"`), and `Audio`
  (`"noise"` / `"real"`, default `"noise"`) now govern the injected script per surface, with the
  v1.0 default output preserved byte-for-byte.
- **Navigator coherence and Client Hints (Phase 11).** `navigator.appVersion`, `productSub`,
  `vendor`, `maxTouchPoints`, `mobile`, and `connection` are now derived from the same generated
  persona as the rest of the fingerprint. User-Agent Client Hints (`navigator.userAgentData` +
  `Sec-CH-UA*` request headers, including `Sec-CH-UA-Full-Version-List`) are injected in the core
  launch path (not only a consumer helper) and applied to every tab. WebGL numeric capabilities
  (`MAX_TEXTURE_SIZE`, `MAX_VIEWPORT_DIMS`, etc.) are now stable per profile, chosen from a
  GPU-family table instead of being randomized per call.
- **Permissions, plugins, and fonts guards (Phase 12).** `navigator.permissions.query` returns
  coherent, platform-consistent states for `camera`, `microphone`, `geolocation`, and
  `notifications`. `navigator.plugins`/`navigator.mimeTypes` expose a platform-specific
  (Windows/macOS/Linux) array-like surface with correct `item`/`namedItem`/`refresh` shapes. A
  lightweight per-OS `document.fonts.check` whitelist guard was added (see Deferred/caveats: this
  is not full OS-level font spoofing).
- **WebGPU and optional timing controls (Phase 13).** `navigator.gpu.requestAdapter()` returns a
  mocked `GPUAdapter` whose `info` is consistent with the profile's spoofed GPU family. Optional,
  gated timing spoofing rounds `performance.now()`/`Date.now()` to a configured precision
  (monotonic, drift-free; off by default). Optional CDP CPU throttling via
  `FingerprintConfig.CPUThrottlingRate` (off by default, applied once at launch via
  `Emulation.setCPUThrottlingRate`).

### Notes

- Self-contained detector-oracle coherence checks (`navigator_coherence`,
  `permissions_plugins_fonts_coherence`, `webgpu_timing_coherence`) all pass in the current
  baseline, and the ThumbmarkJS oracle records distinct fingerprint hashes across profiles. The
  network-based oracles (CreepJS, BrowserLeaks) stay opt-in and are not part of the default test
  run — see `.planning/data/09-detector-baseline.json`.
- See `README.md` (feature list, "Mode flags", and "Notes") and `docs/ARCHITECTURE.md` (the "v1.1
  Anti-Detect Surfaces" table) for usage and caveats of every surface above.

### Deferred

- CI pipeline (GitHub Actions running `go build`/`vet`/`test`/`-race` on push and PR).
- Release tooling (goreleaser, version tags, cross-compiled binaries).
- TLS / JA3 / JA4 fingerprint spoofing.
- Behavioral / interaction biometrics (mouse, keyboard, touch realism).
- DNS leak prevention.
- Persistent `CreateSession(Temporary: false)`.
- ExTower HTTP client.

## [0.3.0-go] - 2026-07-06

### Changed

- **Ported to Go, driven by go-rod.** Full reimplementation of the library and CLI in Go
  (`github.com/postfix/browser-profiles`) on [go-rod](https://github.com/go-rod/rod), replacing the
  TypeScript / Puppeteer / Playwright implementation. The on-disk profile format is preserved
  (`~/.aitofy/browser-profiles`), and the injected anti-detect scripts are byte-identical to the
  reference (golden-string verified).

### Added

- Authenticated **SOCKS5** proxy support (the TypeScript launcher rejected SOCKS5) via a local
  credential-injecting forward proxy.
- Browser-level protection injection so anti-detect scripts reach every tab, not only the launch target.

### Removed

- TypeScript sources, the npm package, and the separate Puppeteer / Playwright / ExTower
  integrations — the Puppeteer and Playwright integrations collapse into the single built-in
  go-rod driver.

## [0.2.12] - 2026-01-14

### Added

- **Custom Profile IDs** 🆔
  - Create profiles with your own custom IDs instead of auto-generated hex strings
  - IDs must be 1-64 characters, alphanumeric with hyphens/underscores only
  - Validation prevents invalid IDs and duplicates
  
  ```typescript
  const profile = await profiles.create({
    id: 'google-main',      // Custom ID!
    name: 'Google Account',
  });
  
  // Launch by custom ID
  await profiles.launch('google-main');
  ```

- **Launch by Profile Name** 📛
  - New methods to find and launch profiles by name (case-insensitive)
  - `getByName(name)` - Find profile by name
  - `getByIdOrName(idOrName)` - Find by ID first, then by name
  - `launchByName(name, options?)` - Launch browser by profile name
  - `launchByIdOrName(idOrName, options?)` - Launch by ID or name
  
  ```typescript
  // Create profile
  await profiles.create({ name: 'Facebook Account' });
  
  // Launch by name
  await profiles.launchByName('Facebook Account');
  
  // Or use flexible method
  await profiles.launchByIdOrName('Facebook Account'); // By name
  await profiles.launchByIdOrName('google-main');      // By ID
  ```

- **CLI Improvements**
  - `browser-profiles create <name> --id <custom-id>` - Create profile with custom ID
  - `browser-profiles open <id-or-name>` - Open browser by ID or name
  - `browser-profiles info <id-or-name>` - Show profile info by ID or name
  - `browser-profiles delete <id-or-name>` - Delete profile by ID or name

### Changed

- `withPuppeteer()` now uses `getByIdOrName()` for cleaner profile lookup with case-insensitive name matching

## [0.2.10] - 2026-01-12

### Changed

- **Simplified Puppeteer connect logic** - Removed redundant retry at puppeteer level (chrome-launcher already handles retries for CDP and wsEndpoint)
- Cleaner, more maintainable code

## [0.2.9] - 2026-01-12

### Fixed

- **Chrome stale lock file cleanup** 🔓
  - Auto-cleans `SingletonLock`, `SingletonCookie`, `SingletonSocket` files before launching
  - These files are left behind when Chrome crashes and prevent new instances from starting
  - Fixes "Failed to create ProcessSingleton" errors
  - Added CDP connection retry with delay (300ms, 10 retries)
  - Added wsEndpoint fetch retry with delay (200ms, 10 retries)
  - Better logging during launch sequence for debugging

  ```
  [browser-profiles] 🧹 Cleaned up stale SingletonLock
  [browser-profiles] ✅ Chrome process started, port: 54000, pid: 12345
  ```

## [0.2.8] - 2026-01-12

### Fixed

- **Connection retry on stale browser** ⚡
  - If puppeteer.connect() fails with ECONNREFUSED, automatically retries with a fresh browser launch
  - Handles race condition where browser crashes between detection and connection
  - Improved `tryConnectExisting()` to not rely on PID check alone (OS can reuse PIDs)
  - Better error handling and logging for connection failures

  ```
  [browser-profiles] ⚠️ Connection failed (ECONNREFUSED), retrying with fresh browser...
  Chrome launched on port 54000, PID: 12345
  ```

## [0.2.7] - 2026-01-12

### Added

- **Session isolation** - Each session now creates its own page
  - When reconnecting to existing browser, creates NEW page instead of reusing existing pages
  - Sessions no longer interfere with each other's pages
  
- **`close()` and `terminate()` separation** 🔄
  - `close()` - Only closes THIS session's page (browser stays running for other sessions)
  - `terminate()` - Kills the browser process entirely (same as old behavior)
  - `close({ terminate: true })` - Alternative way to terminate

  ```typescript
  // Session 1
  const session1 = await withPuppeteer({ profile: 'my-profile' });
  
  // Session 2 (same browser, different page)
  const session2 = await withPuppeteer({ profile: 'my-profile' });
  
  // Close session2's page only (browser stays running)
  await session2.close();
  
  // Session1 still works!
  await session1.page.goto('https://example.com');
  
  // Kill browser entirely
  await session1.terminate();
  ```

### Changed

- Default `close()` behavior: Now only closes the session's page (previously killed browser)
- To kill browser, use `terminate()` or `close({ terminate: true })`

## [0.2.6] - 2026-01-12

### Added

- **Cross-process browser session detection** 🔄
  - Automatically detects if a browser is already running for a profile
  - If browser is already running: returns existing connection (no error!)
  - If browser is not running: launches new browser as usual
  - Uses lock files (`~/.aitofy/browser-profiles/<profile-id>/.browser-lock.json`) to track sessions
  - Works across different Node.js processes and terminals

  ```typescript
  // Terminal 1
  const { page } = await withPuppeteer({ profile: 'my-profile' });
  // Browser launched...
  
  // Terminal 2 (same profile, no error!)
  const { page } = await withPuppeteer({ profile: 'my-profile' });
  // [browser-profiles] ♻️ Found existing browser for profile "my-profile"
  // Connects to existing browser instead of failing!
  ```

### Fixed

- Running multiple scripts with the same profile ID no longer causes "port already in use" errors
- Stale lock files are automatically cleaned up when browser process has died

## [0.2.5] - 2026-01-12

### Added

- **Native Type Re-exports** - Full Puppeteer/Playwright API access! 🎉
  - `PuppeteerPage`, `PuppeteerBrowser`, `HTTPRequest`, `HTTPResponse`, `Cookie` re-exported from `puppeteer-core`
  - `PlaywrightPage`, `PlaywrightBrowser`, `PlaywrightContext`, `PlaywrightRequest`, `PlaywrightResponse`, `Route` re-exported from `playwright`
  - No more TypeScript errors when using `setRequestInterception()`, `on('request')`, `cookies()`, `route()`, etc.
  
  ```typescript
  // Before v0.2.5: Type errors!
  const { page } = await withPuppeteer({ profile: 'my-profile' });
  await page.setRequestInterception(true);  // ❌ Property does not exist
  
  // v0.2.5+: Full API access!
  const { page } = await withPuppeteer({ profile: 'my-profile' });
  await page.setRequestInterception(true);  // ✅ Works!
  page.on('request', (req) => { ... });     // ✅ Works!
  const cookies = await page.cookies();     // ✅ Works!
  ```

### Changed

- `playwright` added to devDependencies for type declarations

---

## [0.2.4] - 2026-01-12

### Fixed

- **Multiple pages issue** - Fixed browser opening with 2 pages instead of 1
  - `withPuppeteer()`, `quickLaunch()`, and `createSession()` now reuse existing browser pages
  - Prevents duplicate empty pages from appearing on browser launch
  - Only creates a new page if no pages exist yet
  - Better resource management and cleaner user experience

## 📝 Known Issues (Historical)

<details>
<summary>Simplified Page Types (v0.2.0 - v0.2.4) - ✅ RESOLVED in v0.2.5</summary>

**Issue:** Current `PuppeteerPage` and `PlaywrightPage` type definitions are simplified interfaces, missing commonly used APIs. This causes TypeScript errors when users try to use full Puppeteer/Playwright APIs.

**Missing APIs:**

| `PuppeteerPage` | `PlaywrightPage` |
|-----------------|------------------|
| `setRequestInterception()` | `on('request', callback)` |
| `on('request', callback)` | `waitForTimeout()` |
| `on('response', callback)` | `reload()` |
| `cookies(...urls)` | `route()` for request interception |
| `setCookie(...cookies)` | `screenshot()` |
| `title()` | `title()` |
| `content()` | `content()` |
| `waitForSelector()` | `waitForSelector()` |

**Resolution:** Fixed in v0.2.5 by re-exporting native types from `puppeteer-core` and `playwright`.

</details>

---

## [0.2.3] - 2026-01-09

### Changed

- Added CLI documentation to README.md (front and center)
- Updated llms.txt with CLI commands

## [0.2.2] - 2026-01-09

### Added

- **CLI Tool** - Command line interface for managing browser profiles
  ```bash
  browser-profiles list              # List all profiles
  browser-profiles create <name>     # Create new profile
  browser-profiles delete <id>       # Delete profile
  browser-profiles open <id>         # Open browser with profile
  browser-profiles launch            # Quick launch with random fingerprint
  browser-profiles info <id>         # Show profile details
  browser-profiles path              # Show storage path
  ```

- Added `commander` dependency for CLI

## [0.2.1] - 2026-01-09

### Changed

- **Simplified imports** - All Puppeteer functions now available from main entry point
  ```typescript
  // Before (still works)
  import { quickLaunch } from '@aitofy/browser-profiles/puppeteer';
  
  // Now also available (recommended)
  import { quickLaunch } from '@aitofy/browser-profiles';
  ```

- Re-exported from main entry:
  - `withPuppeteer`, `quickLaunch`, `connectPuppeteer`
  - `patchPage`, `createSession`
  - All related TypeScript types

## [0.2.0] - 2026-01-09

### Added

#### New Functions
- **`createSession()`** - Create lightweight temporary browser sessions with random fingerprints
  ```typescript
  const session = await createSession({
    temporary: true,
    randomFingerprint: true,
    proxy: { type: 'http', host: 'proxy.com', port: 8080 },
  });
  ```

- **`patchPage()`** - Apply anti-detect patches to any existing Puppeteer page
  ```typescript
  await patchPage(page, {
    webdriver: true,
    plugins: true,
    webrtc: true,
    fingerprint: { platform: 'Win32' },
  });
  ```

- **`generateFingerprint()`** - Generate complete browser fingerprints on-demand
  ```typescript
  const fp = generateFingerprint({
    platform: 'macos',
    gpu: 'apple',
    language: 'ja-JP',
  });
  ```

- **`getFingerprintScripts()`** - Get all injection scripts for a fingerprint
  ```typescript
  const scripts = getFingerprintScripts(fp);
  await page.evaluateOnNewDocument(scripts);
  ```

- **`launchChromeStandalone()`** - Launch Chrome without profile management
  ```typescript
  const { wsEndpoint, close } = await launchChromeStandalone({
    headless: false,
    proxy: { type: 'http', host: 'proxy.com', port: 8080 },
  });
  ```

#### New Options
- Added `puppeteer` option to `withPuppeteer()`, `quickLaunch()`, and `connectPuppeteer()` to inject your own puppeteer instance
  ```typescript
  import puppeteer from 'rebrowser-puppeteer-core';
  const { browser, page } = await withPuppeteer({
    profile: 'my-profile',
    puppeteer, // Use your own instance
  });
  ```

### Fixed

- **ESM Compatibility** - Package now works correctly in ESM environments (tsx, vite, next.js)
  - Replaced `require()` with dynamic `import()` in `getPuppeteer()`
  - Properly handles both ESM default exports and CJS module.exports
  - Cleaner error messages with proper newlines

### Changed

- Better code organization in `getPuppeteer()` with loop-based package detection
- Cleaner logging with package labels
- Improved TypeScript types with better documentation

### Types

- Added `GenerateFingerprintOptions` and `GeneratedFingerprint` types
- Added `StandaloneLaunchOptions` and `StandaloneLaunchResult` types
- Added `PatchPageOptions`, `CreateSessionOptions`, and `SessionResult` types

## [0.1.1] - 2026-01-07

### Fixed
- Minor bug fixes and stability improvements

## [0.1.0] - 2026-01-06

### Added
- Initial release
- `BrowserProfiles` class for profile management
- `withPuppeteer()` and `quickLaunch()` for Puppeteer integration
- `withPlaywright()` for Playwright integration
- Anti-detect features: WebRTC, Canvas, WebGL, Audio fingerprint protection
- Proxy support with auto timezone detection
- ExTower integration

[0.2.0]: https://github.com/aitofy-dev/browser-profiles/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/aitofy-dev/browser-profiles/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/aitofy-dev/browser-profiles/releases/tag/v0.1.0
