package browserprofiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// ============================================================================
// Bucket A — REAL go-rod + REAL Chrome integration tests.
//
// These exercise the browser-dependent surface against a real headless Chrome
// (fakes would test the mock, not the behavior). The running-browser tracking
// map is package-GLOBAL, so every test here: (a) never runs t.Parallel(),
// (b) tears down every launch (Close/Terminate + a CloseAllBrowsers safety
// net), and (c) uses a unique profile ID. requireChrome skips only when no
// browser is installed; with Chrome present a launch error is a hard failure.
// ============================================================================

// requireChrome skips the test when no Chrome/Chromium is installed. When a
// browser IS present, callers treat launch errors as real failures (t.Fatal).
func requireChrome(t *testing.T) {
	t.Helper()
	if _, err := GetChromePath(""); err != nil {
		t.Skip("no Chrome/Chromium available")
	}
}

// runningHas reports whether ids contains want. Assertions use "contains this
// profile ID" rather than len(GetRunning()) so a sibling leak can't flake us.
func runningHas(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// TestApplyAntiDetectTimezoneAndCookies (LAUNCH-03) proves two side effects of the
// CDP anti-detect sequence on a really-launched browser: profile.Timezone reaches
// the browser (Intl resolves to it) and profile.Cookies are injected into the
// browser cookie store. Navigator overrides are proven elsewhere (M5).
func TestApplyAntiDetectTimezoneAndCookies(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
	p, err := bp.Create(ProfileConfig{
		ID:          "int-tz-01",
		Name:        "tz",
		Timezone:    "Asia/Tokyo",
		Cookies:     []ProfileCookie{{Name: "sess", Value: "xyz", Domain: "example.com"}},
		Fingerprint: &FingerprintConfig{Platform: "Win32", Language: "en-US", HardwareConcurrency: 8, DeviceMemory: 8},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	lr, err := bp.Launch(p.ID, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	t.Cleanup(func() { _ = lr.Close() })

	browser := rod.New().ControlURL(lr.WsEndpoint)
	if err := browser.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = browser.Close() })

	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		t.Fatalf("open page: %v", err)
	}
	navigate(t, page)

	if tz := evalStr(t, page, "() => Intl.DateTimeFormat().resolvedOptions().timeZone"); tz != "Asia/Tokyo" {
		t.Fatalf("resolved timeZone = %q, want Asia/Tokyo (profile.Timezone not applied)", tz)
	}

	cookies, err := browser.GetCookies()
	if err != nil {
		t.Fatalf("GetCookies: %v", err)
	}
	found := false
	for _, c := range cookies {
		if c.Name == "sess" && c.Value == "xyz" && strings.TrimPrefix(c.Domain, ".") == "example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("cookie sess=xyz for example.com not injected; store has %d cookie(s): %+v", len(cookies), cookies)
	}
}

// TestLaunchDoubleWriteMetadata (ORCH-01) proves BrowserProfiles.Launch's
// two-step config.json write: after a real launch, the reloaded profile has
// updatedAt bumped past the preserved createdAt (the empty Update patch) AND a
// non-zero lastLaunchedAt (the separate lastLaunchedAt write).
func TestLaunchDoubleWriteMetadata(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
	p, err := bp.Create(ProfileConfig{
		ID:          "int-orch-01",
		Name:        "orch",
		Fingerprint: &FingerprintConfig{Platform: "Win32", Language: "en-US"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	createdAt := p.CreatedAt

	// Force a distinct millisecond so a same-tick write can't masquerade as a bump.
	time.Sleep(2 * time.Millisecond)

	lr, err := bp.Launch(p.ID, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	t.Cleanup(func() { _ = lr.Close() })

	p2, _ := bp.Get(p.ID)
	if p2 == nil {
		t.Fatal("reload profile from disk: nil")
	}
	if p2.CreatedAt != createdAt {
		t.Fatalf("CreatedAt changed across launch: got %d, want preserved %d", p2.CreatedAt, createdAt)
	}
	if p2.UpdatedAt <= p2.CreatedAt {
		t.Fatalf("UpdatedAt (%d) not bumped past CreatedAt (%d) — the updatedAt write is missing", p2.UpdatedAt, p2.CreatedAt)
	}
	if p2.LastLaunchedAt <= createdAt {
		t.Fatalf("LastLaunchedAt (%d) not set to a launch-time value (> createdAt %d) — the lastLaunchedAt write is missing", p2.LastLaunchedAt, createdAt)
	}
}

// TestCloseAllAndDeleteOnLive (ORCH-02) proves two orchestration behaviors on
// really-running browsers: CloseAll drops a live profile from tracking, and
// Delete on a live profile both closes it (drops tracking) and removes it from
// disk.
func TestCloseAllAndDeleteOnLive(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})

	// --- CloseAll clears tracking for a live profile ---
	pa, err := bp.Create(ProfileConfig{ID: "int-closeall-01", Name: "a", Fingerprint: &FingerprintConfig{Platform: "Win32", Language: "en-US"}})
	if err != nil {
		t.Fatalf("create a: %v", err)
	}
	if _, err := bp.Launch(pa.ID, LaunchOptions{Headless: true}); err != nil {
		t.Fatalf("launch a: %v", err)
	}
	if !runningHas(bp.GetRunning(), pa.ID) {
		t.Fatalf("profile %q not tracked after launch", pa.ID)
	}
	if err := bp.CloseAll(); err != nil {
		t.Fatalf("CloseAll: %v", err)
	}
	if runningHas(bp.GetRunning(), pa.ID) {
		t.Fatalf("profile %q still tracked after CloseAll", pa.ID)
	}

	// --- Delete on a live profile closes it AND removes it from disk ---
	pb, err := bp.Create(ProfileConfig{ID: "int-delete-01", Name: "b", Fingerprint: &FingerprintConfig{Platform: "Win32", Language: "en-US"}})
	if err != nil {
		t.Fatalf("create b: %v", err)
	}
	if _, err := bp.Launch(pb.ID, LaunchOptions{Headless: true}); err != nil {
		t.Fatalf("launch b: %v", err)
	}
	if !runningHas(bp.GetRunning(), pb.ID) {
		t.Fatalf("profile %q not tracked after launch", pb.ID)
	}
	ok, err := bp.Delete(pb.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !ok {
		t.Fatal("Delete returned false for an existing profile")
	}
	if runningHas(bp.GetRunning(), pb.ID) {
		t.Fatalf("profile %q still tracked after Delete-while-live", pb.ID)
	}
	if got, _ := bp.Get(pb.ID); got != nil {
		t.Fatalf("profile %q still on disk after Delete", pb.ID)
	}
}

// TestLaunchChromeStandaloneFingerprint (LAUNCH-05) proves LaunchChromeStandalone
// spins up a usable browser (connectable ws, valid pid/port) and injects its
// fingerprint into the launch target. hardwareConcurrency=8 is the sentinel: the
// host reports its true core count (≠8), so seeing 8 can only be the override.
func TestLaunchChromeStandaloneFingerprint(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	sr, err := LaunchChromeStandalone(StandaloneLaunchOptions{
		Headless:    true,
		Fingerprint: &FingerprintConfig{Platform: "Linux x86_64", Language: "en-US", HardwareConcurrency: 8, DeviceMemory: 8},
	})
	if err != nil {
		t.Fatalf("LaunchChromeStandalone: %v", err)
	}
	t.Cleanup(func() { _ = sr.Close() })

	if sr.WsEndpoint == "" || sr.PID <= 0 || sr.Port <= 0 {
		t.Fatalf("bad StandaloneLaunchResult: %+v", sr)
	}

	browser := rod.New().ControlURL(sr.WsEndpoint)
	if err := browser.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = browser.Close() })

	// applyAntiDetect ran on the launch target (the default page); the injection
	// is per-page via EvalOnNewDocument, so assert on pages[0].
	pages, err := browser.Pages()
	if err != nil || len(pages) == 0 {
		t.Fatalf("no default page on standalone browser: %v", err)
	}
	page := pages[0]
	navigate(t, page)
	if hw := evalInt(t, page, "() => navigator.hardwareConcurrency"); hw != 8 {
		t.Fatalf("navigator.hardwareConcurrency = %d, want 8 (standalone fingerprint not injected on the launch target)", hw)
	}
}

// TestLaunchCrossProcessReuse (LAUNCH-04) is the REAL replacement for the deleted
// fake TestLaunchChromeReuseNoSpawn. It launches a profile (writing the on-disk
// lock), simulates a fresh process by dropping only the in-memory tracking with
// untrackBrowser (Chrome + lock survive), then launches AGAIN — which must adopt
// the live browser via readLockFile → tryConnectExisting (/json/version) rather
// than spawn a second Chrome. Proof of no-respawn: identical PID and port.
func TestLaunchCrossProcessReuse(t *testing.T) {
	requireChrome(t)

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
	p, err := bp.Create(ProfileConfig{
		ID:          "int-reuse-01",
		Name:        "reuse",
		Fingerprint: &FingerprintConfig{Platform: "Win32", Language: "en-US", HardwareConcurrency: 8, DeviceMemory: 8},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	lr1, err := bp.Launch(p.ID, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("first launch: %v", err)
	}
	// Reap the real process via the launcher-backed Close (SIGKILL + leakless).
	// Registered before the safety net so it runs first (LIFO).
	t.Cleanup(func() { CloseAllBrowsers() })
	t.Cleanup(func() { _ = lr1.Close() })

	pid1, port1 := lr1.PID, lr1.Port
	if pid1 <= 0 || port1 <= 0 {
		t.Fatalf("bad first launch result: pid=%d port=%d", pid1, port1)
	}

	// Simulate a fresh process: drop in-memory tracking only. The real Chrome and
	// its .browser-lock.json stay on disk so the next Launch must go through the
	// cross-process reuse path.
	untrackBrowser(p.ID)
	if runningHas(bp.GetRunning(), p.ID) {
		t.Fatalf("profile %q still tracked after untrackBrowser", p.ID)
	}

	lr2, err := bp.Launch(p.ID, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("second launch (reuse): %v", err)
	}
	if lr2.PID != pid1 {
		t.Fatalf("reuse spawned a NEW Chrome: pid %d != first pid %d", lr2.PID, pid1)
	}
	if lr2.Port != port1 {
		t.Fatalf("reuse bound a different port: %d != first port %d", lr2.Port, port1)
	}
	if !runningHas(bp.GetRunning(), p.ID) {
		t.Fatalf("reused browser not re-tracked for %q", p.ID)
	}
}

// TestLaunchStaleLockAndSingletonCleanup (LAUNCH-04) proves a poisoned
// --user-data-dir does not hijack or block a launch. A stale .browser-lock.json
// pointing at a dead port must NOT be adopted (tryConnectExisting fails, the lock
// is discarded), and leftover Chrome Singleton* files must not block startup:
// LaunchChrome launches a FRESH browser on a live port and rewrites the lock.
func TestLaunchStaleLockAndSingletonCleanup(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	dir := t.TempDir()
	const deadPort = 1 // nothing listens on :1 → tryConnectExisting fails fast
	const sentinel = "STALE-SENTINEL"
	singletons := []string{"SingletonLock", "SingletonCookie", "SingletonSocket"}
	for _, f := range singletons {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(sentinel), 0o600); err != nil {
			t.Fatalf("plant %s: %v", f, err)
		}
	}
	// Stale lock with a dead port and a bogus ws endpoint.
	writeLockFile(dir, browserLockInfo{PID: 999999999, Port: deadPort, WsEndpoint: "ws://127.0.0.1:1/dead"})

	profile := &StoredProfile{ProfileConfig: ProfileConfig{ID: "int-stale-01", Name: "stale"}}
	lr, err := LaunchChrome(profile, dir, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("launch with poisoned dir failed (stale lock / singleton files blocked it): %v", err)
	}
	t.Cleanup(func() { _ = lr.Close() })

	// The stale dead-port lock was NOT adopted: we got a fresh, live launch.
	if lr.Port == deadPort {
		t.Fatalf("adopted the stale dead-port lock (port=%d) instead of launching fresh", deadPort)
	}
	if lr.Port <= 0 || lr.PID <= 0 {
		t.Fatalf("bad launch result: %+v", lr)
	}
	if !isProcessRunning(lr.PID) {
		t.Fatalf("launched PID %d is not running", lr.PID)
	}

	// The stale lock on disk was replaced with one pointing at the live port.
	nl := readLockFile(dir)
	if nl == nil {
		t.Fatal("no .browser-lock.json after launch")
	}
	if nl.Port != lr.Port {
		t.Fatalf("lock port %d != live port %d (stale lock not replaced)", nl.Port, lr.Port)
	}

	// Our planted stale Singleton* regular files did not survive: Chrome could
	// only start (and recreate its own singleton symlinks) because they were cleaned.
	for _, f := range singletons {
		if b, rerr := os.ReadFile(filepath.Join(dir, f)); rerr == nil && string(b) == sentinel {
			t.Fatalf("%s still holds the stale sentinel content — the singleton cleanup was skipped", f)
		}
	}
}

// TestQuickLaunchAutoNamedLifecycle (INT-02) proves QuickLaunch with no Name
// creates a persistent auto-named "Quick-<millis>" profile, really launches it
// (injected fingerprint present), and — because it was auto-named — deletes it
// on Terminate. HOME is redirected to a temp dir so the default manager path
// (~/.aitofy/browser-profiles) never touches the real home.
func TestQuickLaunchAutoNamedLifecycle(t *testing.T) {
	requireChrome(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Cleanup(func() { CloseAllBrowsers() })

	sess, err := QuickLaunch(QuickLaunchOptions{
		Headless:    true,
		Fingerprint: &FingerprintConfig{Platform: "QuickLaunch-sentinel", Language: "en-US", HardwareConcurrency: 8, DeviceMemory: 8},
	})
	if err != nil {
		t.Fatalf("QuickLaunch: %v", err)
	}
	if sess.Page == nil || sess.Browser == nil {
		t.Fatal("session missing Page/Browser")
	}
	if !strings.HasPrefix(sess.Profile.Name, "Quick-") {
		t.Fatalf("auto-name = %q, want a Quick-<millis> prefix", sess.Profile.Name)
	}
	id := sess.ID

	// Real launch worked: the profile fingerprint is injected on the session page.
	navigate(t, sess.Page)
	if plat := evalStr(t, sess.Page, "() => navigator.platform"); plat != "QuickLaunch-sentinel" {
		t.Fatalf("navigator.platform = %q, want QuickLaunch-sentinel (fingerprint not injected)", plat)
	}

	// The auto-named profile was persisted under the temp-HOME store.
	store := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: filepath.Join(home, ".aitofy", "browser-profiles")})
	if got, _ := store.Get(id); got == nil {
		t.Fatalf("auto-named profile %q was not persisted", id)
	}

	// Terminate deletes the auto-named profile and drops it from tracking.
	if err := sess.Terminate(); err != nil {
		t.Fatalf("terminate: %v", err)
	}
	if got, _ := store.Get(id); got != nil {
		t.Fatalf("auto-named profile %q was not deleted on Terminate", id)
	}
	if runningHas(GetRunningBrowsers(), id) {
		t.Fatalf("profile %q still tracked after Terminate", id)
	}
}

// TestCreateSessionTemporary (INT-03, temporary path) proves CreateSession's
// default temporary flow launches via LaunchChromeStandalone and returns a live
// Session whose page carries the fingerprint. An explicit Platform override
// (which wins over the random fingerprint) is the machine-independent sentinel.
func TestCreateSessionTemporary(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	sess, err := CreateSession(CreateSessionOptions{
		Headless:    true,
		Fingerprint: &FingerprintConfig{Platform: "CreateSession-sentinel", Language: "en-US"},
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() { _ = sess.Terminate() })

	if sess.Page == nil || sess.Browser == nil {
		t.Fatal("session missing Page/Browser")
	}
	navigate(t, sess.Page)
	if plat := evalStr(t, sess.Page, "() => navigator.platform"); plat != "CreateSession-sentinel" {
		t.Fatalf("navigator.platform = %q, want CreateSession-sentinel (fingerprint override not injected)", plat)
	}
}

// TestPatchPageInjectionOnExternalPage (INT-03) proves PatchPage really injects
// its protection subset into an EXTERNAL page. A page opened directly on a
// standalone browser is not auto-patched (no M5 loop), so its navigator.platform
// is the host's real value; after PatchPage it must become the sentinel.
func TestPatchPageInjectionOnExternalPage(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	sr, err := LaunchChromeStandalone(StandaloneLaunchOptions{
		Headless:    true,
		Fingerprint: &FingerprintConfig{Platform: "Win32", Language: "en-US", HardwareConcurrency: 8, DeviceMemory: 8},
	})
	if err != nil {
		t.Fatalf("LaunchChromeStandalone: %v", err)
	}
	t.Cleanup(func() { _ = sr.Close() })

	browser := rod.New().ControlURL(sr.WsEndpoint)
	if err := browser.Connect(); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = browser.Close() })

	// A page opened directly on the browser is NOT auto-patched by the launcher.
	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		t.Fatalf("open external page: %v", err)
	}

	if err := PatchPage(page, PatchPageOptions{
		Fingerprint: &FingerprintConfig{Platform: "FreeBSD-sentinel", Language: "en-US", HardwareConcurrency: 8, DeviceMemory: 8},
	}); err != nil {
		t.Fatalf("PatchPage: %v", err)
	}
	navigate(t, page)
	if plat := evalStr(t, page, "() => navigator.platform"); plat != "FreeBSD-sentinel" {
		t.Fatalf("navigator.platform = %q, want FreeBSD-sentinel (PatchPage injection did not take effect)", plat)
	}
}

// TestChromeSkipWhenMissing pins the graceful-skip contract: with CHROME_PATH and
// CHROMIUM_PATH unset, GetChromePath returns an error and CreateSession returns a
// skip-safe error rather than crashing the suite.
func TestChromeSkipWhenMissing(t *testing.T) {
	t.Setenv("CHROME_PATH", "")
	t.Setenv("CHROMIUM_PATH", "")
	if _, err := GetChromePath(""); err == nil {
		t.Skip("Chrome was found via platform defaults; cannot test the missing-Chrome skip path")
	}
	_, err := CreateSession(CreateSessionOptions{})
	if err == nil {
		t.Fatal("CreateSession without Chrome should return an error")
	}
}

// TestCrossContextInjection verifies that navigator.hardwareConcurrency is spoofed
// inside a newly created iframe, not just the top-level page. Real Chrome required.
func TestCrossContextInjection(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
	p, err := bp.Create(ProfileConfig{
		ID:          "crossctx-01",
		Name:        "crossctx",
		Fingerprint: &FingerprintConfig{Platform: "Win32", Language: "en-US", HardwareConcurrency: 12, DeviceMemory: 8},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	sess, err := WithProfile(bp, p.ID, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("WithProfile: %v", err)
	}
	t.Cleanup(func() { _ = sess.Terminate() })

	page := sess.Page
	navigate(t, page)

	// Create an iframe and read navigator.hardwareConcurrency inside it.
	_, err = page.Eval(`() => {
		const f = document.createElement('iframe');
		f.id = 'probe';
		document.body.appendChild(f);
		return 'ok';
	}`)
	if err != nil {
		t.Fatalf("create iframe: %v", err)
	}
	hw := evalInt(t, page, `() => document.getElementById('probe').contentWindow.navigator.hardwareConcurrency`)
	if hw != 12 {
		t.Fatalf("iframe hardwareConcurrency = %d, want 12", hw)
	}
}

// TestAntiDetectModesSmoke verifies that the WebRTC / Canvas / Audio mode fields
// actually change runtime behavior in a real headless Chrome. It uses two profiles
// (non-default and default modes) and checks the observable API shape on each page.
// Skips gracefully when Chrome is unavailable.
func TestAntiDetectModesSmoke(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})

	// Non-default combination: WebRTC disabled, canvas real, audio noised.
	nonDefault, err := bp.Create(ProfileConfig{
		ID:   "modes-nondflt-01",
		Name: "modes-nondflt",
		Fingerprint: &FingerprintConfig{
			Platform:            "Win32",
			Language:            "en-US",
			HardwareConcurrency: 8,
			DeviceMemory:        8,
			WebRTC:              "disable",
			Canvas:              "real",
			Audio:               "noise",
		},
	})
	if err != nil {
		t.Fatalf("create non-default profile: %v", err)
	}

	sess, err := WithProfile(bp, nonDefault.ID, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("WithProfile non-default: %v", err)
	}
	t.Cleanup(func() { _ = sess.Terminate() })

	page := sess.Page
	navigate(t, page)

	if got := evalStr(t, page, "() => typeof window.RTCPeerConnection"); got != "undefined" {
		t.Fatalf("WebRTC disable: typeof RTCPeerConnection = %q, want undefined", got)
	}
	if got := evalStr(t, page, "() => HTMLCanvasElement.prototype.toDataURL.toString()"); !strings.Contains(got, "[native code]") {
		t.Fatalf("Canvas real: toDataURL toString = %q, want native code", got)
	}
	if got := evalStr(t, page, "() => CanvasRenderingContext2D.prototype.getImageData.toString()"); !strings.Contains(got, "[native code]") {
		t.Fatalf("Canvas real: getImageData toString = %q, want native code", got)
	}
	if got := evalStr(t, page, "() => AudioBuffer.prototype.getChannelData.toString()"); strings.Contains(got, "[native code]") {
		t.Fatalf("Audio noise: getChannelData toString = %q, want wrapped (non-native)", got)
	}

	// Default combination: v1.0 behavior (fake WebRTC, noisy canvas, noisy audio).
	defaultProf, err := bp.Create(ProfileConfig{
		ID:   "modes-default-01",
		Name: "modes-default",
		Fingerprint: &FingerprintConfig{
			Platform:            "Win32",
			Language:            "en-US",
			HardwareConcurrency: 8,
			DeviceMemory:        8,
		},
	})
	if err != nil {
		t.Fatalf("create default profile: %v", err)
	}

	sess2, err := WithProfile(bp, defaultProf.ID, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("WithProfile default: %v", err)
	}
	t.Cleanup(func() { _ = sess2.Terminate() })

	page2 := sess2.Page
	navigate(t, page2)

	if got := evalStr(t, page2, "() => typeof window.RTCPeerConnection"); got == "undefined" {
		t.Fatalf("WebRTC fake: RTCPeerConnection should be defined")
	}
	if got := evalStr(t, page2, "() => HTMLCanvasElement.prototype.toDataURL.toString()"); strings.Contains(got, "[native code]") {
		t.Fatalf("Canvas noise: toDataURL should be wrapped (non-native)")
	}
	if got := evalStr(t, page2, "() => AudioBuffer.prototype.getChannelData.toString()"); strings.Contains(got, "[native code]") {
		t.Fatalf("Audio noise: getChannelData should be wrapped (non-native)")
	}
}
