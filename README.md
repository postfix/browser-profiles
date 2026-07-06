# browser-profiles (Go)

> 🔒 **Self-hosted anti-detect browser profiles for Go.** The open-source AdsPower / Multilogin
> alternative — run locally, own your data, no subscriptions. Driven by [go-rod](https://github.com/go-rod/rod).

[![Go Reference](https://img.shields.io/badge/go-reference-blue.svg)](https://pkg.go.dev/github.com/postfix/browser-profiles)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A faithful Go port of `@aitofy/browser-profiles`, reimplemented on **go-rod** in place of
Puppeteer/Playwright. Same features, same on-disk profile format, same anti-detect behavior.

## ✨ Features

- 🏠 **Self-hosted** — profiles live on your machine (`~/.aitofy/browser-profiles`), no cloud.
- 🛡️ **Anti-detect** — WebRTC, Canvas, WebGL, Audio, and automation-bypass fingerprint protection,
  injected byte-for-byte identically to the reference implementation. WebRTC/Canvas/Audio each
  support a **mode flag** (`disable`/`fake`/`real` for WebRTC, `noise`/`real` for Canvas and Audio)
  — see [🎛️ Mode flags](#-mode-flags).
- 🧭 **Navigator coherence** — `appVersion`, `productSub`, `vendor`, `maxTouchPoints`, `mobile`, and
  `connection` are derived from the same generated persona, so `navigator.*` never contradicts itself.
- 🪪 **Client Hints in the core launch path** — `navigator.userAgentData` + `Sec-CH-UA*` request
  headers are injected on launch (not only via a consumer helper) and stay coherent across every tab.
- 🎮 **Stable per-profile WebGL caps** — numeric WebGL parameters (`MAX_TEXTURE_SIZE`,
  `MAX_VIEWPORT_DIMS`, etc.) are pinned per profile from a GPU-family table instead of being
  randomized on every call.
- 🔐 **Permissions guard** — `navigator.permissions.query` returns coherent states for `camera`,
  `microphone`, `geolocation`, and `notifications`.
- 🧩 **Platform-specific plugins/mimeTypes** — `navigator.plugins` and `navigator.mimeTypes` expose
  array-like objects matching the profile's OS (Windows/macOS/Linux).
- 🔤 **Fonts guard** — a lightweight `document.fonts.check` whitelist per OS (see Notes below for
  its scope).
- 🖥️ **WebGPU spoofing** — `navigator.gpu.requestAdapter().info` matches the profile's spoofed GPU
  family.
- ⏱️ **Optional timing/CPU-throttling controls** — gated `performance.now()`/`Date.now()` precision
  rounding and CDP CPU throttling, both off by default (see Notes below).
- 🌐 **Proxy support** — HTTP, HTTPS, and SOCKS5, **including authenticated proxies** via a local
  credential-injecting forward proxy (Chrome's `--proxy-server` can't carry credentials).
- 🕒 **Auto timezone** — detected from the proxy IP (ip-api.com) and applied via CDP.
- 📦 **Profile management** — create, update, delete, group, duplicate, export/import.
- 🎭 **go-rod native** — returns a live `*rod.Browser` + `*rod.Page`; protections reach every tab.
- ⚡ **CLI included** — `browser-profiles` for quick profile management and launches.

## 📦 Installation

```bash
go get github.com/postfix/browser-profiles
```

Requires Go 1.26+ and a local Chrome/Chromium (auto-detected, or set `CHROME_PATH`).

Install the CLI:

```bash
go install github.com/postfix/browser-profiles/cmd/browser-profiles@latest
```

### Anti-Detect Status

Parity with the reference implementation (95% is the practical ceiling for CDP-driven Chrome;
100% requires a modified Chromium). The injected protection scripts are verified byte-identical
to the original via golden-string tests (`fingerprint/testdata/checksums.txt`).

v1.1 adds self-contained coherence checks — navigator, permissions/plugins/fonts, and
WebGPU/WebGL alignment — plus a ThumbmarkJS run confirming distinct per-profile fingerprints;
all currently pass (`.planning/data/09-detector-baseline.json`). The network-based oracles
(CreepJS, BrowserLeaks) are opt-in via `BROWSER_PROFILES_RUN_NETWORK_ORACLES=1` and are skipped
by default.

## 🚀 Quick Start

### Quick launch (temporary session, random fingerprint)

```go
package main

import (
	"log"

	bp "github.com/postfix/browser-profiles"
)

func main() {
	sess, err := bp.CreateSession(bp.CreateSessionOptions{
		Proxy: &bp.ProxyConfig{Type: "http", Host: "proxy.example.com", Port: 8080},
		// RandomFingerprint defaults to true; timezone auto-detected from the proxy IP.
	})
	if err != nil {
		log.Fatal(err)
	}
	defer sess.Terminate()

	if err := sess.Page.Navigate("https://browserscan.net"); err != nil {
		log.Fatal(err)
	}
	sess.Page.MustWaitLoad()
}
```

### Profile management

```go
profiles := bp.NewBrowserProfiles(bp.BrowserProfilesOptions{})

// Create a persistent profile (stored under ~/.aitofy/browser-profiles/).
p, err := profiles.Create(bp.ProfileConfig{
	Name:  "My Account",
	Proxy: &bp.ProxyConfig{Type: "http", Host: "proxy.example.com", Port: 8080, Username: "u", Password: "p"},
})

// Launch it and drive it with go-rod.
sess, err := bp.WithProfile(profiles, p.ID, bp.LaunchOptions{})
defer sess.Close()
sess.Page.MustNavigate("https://whoer.net")
```

`Create` accepts a custom `ID` (1–64 chars, `[a-zA-Z0-9_-]`); launch by ID or name via
`WithProfile(profiles, "My Account", …)` (case-insensitive).

### Standalone launch (no profile, raw endpoint)

```go
res, err := bp.LaunchChromeStandalone(bp.StandaloneLaunchOptions{
	Headless: true,
	Proxy:    &bp.ProxyConfig{Type: "socks5", Host: "proxy.example.com", Port: 1080, Username: "u", Password: "p"},
})
defer res.Close()
// res.WsEndpoint / res.PID / res.Port — connect any CDP client, e.g. rod.New().ControlURL(res.WsEndpoint)
```

### Patch an existing go-rod page

```go
if err := bp.PatchPage(page, bp.PatchPageOptions{}); err != nil { /* ... */ }
// Applies navigator + WebRTC + automation-bypass protections to a page you already control.
```

### Generate a fingerprint on demand

```go
import "github.com/postfix/browser-profiles/fingerprint"

fp := fingerprint.GenerateFingerprint(fingerprint.GenerateFingerprintOptions{
	Platform: "macos", Gpu: "apple", Screen: "retina", Language: "ja-JP",
})
scripts := fingerprint.GetFingerprintScripts(fp) // inject via page.EvalOnNewDocument(scripts)
```

### 🎛️ Mode flags

`FingerprintConfig.WebRTC`, `Canvas`, and `Audio` each take a mode string. An empty or
unrecognized value falls back to the v1.0 default (**bold** below):

| Field | Valid values | Default behavior |
|---|---|---|
| `WebRTC` | `"disable"`, **`"fake"`**, `"real"` | `"fake"` spoofs local/public candidates; `"disable"` removes `RTCPeerConnection`; `"real"` leaves WebRTC untouched. |
| `Canvas` | **`"noise"`**, `"real"` | `"noise"` adds per-pixel noise to `toDataURL`/`getImageData`; `"real"` leaves the canvas APIs native. |
| `Audio` | **`"noise"`**, `"real"` | `"noise"` perturbs `AudioBuffer.getChannelData`; `"real"` leaves the Web Audio APIs native. |

```go
sess, err := bp.CreateSession(bp.CreateSessionOptions{
	Fingerprint: &bp.FingerprintConfig{
		WebRTC: "disable", // window.RTCPeerConnection === undefined
		Canvas: "real",    // skip canvas noise for this profile
		Audio:  "noise",   // keep the default audio noise
	},
})
```

The same fields work on `ProfileConfig.Fingerprint` (persistent profiles), and on generated
fingerprints via `GenerateFingerprintOptions.Overrides` (which mutates
`GeneratedFingerprint.WebRTC`/`Canvas`/`Audio` before injection).

## 💻 CLI

```bash
browser-profiles list                       # list profiles (add --json)
browser-profiles create my-account          # create a profile
browser-profiles create acct --proxy http://user:pass@proxy.com:8080 --timezone America/New_York
browser-profiles open <id-or-name>          # launch a profile (Ctrl-C to close)
browser-profiles launch --proxy http://proxy.com:8080   # temporary random-fingerprint session
browser-profiles info <id-or-name>          # show profile details
browser-profiles delete <id-or-name>        # delete a profile
browser-profiles path                       # print the storage directory
browser-profiles --version
```

## API overview

- `NewBrowserProfiles(BrowserProfilesOptions)` → `*BrowserProfiles`: `Create`, `Get`, `GetByName`,
  `GetByIdOrName`, `List`, `Update`, `Delete`, `CreateGroup`/`ListGroups`/`DeleteGroup`/`MoveToGroup`,
  `Duplicate`, `Export`/`Import`, `Launch`/`LaunchByName`/`LaunchByIdOrName`, `Close`/`CloseAll`/`GetRunning`.
- `WithProfile(bp, idOrName, LaunchOptions)` / `QuickLaunch(QuickLaunchOptions)` /
  `CreateSession(CreateSessionOptions)` → `*Session{Browser, Page, Close, Terminate}`.
- `LaunchChrome` / `LaunchChromeStandalone`, `GetChromePath`, `PatchPage`.
- Package `fingerprint`: `GenerateFingerprint`, `GetAllProtectionScripts`, `GetFingerprintScripts`,
  and the protection-script constants.

## Notes

- **Profile storage** is byte-format-compatible with the reference `@aitofy/browser-profiles`:
  `~/.aitofy/browser-profiles/profiles/<id>/{config.json,data/}` and `groups/<id>.json`.
- **Authenticated SOCKS5** is supported (an enhancement over the reference, which rejects it).
- **The `document.fonts` guard is a lightweight check, not a full font spoof.** It only makes
  `document.fonts.check(family)` return `true` for a per-OS whitelist (`FingerprintConfig.Fonts.Whitelist`)
  and falls back to the real check otherwise. True font spoofing requires installing/removing fonts
  at the OS level, which is outside the scope of a CDP-injection library.
- **Timing spoofing and CDP CPU throttling are off by default.** Enable them explicitly:
  ```go
  cfg := &bp.FingerprintConfig{
  	Timing:            &bp.TimingConfig{Enabled: true, Precision: time.Millisecond},
  	CPUThrottlingRate: 2, // 2x slowdown via Emulation.setCPUThrottlingRate; 0 = disabled
  }
  ```
  Timing rounds `performance.now()`/`Date.now()` to `Precision` (monotonic, drift-free); CPU
  throttling is a per-launch CDP setting only, so it is not re-applied to tabs opened later.
- Persistent (`Temporary: false`) `CreateSession` is not yet implemented; use a stored profile.

## License

MIT.
