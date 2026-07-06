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
  injected byte-for-byte identically to the reference implementation.
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
to the original via golden-string tests.

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
- Persistent (`Temporary: false`) `CreateSession` is not yet implemented; use a stored profile.

## License

MIT.
