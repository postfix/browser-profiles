package browserprofiles

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"

	"github.com/postfix/browser-profiles/fingerprint"
)

// ---- running-browser tracking (mirrors chrome-launcher's module-level map) ----

var (
	runningMu       sync.Mutex
	runningBrowsers = map[string]*LaunchResult{}

	profileLocksMu sync.Mutex
	profileLocks   = map[string]*sync.Mutex{}
)

// lockProfile serializes launch/reuse for a single profile ID so concurrent Launch()
// calls cannot race past session reuse and spawn duplicate Chrome processes on one
// --user-data-dir. Returns the unlock func.
func lockProfile(id string) func() {
	profileLocksMu.Lock()
	m, ok := profileLocks[id]
	if !ok {
		m = &sync.Mutex{}
		profileLocks[id] = m
	}
	profileLocksMu.Unlock()
	m.Lock()
	return m.Unlock
}

func trackBrowser(id string, lr *LaunchResult) {
	runningMu.Lock()
	runningBrowsers[id] = lr
	runningMu.Unlock()
}

func getTracked(id string) *LaunchResult {
	runningMu.Lock()
	defer runningMu.Unlock()
	return runningBrowsers[id]
}

func untrackBrowser(id string) {
	runningMu.Lock()
	delete(runningBrowsers, id)
	runningMu.Unlock()
}

// CloseBrowser closes a running browser by profile ID. Returns false if none tracked.
func CloseBrowser(id string) bool {
	runningMu.Lock()
	lr, ok := runningBrowsers[id]
	runningMu.Unlock()
	if !ok || lr == nil || lr.Close == nil {
		return false
	}
	_ = lr.Close() // Close untracks
	return true
}

// CloseAllBrowsers closes every tracked running browser.
func CloseAllBrowsers() {
	runningMu.Lock()
	ids := make([]string, 0, len(runningBrowsers))
	for id := range runningBrowsers {
		ids = append(ids, id)
	}
	runningMu.Unlock()
	for _, id := range ids {
		CloseBrowser(id)
	}
}

// GetRunningBrowsers returns the profile IDs of tracked running browsers.
func GetRunningBrowsers() []string {
	runningMu.Lock()
	defer runningMu.Unlock()
	ids := make([]string, 0, len(runningBrowsers))
	for id := range runningBrowsers {
		ids = append(ids, id)
	}
	return ids
}

// ---- lock file (cross-process session reuse) ----

type browserLockInfo struct {
	PID        int    `json:"pid"`
	Port       int    `json:"port"`
	WsEndpoint string `json:"wsEndpoint"`
	StartedAt  int64  `json:"startedAt"`
	ProxyURL   string `json:"proxyUrl,omitempty"`
}

func lockFilePath(userDataDir string) string {
	return filepath.Join(userDataDir, ".browser-lock.json")
}

func writeLockFile(userDataDir string, info browserLockInfo) {
	if b, err := json.MarshalIndent(info, "", "  "); err == nil {
		_ = os.WriteFile(lockFilePath(userDataDir), b, 0o600)
	}
}

func readLockFile(userDataDir string) *browserLockInfo {
	b, err := os.ReadFile(lockFilePath(userDataDir))
	if err != nil {
		return nil
	}
	var info browserLockInfo
	if json.Unmarshal(b, &info) != nil {
		return nil
	}
	return &info
}

func deleteLockFile(userDataDir string) { _ = os.Remove(lockFilePath(userDataDir)) }

// isProcessRunning reports whether pid is alive (signal 0 probe, POSIX semantics).
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

func terminateProcess(pid int) {
	if p, err := os.FindProcess(pid); err == nil {
		_ = p.Signal(syscall.SIGTERM)
	}
}

// tryConnectExisting probes /json/version to confirm a locked browser is responsive.
func tryConnectExisting(lock *browserLockInfo) *browserLockInfo {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/json/version", lock.Port))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var v struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if json.NewDecoder(resp.Body).Decode(&v) != nil || v.WebSocketDebuggerURL == "" {
		return nil
	}
	return &browserLockInfo{PID: lock.PID, Port: lock.Port, WsEndpoint: v.WebSocketDebuggerURL, ProxyURL: lock.ProxyURL}
}

// ---- chrome path & proxy url ----

// GetChromePath resolves the Chrome/Chromium executable: custom → env → platform defaults.
func GetChromePath(customPath string) (string, error) {
	if customPath != "" && fileExists(customPath) {
		return customPath, nil
	}
	if env := os.Getenv("CHROMIUM_PATH"); env != "" && fileExists(env) {
		return env, nil
	}
	if env := os.Getenv("CHROME_PATH"); env != "" && fileExists(env) {
		return env, nil
	}
	home, _ := os.UserHomeDir()
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			filepath.Join(home, "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
		}
	case "windows":
		candidates = []string{
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			filepath.Join(home, `AppData\Local\Google\Chrome\Application\chrome.exe`),
		}
	default: // linux
		candidates = []string{
			"/usr/bin/google-chrome",
			"/usr/bin/google-chrome-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
		}
	}
	for _, p := range candidates {
		if fileExists(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("Chrome/Chromium not found. Please install Chrome or set CHROME_PATH environment variable.")
}

// buildProxyURL builds a proxy URL from config (mirrors buildProxyUrl).
func buildProxyURL(p *ProxyConfig) string {
	protocol := "http"
	if p.Type == "socks5" {
		protocol = "socks5"
	}
	if p.Username != "" && p.Password != "" {
		return fmt.Sprintf("%s://%s:%s@%s:%s", protocol, url.QueryEscape(p.Username), url.QueryEscape(p.Password), p.Host, p.Port.String())
	}
	return fmt.Sprintf("%s://%s:%s", protocol, p.Host, p.Port.String())
}

// ---- launcher construction (the exact TS flag set; B1: no --enable-automation) ----

// buildLauncher constructs a go-rod launcher whose flag set exactly mirrors the TS
// chrome-launcher stealth flags. It REBUILDS launcher.Flags from scratch so none of
// go-rod's defaults (critically --enable-automation, which sets navigator.webdriver=true
// and shows the automation infobar) leak through. --proxy-server is set to proxyServerURL
// (already resolved by resolveProxy) when non-empty.
func buildLauncher(profile *StoredProfile, userDataDir string, opts LaunchOptions, proxyServerURL string) (*launcher.Launcher, error) {
	exe, err := GetChromePath(opts.ChromePath)
	if err != nil {
		return nil, err
	}

	l := launcher.New()
	// Rebuild the flag map: only go-rod operational pseudo-flags + the exact TS set.
	l.Flags = map[flags.Flag][]string{
		flags.Bin:                 {exe},
		flags.Leakless:            nil,
		flags.RemoteDebuggingPort: {"0"},
		flags.UserDataDir:         {userDataDir},
	}

	lang := "en-US"
	if profile.Fingerprint != nil && profile.Fingerprint.Language != "" {
		lang = profile.Fingerprint.Language
	}
	for _, f := range []flags.Flag{
		"no-first-run", "no-default-browser-check", "disable-background-networking",
		"disable-client-side-phishing-detection", "disable-default-apps", "disable-hang-monitor",
		"disable-popup-blocking", "disable-prompt-on-repost", "disable-sync", "disable-translate",
		"metrics-recording-only", "no-sandbox", "disable-setuid-sandbox", "disable-dev-shm-usage",
		"disable-infobars", "disable-extensions-file-access-check", "force-webrtc-ip-handling-policy",
	} {
		l.Set(f)
	}
	l.Set("lang", lang)
	l.Set("disable-blink-features", "AutomationControlled")
	l.Set("enable-features", "NetworkService", "NetworkServiceInProcess")
	l.Set("disable-features", "IsolateOrigins", "site-per-process")
	l.Set("webrtc-ip-handling-policy", "disable_non_proxied_udp")

	// User-supplied extra args (appended verbatim in TS).
	for _, a := range opts.Args {
		a = strings.TrimPrefix(strings.TrimSpace(a), "--")
		if a == "" {
			continue
		}
		if k, v, ok := strings.Cut(a, "="); ok {
			l.Set(flags.Flag(k), v)
		} else {
			l.Set(flags.Flag(a))
		}
	}

	// Headless extras.
	if opts.Headless {
		l.Set("headless", "new")
		l.Set("mute-audio")
		l.Set("hide-scrollbars")
	}

	// Extensions (only when not headless).
	if !opts.Headless && len(opts.Extensions) > 0 {
		var valid []string
		for _, e := range opts.Extensions {
			if dirExists(e) && fileExists(filepath.Join(e, "manifest.json")) {
				valid = append(valid, e)
			}
		}
		if len(valid) > 0 {
			joined := strings.Join(valid, ",")
			l.Set("disable-extensions-except", joined)
			l.Set("load-extension", joined)
		}
	}

	if proxyServerURL != "" {
		l.Set(flags.ProxyServer, proxyServerURL)
	}

	return l, nil
}

// ---- launch ----

// LaunchChrome launches (or reconnects to) Chrome for a profile via go-rod, applies the
// CDP anti-detect sequence, writes the session lock, and returns a LaunchResult.
func LaunchChrome(profile *StoredProfile, userDataDir string, opts LaunchOptions) (*LaunchResult, error) {
	unlock := lockProfile(profile.ID)
	defer unlock()

	// Same-process reuse: return the already-tracked launch (preserves its proxy cleanup + tracking).
	if lr := getTracked(profile.ID); lr != nil {
		return lr, nil
	}

	// Session reuse across processes.
	if lock := readLockFile(userDataDir); lock != nil {
		if existing := tryConnectExisting(lock); existing != nil {
			id := profile.ID
			lr := &LaunchResult{WsEndpoint: existing.WsEndpoint, PID: existing.PID, Port: existing.Port, ProfileID: id}
			lr.Close = func() error {
				if isProcessRunning(lr.PID) {
					terminateProcess(lr.PID)
				}
				deleteLockFile(userDataDir)
				untrackBrowser(id)
				// Cross-process reuse: a forward proxy (if any) is owned by the launching
				// process and cannot be torn down from here (accepted same-process-only limit).
				return nil
			}
			trackBrowser(id, lr)
			return lr, nil
		}
		deleteLockFile(userDataDir) // stale
	}

	// Clean stale Chrome singleton files that block launch.
	for _, f := range []string{"SingletonLock", "SingletonCookie", "SingletonSocket"} {
		_ = os.Remove(filepath.Join(userDataDir, f))
	}

	proxyServerURL, proxyCleanup, err := resolveProxy(profile.Proxy)
	if err != nil {
		return nil, err
	}
	l, err := buildLauncher(profile, userDataDir, opts, proxyServerURL)
	if err != nil {
		if proxyCleanup != nil {
			_ = proxyCleanup()
		}
		return nil, err
	}

	// Two-timezone quirk: env TZ from proxy geo-IP / system; CDP override uses profile.Timezone.
	envTZ := resolveEnvTimezone(profile)
	if envTZ != "" {
		l.Env(append(os.Environ(), "TZ="+envTZ)...)
	}

	controlURL, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launch chrome: %w", err)
	}
	pid := l.PID()
	port := parseWSPort(controlURL)

	browser := rod.New().ControlURL(controlURL)
	if err := browser.Connect(); err != nil {
		l.Kill()
		if proxyCleanup != nil {
			_ = proxyCleanup()
		}
		return nil, fmt.Errorf("connect cdp: %w", err)
	}

	page, err := defaultPage(browser)
	if err != nil {
		_ = browser.Close()
		l.Kill()
		if proxyCleanup != nil {
			_ = proxyCleanup()
		}
		return nil, fmt.Errorf("acquire page: %w", err)
	}

	if err := applyAntiDetect(page, profile); err != nil {
		_ = browser.Close()
		l.Kill()
		if proxyCleanup != nil {
			_ = proxyCleanup()
		}
		return nil, err
	}

	id := profile.ID
	lr := &LaunchResult{WsEndpoint: controlURL, PID: pid, Port: port, ProfileID: id}
	lr.Close = func() error {
		_ = browser.Close()
		l.Kill()
		deleteLockFile(userDataDir)
		untrackBrowser(id)
		if proxyCleanup != nil {
			_ = proxyCleanup()
		}
		return nil
	}
	trackBrowser(id, lr)
	writeLockFile(userDataDir, browserLockInfo{
		PID: pid, Port: port, WsEndpoint: controlURL, StartedAt: time.Now().UnixMilli(), ProxyURL: proxyServerURL,
	})
	return lr, nil
}

// applyAntiDetect runs the TS CDP override sequence on the default page:
// Network.enable → SetUserAgent(+metadata) → EvalOnNewDocument(scripts) → timezone → cookies.
func applyAntiDetect(page *rod.Page, profile *StoredProfile) error {
	if err := (proto.NetworkEnable{}).Call(page); err != nil {
		return fmt.Errorf("network enable: %w", err)
	}

	fp := profile.Fingerprint
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	platform := "Win32"
	language := "en-US"
	if fp != nil {
		if fp.UserAgent != "" {
			ua = fp.UserAgent
		}
		if fp.Platform != "" {
			platform = fp.Platform
		}
		if fp.Language != "" {
			language = fp.Language
		}
	}
	chPlatform := "Linux"
	chPlatVer := "14.0.0"
	switch {
	case strings.Contains(platform, "Win"):
		chPlatform, chPlatVer = "Windows", "10.0.0"
	case strings.Contains(platform, "Mac"):
		chPlatform = "macOS"
	}
	meta := &proto.EmulationUserAgentMetadata{
		Brands: []*proto.EmulationUserAgentBrandVersion{
			{Brand: "Not_A Brand", Version: "8"},
			{Brand: "Chromium", Version: "120"},
			{Brand: "Google Chrome", Version: "120"},
		},
		FullVersion:     "120.0.0.0",
		Platform:        chPlatform,
		PlatformVersion: chPlatVer,
		Architecture:    "x86",
		Model:           "",
		Mobile:          false,
	}
	if err := page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: ua, AcceptLanguage: language, Platform: platform, UserAgentMetadata: meta,
	}); err != nil {
		return fmt.Errorf("set user agent: %w", err)
	}

	hw, mem := 8, 8
	if fp != nil {
		if fp.HardwareConcurrency != 0 {
			hw = fp.HardwareConcurrency
		}
		if fp.DeviceMemory != 0 {
			mem = fp.DeviceMemory
		}
	}
	var webglCfg *fingerprint.WebGLScriptConfig
	if fp != nil && fp.WebGL != nil {
		webglCfg = &fingerprint.WebGLScriptConfig{Vendor: fp.WebGL.Vendor, Renderer: fp.WebGL.Renderer}
	}
	script := fingerprint.GetAllProtectionScripts(&fingerprint.AllProtectionOptions{
		Navigator: &fingerprint.NavigatorConfig{
			Language: language, Platform: platform, HardwareConcurrency: hw, DeviceMemory: mem,
		},
		WebGLConfig: webglCfg,
	})
	if _, err := page.EvalOnNewDocument(script); err != nil {
		return fmt.Errorf("inject protection scripts: %w", err)
	}

	tz := profile.Timezone
	if tz == "" {
		tz = "America/New_York"
	}
	if err := (proto.EmulationSetTimezoneOverride{TimezoneID: tz}).Call(page); err != nil {
		return fmt.Errorf("set timezone: %w", err)
	}

	for _, c := range profile.Cookies {
		sameSite := proto.NetworkCookieSameSite(c.SameSite)
		if c.SameSite == "" {
			sameSite = proto.NetworkCookieSameSiteLax
		}
		sc := proto.NetworkSetCookie{
			URL: "https://" + c.Domain, Name: c.Name, Value: c.Value, Domain: c.Domain,
			Path: firstNonEmpty(c.Path, "/"), HTTPOnly: c.HTTPOnly, Secure: c.Secure, SameSite: sameSite,
		}
		if c.Expires != 0 {
			sc.Expires = proto.TimeSinceEpoch(c.Expires)
		}
		_, _ = sc.Call(page) // errors swallowed, matching TS
	}
	return nil
}

func defaultPage(b *rod.Browser) (*rod.Page, error) {
	if pages, err := b.Pages(); err == nil && len(pages) > 0 {
		return pages[0], nil
	}
	return b.Page(proto.TargetCreateTarget{URL: "about:blank"})
}

func parseWSPort(ws string) int {
	if u, err := url.Parse(ws); err == nil {
		if p, err := strconv.Atoi(u.Port()); err == nil {
			return p
		}
	}
	return 0
}

// systemTimezone best-effort mirrors Intl.DateTimeFormat().resolvedOptions().timeZone.
func systemTimezone() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}
	if p, err := os.Readlink("/etc/localtime"); err == nil {
		if i := strings.Index(p, "zoneinfo/"); i >= 0 {
			return p[i+len("zoneinfo/"):]
		}
	}
	return "UTC"
}

// resolveEnvTimezone computes the TZ handed to Chrome's process environment — the "env"
// half of the two-timezone quirk: profile.Timezone wins; else the proxy's geo-IP timezone;
// else the system zone. The CDP Emulation.setTimezoneOverride path uses profile.Timezone
// directly (see applyAntiDetect) and is NOT this value — the two agree only when
// profile.Timezone is set. Pure (modulo the geo-IP lookup) so it is unit-testable.
func resolveEnvTimezone(profile *StoredProfile) string {
	if profile.Timezone != "" {
		return profile.Timezone
	}
	if profile.Proxy != nil {
		if gi, _ := DetectTimezoneFromIP(profile.Proxy.Host); gi != nil && gi.Timezone != "" {
			return gi.Timezone
		}
	}
	return systemTimezone()
}

// ---- standalone launch (no persisted profile) ----

// StandaloneLaunchOptions configures a standalone Chrome launch (mirrors the TS type).
type StandaloneLaunchOptions struct {
	Headless    bool
	ChromePath  string
	UserDataDir string
	Proxy       *ProxyConfig
	Timezone    string
	Fingerprint *FingerprintConfig
	Args        []string
	Extensions  []string
}

// StandaloneLaunchResult is the result of a standalone launch (no ProfileID).
type StandaloneLaunchResult struct {
	WsEndpoint string
	PID        int
	Port       int
	Close      func() error
}

// LaunchChromeStandalone launches Chrome with a synthetic profile and a temp user-data-dir.
func LaunchChromeStandalone(opts StandaloneLaunchOptions) (*StandaloneLaunchResult, error) {
	id := fmt.Sprintf("standalone-%d-%s", time.Now().UnixMilli(), randHex(4))
	userDataDir := opts.UserDataDir
	createdTemp := false
	if userDataDir == "" {
		userDataDir = filepath.Join(os.TempDir(), "chrome-"+id)
		createdTemp = true
	}
	profile := &StoredProfile{ProfileConfig: ProfileConfig{
		ID: id, Name: id, Proxy: opts.Proxy, Timezone: opts.Timezone, Fingerprint: opts.Fingerprint,
	}}
	lr, err := LaunchChrome(profile, userDataDir, LaunchOptions{
		Headless: opts.Headless, ChromePath: opts.ChromePath, Args: opts.Args, Extensions: opts.Extensions,
	})
	if err != nil {
		if createdTemp {
			_ = os.RemoveAll(userDataDir)
		}
		return nil, err
	}
	closeFn := func() error {
		_ = lr.Close()
		if createdTemp {
			_ = os.RemoveAll(userDataDir)
		}
		return nil
	}
	return &StandaloneLaunchResult{WsEndpoint: lr.WsEndpoint, PID: lr.PID, Port: lr.Port, Close: closeFn}, nil
}

// ---- small helpers ----

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
