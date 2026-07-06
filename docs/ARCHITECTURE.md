# Architecture

`browser-profiles` is a self-hosted anti-detect browser-profile manager for Go, driven by
[go-rod](https://github.com/go-rod/rod). It is a faithful port of the TypeScript
`@aitofy/browser-profiles` v0.2.12.

## Package layout

```
browserprofiles/            root package — the cohesive core (flat by design; see below)
  types.go                  all config/result structs + BrowserError; flexible Port (number|string)
  profiles.go               BrowserProfiles: filesystem CRUD, groups, duplicate/export/import
  launch_orchestration.go   BrowserProfiles.Launch/LaunchByName/LaunchByIdOrName/Close/CloseAll/GetRunning
  chrome.go                 go-rod launcher, CDP anti-detect sequence, session reuse, standalone launch
  geoip.go                  timezone detection from proxy IP (ip-api.com)
  proxy.go                  authenticated forward proxy (HTTP/HTTPS/SOCKS5)
  session.go                WithProfile / QuickLaunch / CreateSession / PatchPage + M5 injection loop
  fingerprint/              leaf subpackage (root imports it; no cycle)
    scripts.go + scripts/*.js   verbatim protection scripts (embedded) + builders + assemblers
    generate.go             fingerprint generator + data tables
  cmd/browser-profiles/     cobra CLI
```

**Why a flat root package (not `launcher/` + `integrations/` subpackages):** `BrowserProfiles.Launch`
must call the launcher, and the launcher needs the shared types — a subpackage split creates a Go
import cycle (and methods can't be defined on a type from another package). The idiomatic resolution
is a single cohesive core package with `fingerprint` as the one genuinely-separable leaf. This also
gives the "import everything from one entry point" ergonomics the reference advertised.

## Anti-detect mechanism (the core value)

Detection resistance has two independent layers:

1. **Launch flags (browser-wide).** `buildLauncher` (`chrome.go`) constructs the Chrome flag set
   **exactly** — it rebuilds go-rod's `launcher.Flags` map from scratch so go-rod's defaults never
   leak. Critically it **omits `--enable-automation`** (which would natively set
   `navigator.webdriver=true` and show the automation infobar) and includes
   `--disable-blink-features=AutomationControlled`, `--disable-infobars`, the WebRTC-leak flags, etc.
   This is asserted browser-free via `launcher.FormatArgs()` and proven at runtime
   (`navigator.webdriver === false` on a fresh page, *without* relying on the JS mask).

2. **Injected JavaScript (per page).** The protection scripts (WebRTC, Canvas, WebGL, Audio,
   automation-bypass, navigator/screen/client-hints spoofing) live in `fingerprint/scripts/*.js`,
   embedded **byte-for-byte** from the reference implementation. `applyAntiDetect` (`chrome.go`)
   injects the bundle on the launch target via CDP `Page.addScriptToEvaluateOnNewDocument`
   (go-rod `page.EvalOnNewDocument`), after `Network.setUserAgentOverride` (+ client-hint metadata)
   and before `Emulation.setTimezoneOverride` and per-cookie `Network.setCookie`.

   Because `EvalOnNewDocument` is **per-target**, the `session.go` convenience layer adds the **M5**
   browser-level loop: `installProtections` enables CDP target discovery and runs a cancelable
   `browser.EachEvent(*proto.TargetTargetCreated)` → `PageFromTarget` → `EvalOnNewDocument` goroutine
   so every newly-opened tab is protected too.

Fidelity is guarded by **golden-string tests** (`fingerprint/scripts_test.go`) that byte-compare the
Go output to fixtures captured from the reference (`fingerprint/testdata/`), including an HTML-escape
case (Go's `json.Marshal` HTML-escapes by default; the builders use `SetEscapeHTML(false)` +
ordered structs to match JS `JSON.stringify`).

## Proxy mechanism

Chrome's `--proxy-server` cannot carry credentials. For authenticated proxies, `resolveProxy`
(`proxy.go`) starts a **local forward proxy** bound to `127.0.0.1` that Chrome connects to without
credentials and which forwards upstream **with** them:

- HTTP/HTTPS upstream: CONNECT tunneling + plain-HTTP forwarding, injecting `Proxy-Authorization`.
- SOCKS5 upstream: dials targets through `golang.org/x/net/proxy` with `proxy.Auth` (an enhancement —
  the reference rejected SOCKS5).

Credential-free proxies use `--proxy-server` directly. Only the local forward-proxy URL (not upstream
credentials) is written to the session lock file.

## Profile store

Profiles are filesystem-backed under `~/.aitofy/browser-profiles/profiles/<id>/{config.json,data/}`
with `groups/<id>.json`, **format-compatible** with the reference (reads its `config.json` unchanged;
writes semantically-equivalent 2-space JSON). `data/` is the Chrome `--user-data-dir`. Cross-process
session reuse is tracked via `.browser-lock.json` (a `/json/version` probe decides reconnect).

## Fingerprint engine

`fingerprint.GenerateFingerprint` produces a realistic, consistent fingerprint from per-OS/GPU/screen
tables (`math/rand/v2`). `GetAllProtectionScripts` (launch path) and `GetFingerprintScripts` (consumer
helper) assemble the injected bundle.

## Testing

`go test ./...`:
- `fingerprint/` — golden byte-identity of scripts + generator range/table tests (browser-free).
- root — profile-store CRUD (`t.TempDir`), launcher B1 flag assertion (browser-free), a real
  headless-Chrome launch smoke test (skips if no Chrome), and the forward-proxy integration tests
  (fake auth-checked HTTP upstream + a minimal fake SOCKS5 server).
