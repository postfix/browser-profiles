package browserprofiles

import (
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/postfix/browser-profiles/fingerprint"
)

// TestWithProfileSmokeAndM5 is a headless smoke test for WithProfile: it launches
// a profile, confirms the returned page carries the profile's fingerprint
// overrides, and — critically for M5 — confirms that a SECOND page opened via the
// browser is ALSO protected (proving the browser-level targetcreated re-injection
// reaches new tabs, not just the launch target). Skips when no Chrome is present.
func TestWithProfileSmokeAndM5(t *testing.T) {
	if _, err := GetChromePath(""); err != nil {
		t.Skipf("no Chrome available: %v", err)
	}

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})

	// A distinctive fingerprint: hardwareConcurrency=12 is the sentinel. The
	// launcher only injects it into the FIRST target, so seeing 12 on a
	// later-opened tab can only come from the M5 re-injection loop.
	profile, err := bp.Create(ProfileConfig{
		Name: "smoke-test",
		Fingerprint: &FingerprintConfig{
			Language:            "en-US",
			Platform:            "MacIntel",
			HardwareConcurrency: 12,
			DeviceMemory:        16,
		},
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	sess, err := WithProfile(bp, profile.ID, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("WithProfile: %v", err)
	}
	t.Cleanup(func() { _ = sess.Terminate() })

	if sess.Page == nil || sess.Browser == nil {
		t.Fatal("session missing Page/Browser")
	}

	// First page: navigate to a fresh document so the on-new-document script runs.
	navigate(t, sess.Page)
	if hw := evalInt(t, sess.Page, "() => navigator.hardwareConcurrency"); hw != 12 {
		t.Fatalf("first page hardwareConcurrency = %d, want 12 (first-page protection failed)", hw)
	}

	// M5: open a SECOND page directly on the browser. The targetcreated
	// re-injection is asynchronous, so retry across fresh navigations until the
	// injected script is registered (or time out).
	p2, err := sess.Browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		t.Fatalf("open second page: %v", err)
	}

	deadline := time.Now().Add(20 * time.Second)
	last := -1
	for time.Now().Before(deadline) {
		navigate(t, p2)
		last = evalInt(t, p2, "() => navigator.hardwareConcurrency")
		if last == 12 {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if last != 12 {
		t.Fatalf("second page hardwareConcurrency = %d, want 12 (M5 re-injection never reached the new tab)", last)
	}

	// Sanity: the platform sentinel is present on the new tab too.
	if plat := evalStr(t, p2, "() => navigator.platform"); plat != "MacIntel" {
		t.Fatalf("second page navigator.platform = %q, want MacIntel", plat)
	}
}

// navigate loads a fresh distinct document so EvalOnNewDocument scripts execute.
func navigate(t *testing.T, p *rod.Page) {
	t.Helper()
	if err := p.Navigate("data:text/html,<html><body>ok</body></html>"); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if err := p.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}
}

func evalInt(t *testing.T, p *rod.Page, js string) int {
	t.Helper()
	obj, err := p.Eval(js)
	if err != nil {
		t.Fatalf("eval %q: %v", js, err)
	}
	return obj.Value.Int()
}

func evalStr(t *testing.T, p *rod.Page, js string) string {
	t.Helper()
	obj, err := p.Eval(js)
	if err != nil {
		t.Fatalf("eval %q: %v", js, err)
	}
	return obj.Value.Str()
}

// TestPatchPageScriptModes verifies that patchPageScript dispatches to the
// mode-aware WebRTC builder and falls back to the v1.0 "fake" default. This is
// a Chrome-free test: it only inspects the generated script string.
func TestPatchPageScriptModes(t *testing.T) {
	base := patchPageScript(PatchPageOptions{})
	if !strings.Contains(base, fingerprint.WebRTCProtectionScript) {
		t.Errorf("default PatchPage should inject the fake WebRTC protection script")
	}
	if !strings.Contains(base, "Navigator spoofing enabled") {
		t.Errorf("default PatchPage should always inject the navigator override")
	}

	disable := patchPageScript(PatchPageOptions{Fingerprint: &FingerprintConfig{WebRTC: "disable"}})
	if strings.Contains(disable, fingerprint.WebRTCProtectionScript) {
		t.Errorf("webrtc=disable should not inject the fake WebRTC script")
	}
	if !strings.Contains(disable, fingerprint.WebRTCProtectionDisableScript) {
		t.Errorf("webrtc=disable should inject the disable script")
	}
	if !strings.Contains(disable, "Navigator spoofing enabled") {
		t.Errorf("webrtc=disable should still inject the navigator override")
	}

	real := patchPageScript(PatchPageOptions{Fingerprint: &FingerprintConfig{WebRTC: "real"}})
	if strings.Contains(real, fingerprint.WebRTCProtectionScript) {
		t.Errorf("webrtc=real should not inject the fake WebRTC script")
	}
	if strings.Contains(real, fingerprint.WebRTCProtectionDisableScript) {
		t.Errorf("webrtc=real should not inject the disable script")
	}
	if !strings.Contains(real, "Navigator spoofing enabled") {
		t.Errorf("webrtc=real should still inject the navigator override")
	}

	empty := patchPageScript(PatchPageOptions{Fingerprint: &FingerprintConfig{WebRTC: ""}})
	if !strings.Contains(empty, fingerprint.WebRTCProtectionScript) {
		t.Errorf("empty webrtc mode should fall back to the fake WebRTC protection script")
	}

	// WebRTC toggle off should override mode and emit nothing.
	off := patchPageScript(PatchPageOptions{WebRTC: new(bool), Fingerprint: &FingerprintConfig{WebRTC: "disable"}})
	if strings.Contains(off, "WebRTC") {
		t.Errorf("WebRTC toggle=false should prevent any WebRTC script from being emitted")
	}
}
