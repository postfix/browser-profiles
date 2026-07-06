package browserprofiles

import (
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
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
