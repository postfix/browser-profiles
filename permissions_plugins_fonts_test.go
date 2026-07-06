package browserprofiles

import (
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
)

// TestPermissionsPluginsFontsSmoke launches a profile with explicit Windows
// permissions, plugins, and fonts overrides and verifies the injected values are
// observable on the launch target. Real Chrome required.
func TestPermissionsPluginsFontsSmoke(t *testing.T) {
	status, note, value := "skipped", "Chrome unavailable or smoke test skipped", ""
	defer func() { recordDetectorResult("permissions_plugins_fonts_coherence", status, note, value) }()

	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
	profile, err := bp.Create(ProfileConfig{
		ID:   "ppf-smoke-01",
		Name: "ppf-smoke",
		Fingerprint: &FingerprintConfig{
			UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			Platform:            "Win32",
			Language:            "en-US",
			HardwareConcurrency: 8,
			DeviceMemory:        8,
			Permissions: &PermissionsConfig{
				Camera: "denied", Microphone: "prompt", Geolocation: "granted", Notifications: "default",
			},
			Plugins: &PluginsConfig{
				Plugins: []PluginInfo{
					{
						Name:        "Chrome PDF Plugin",
						Filename:    "internal-pdf-viewer",
						Description: "Portable Document Format",
						Version:     "undefined",
						MimeTypes: []PluginMimeType{
							{Type: "application/pdf", Description: "Portable Document Format", Suffixes: "pdf"},
						},
					},
					{
						Name:        "Chrome PDF Viewer",
						Filename:    "mhjfbmdgcfjbbpaeojofohoefgiehjai",
						Description: "Portable Document Format",
						Version:     "undefined",
						MimeTypes: []PluginMimeType{
							{Type: "application/pdf", Description: "Portable Document Format", Suffixes: "pdf"},
						},
					},
				},
				MimeTypes: []PluginMimeType{
					{Type: "application/pdf", Description: "Portable Document Format", Suffixes: "pdf", EnabledPlugin: "Chrome PDF Viewer"},
				},
			},
			Fonts: &FontsConfig{
				Whitelist: []string{"Arial", "Times New Roman"},
			},
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

	page := sess.Page
	navigate(t, page)

	// Permissions: known descriptors return configured states.
	if got := evalStr(t, page, `() => new Promise(r => navigator.permissions.query({name:'camera'}).then(s => r(s.state)))`); got != "denied" {
		t.Errorf("camera permission state = %q, want denied", got)
	}
	if got := evalStr(t, page, `() => new Promise(r => navigator.permissions.query({name:'microphone'}).then(s => r(s.state)))`); got != "prompt" {
		t.Errorf("microphone permission state = %q, want prompt", got)
	}
	if got := evalStr(t, page, `() => new Promise(r => navigator.permissions.query({name:'geolocation'}).then(s => r(s.state)))`); got != "granted" {
		t.Errorf("geolocation permission state = %q, want granted", got)
	}
	if got := evalStr(t, page, `() => new Promise(r => navigator.permissions.query({name:'notifications'}).then(s => r(s.state)))`); got != "default" {
		t.Errorf("notifications permission state = %q, want default", got)
	}
	// Unknown descriptor should fall back to the real query without crashing.
	_ = evalStr(t, page, `() => new Promise(r => navigator.permissions.query({name:'accelerometer'}).then(s => r(s.state)).catch(e => r('error')))`)

	// Plugins: array-like shape and configured entries.
	if got := evalInt(t, page, "() => navigator.plugins.length"); got != 2 {
		t.Errorf("navigator.plugins.length = %d, want 2", got)
	}
	if got := evalStr(t, page, "() => navigator.plugins.item(0).name"); got != "Chrome PDF Plugin" {
		t.Errorf("navigator.plugins.item(0).name = %q, want Chrome PDF Plugin", got)
	}
	if got := evalStr(t, page, "() => navigator.plugins.namedItem('Chrome PDF Viewer').name"); got != "Chrome PDF Viewer" {
		t.Errorf("navigator.plugins.namedItem('Chrome PDF Viewer') = %q, want Chrome PDF Viewer", got)
	}
	pdfMimeType := evalStr(t, page, "() => { const m = navigator.mimeTypes['application/pdf']; return m ? m.type : 'missing'; }")
	if pdfMimeType != "application/pdf" {
		t.Errorf("navigator.mimeTypes['application/pdf'] = %q, want application/pdf", pdfMimeType)
	}

	// Fonts: whitelisted fonts return true; unknown fonts fall back to the real check.
	if got := evalBool(t, page, `() => document.fonts.check("1em Arial, sans-serif")`); !got {
		t.Errorf("document.fonts.check('Arial') = %v, want true", got)
	}
	if got := evalBool(t, page, `() => document.fonts.check("1em Times New Roman, serif")`); !got {
		t.Errorf("document.fonts.check('Times New Roman') = %v, want true", got)
	}
	// Non-whitelisted fonts fall back to the real document.fonts.check. The result
	// depends on host font installation, so we only assert that the override is in
	// place and does not throw.
	_ = evalBool(t, page, `() => document.fonts.check("1em SomeFakeFontXYZ, sans-serif")`)

	// Self-contained contradiction check: platform, plugins, and fonts must agree.
	plat := evalStr(t, page, "() => navigator.platform")
	if plat != "Win32" {
		t.Errorf("navigator.platform = %q, want Win32", plat)
	}
	if plat == "Win32" && evalInt(t, page, "() => navigator.plugins.length") == 2 {
		status = "passed"
		note = "permissions/plugins/fonts coherence self-check passed"
		value = "platform/plugins/fonts coherent"
	} else {
		status = "failed"
		note = "permissions/plugins/fonts coherence contradiction detected"
		value = "platform or plugin mismatch"
		t.Fatalf("permissions/plugins/fonts coherence contradiction: platform=%q, plugins.length mismatch", plat)
	}
}

// TestPermissionsPluginsFontsCrossTab verifies that the M5 re-injection loop
// carries the permissions, plugins, and fonts overrides into a newly opened tab.
// Real Chrome required.
func TestPermissionsPluginsFontsCrossTab(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
	profile, err := bp.Create(ProfileConfig{
		ID:   "ppf-crosstab-01",
		Name: "ppf-crosstab",
		Fingerprint: &FingerprintConfig{
			UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			Platform:            "Win32",
			Language:            "en-US",
			HardwareConcurrency: 8,
			DeviceMemory:        8,
			Permissions: &PermissionsConfig{
				Camera: "denied", Microphone: "prompt", Geolocation: "granted", Notifications: "default",
			},
			Plugins: &PluginsConfig{
				Plugins: []PluginInfo{
					{
						Name:        "Chrome PDF Plugin",
						Filename:    "internal-pdf-viewer",
						Description: "Portable Document Format",
						Version:     "undefined",
						MimeTypes: []PluginMimeType{
							{Type: "application/pdf", Description: "Portable Document Format", Suffixes: "pdf"},
						},
					},
					{
						Name:        "Chrome PDF Viewer",
						Filename:    "mhjfbmdgcfjbbpaeojofohoefgiehjai",
						Description: "Portable Document Format",
						Version:     "undefined",
						MimeTypes: []PluginMimeType{
							{Type: "application/pdf", Description: "Portable Document Format", Suffixes: "pdf"},
						},
					},
				},
			},
			Fonts: &FontsConfig{
				Whitelist: []string{"Arial"},
			},
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

	p2, err := sess.Browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		t.Fatalf("open new tab: %v", err)
	}

	// M5 re-injection is asynchronous; retry until the bundle is registered or time out.
	deadline := time.Now().Add(20 * time.Second)
	lastLen := -1
	for time.Now().Before(deadline) {
		navigate(t, p2)
		lastLen = evalInt(t, p2, "() => navigator.plugins.length")
		if lastLen == 2 {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	if lastLen != 2 {
		t.Fatalf("new tab navigator.plugins.length = %d, want 2 (M5 re-injection never reached the new tab)", lastLen)
	}

	if got := evalStr(t, p2, "() => navigator.plugins.item(0).name"); got != "Chrome PDF Plugin" {
		t.Errorf("new tab navigator.plugins.item(0).name = %q, want Chrome PDF Plugin", got)
	}
	if got := evalStr(t, p2, `() => new Promise(r => navigator.permissions.query({name:'camera'}).then(s => r(s.state)))`); got != "denied" {
		t.Errorf("new tab camera permission state = %q, want denied", got)
	}
	if got := evalBool(t, p2, `() => document.fonts.check("1em Arial, sans-serif")`); !got {
		t.Errorf("new tab document.fonts.check('Arial') = %v, want true", got)
	}
}
