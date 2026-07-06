package browserprofiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// fakeChrome creates an executable-ish temp file to satisfy GetChromePath without a real browser.
func fakeChrome(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "chrome")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestBuildLauncherB1 is the load-bearing anti-detect check: the launched flag set must NOT
// contain --enable-automation (go-rod's default that sets navigator.webdriver=true) nor other
// go-rod defaults outside the TS set, and MUST contain the exact TS stealth flags. (B1)
func TestBuildLauncherB1(t *testing.T) {
	profile := &StoredProfile{ProfileConfig: ProfileConfig{
		ID: "b1", Name: "b1", Fingerprint: &FingerprintConfig{Language: "en-US"},
	}}
	l, err := buildLauncher(profile, t.TempDir(), LaunchOptions{Headless: true, ChromePath: fakeChrome(t)}, "")
	if err != nil {
		t.Fatalf("buildLauncher: %v", err)
	}
	args := strings.Join(l.FormatArgs(), " ")

	// B1: these must be ABSENT (go-rod defaults that leak automation / are outside the TS set).
	forbidden := []string{
		"--enable-automation", "--use-mock-keychain", "--force-color-profile",
		"--disable-breakpad", "--disable-ipc-flooding-protection", "--disable-renderer-backgrounding",
		"--disable-backgrounding-occluded-windows", "--disable-background-timer-throttling",
		"--disable-component-extensions-with-background-pages", "--disable-site-isolation-trials",
		"--no-startup-window",
	}
	for _, f := range forbidden {
		if strings.Contains(args, f) {
			t.Errorf("forbidden flag present: %s\nargs: %s", f, args)
		}
	}

	// Required exact TS stealth flags.
	required := []string{
		"--no-first-run", "--no-default-browser-check", "--disable-background-networking",
		"--disable-client-side-phishing-detection", "--disable-default-apps", "--disable-hang-monitor",
		"--disable-popup-blocking", "--disable-prompt-on-repost", "--disable-sync", "--disable-translate",
		"--metrics-recording-only", "--no-sandbox", "--disable-setuid-sandbox", "--disable-dev-shm-usage",
		"--lang=en-US", "--disable-blink-features=AutomationControlled", "--disable-infobars",
		"--disable-extensions-file-access-check", "--enable-features=NetworkService,NetworkServiceInProcess",
		"--disable-features=IsolateOrigins,site-per-process", "--webrtc-ip-handling-policy=disable_non_proxied_udp",
		"--force-webrtc-ip-handling-policy", "--headless=new", "--mute-audio", "--hide-scrollbars",
		"--user-data-dir=",
	}
	for _, f := range required {
		if !strings.Contains(args, f) {
			t.Errorf("required flag missing: %s\nargs: %s", f, args)
		}
	}
}

func TestResolveProxyNoCreds(t *testing.T) {
	url, cleanup, err := resolveProxy(&ProxyConfig{Type: "http", Host: "127.0.0.1", Port: 8080})
	if err != nil || url != "http://127.0.0.1:8080" || cleanup != nil {
		t.Fatalf("resolveProxy no-creds: url=%q cleanupNil=%v err=%v", url, cleanup == nil, err)
	}
	profile := &StoredProfile{ProfileConfig: ProfileConfig{ID: "p", Name: "p"}}
	l, err := buildLauncher(profile, t.TempDir(), LaunchOptions{ChromePath: fakeChrome(t)}, url)
	if err != nil {
		t.Fatalf("buildLauncher: %v", err)
	}
	if !strings.Contains(strings.Join(l.FormatArgs(), " "), "--proxy-server=http://127.0.0.1:8080") {
		t.Fatalf("proxy flag missing: %v", l.FormatArgs())
	}
}

func TestResolveProxyNil(t *testing.T) {
	url, cleanup, err := resolveProxy(nil)
	if url != "" || cleanup != nil || err != nil {
		t.Fatalf("resolveProxy(nil) = url=%q cleanupNil=%v err=%v", url, cleanup == nil, err)
	}
}

func TestBuildProxyURL(t *testing.T) {
	cases := []struct {
		p    *ProxyConfig
		want string
	}{
		{&ProxyConfig{Type: "http", Host: "h", Port: 8080}, "http://h:8080"},
		{&ProxyConfig{Type: "socks5", Host: "h", Port: 1080}, "socks5://h:1080"},
		{&ProxyConfig{Type: "https", Host: "h", Port: 3128}, "http://h:3128"}, // https maps to http scheme (TS)
		{&ProxyConfig{Type: "http", Host: "h", Port: 8080, Username: "u", Password: "p"}, "http://u:p@h:8080"},
	}
	for _, c := range cases {
		if got := buildProxyURL(c.p); got != c.want {
			t.Errorf("buildProxyURL(%+v) = %s, want %s", c.p, got, c.want)
		}
	}
}

func TestGetChromePathCustom(t *testing.T) {
	fc := fakeChrome(t)
	got, err := GetChromePath(fc)
	if err != nil || got != fc {
		t.Fatalf("GetChromePath(custom) = %q, %v", got, err)
	}
}

func TestLockFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := browserLockInfo{PID: 1234, Port: 9222, WsEndpoint: "ws://127.0.0.1:9222/x", StartedAt: 999, ProxyURL: "http://p:1"}
	writeLockFile(dir, in)
	got := readLockFile(dir)
	if got == nil || *got != in {
		t.Fatalf("lock round-trip: %+v", got)
	}
	deleteLockFile(dir)
	if readLockFile(dir) != nil {
		t.Fatal("lock not deleted")
	}
}

func TestParseWSPort(t *testing.T) {
	if p := parseWSPort("ws://127.0.0.1:38913/devtools/browser/abc"); p != 38913 {
		t.Fatalf("parseWSPort = %d", p)
	}
}

// TestLaunchRuntimeSmoke launches real headless Chrome and proves, at runtime, that
// navigator.webdriver is false WITHOUT relying on the injected JS mask (the B1 concern),
// and that the full CDP anti-detect sequence executes without error. Skips if no Chrome.
func TestLaunchRuntimeSmoke(t *testing.T) {
	if _, err := GetChromePath(""); err != nil {
		t.Skip("no Chrome/Chromium available")
	}
	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
	p, err := bp.Create(ProfileConfig{Name: "smoke", Fingerprint: &FingerprintConfig{Platform: "Win32", Language: "en-US"}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	lr, err := bp.Launch(p.ID, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("launch (chrome present, so this is a real failure): %v", err)
	}
	defer lr.Close()

	if lr.WsEndpoint == "" || lr.PID <= 0 || lr.Port <= 0 {
		t.Fatalf("bad LaunchResult: %+v", lr)
	}
	if got := bp.GetRunning(); len(got) != 1 || got[0] != p.ID {
		t.Fatalf("tracking: %v", got)
	}

	browser := rod.New().ControlURL(lr.WsEndpoint)
	if err := browser.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	page.MustWaitLoad()
	obj, err := page.Eval(`() => navigator.webdriver`)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	// B1 runtime proof: with --enable-automation stripped, navigator.webdriver is false/undefined.
	if obj.Value.Bool() {
		t.Fatalf("navigator.webdriver is TRUE — --enable-automation leaked (B1 regression)")
	}

	lr.Close()
	if len(bp.GetRunning()) != 0 {
		t.Fatalf("tracking not cleared after close: %v", bp.GetRunning())
	}
}
