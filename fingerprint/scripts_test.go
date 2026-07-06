package fingerprint

import (
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", rel))
	if err != nil {
		t.Fatalf("read fixture %s: %v", rel, err)
	}
	return string(b)
}

// TestConstsMatchReference guards the embedded protection scripts against drift.
func TestConstsMatchReference(t *testing.T) {
	cases := map[string]string{
		"consts/webrtc.js":            WebRTCProtectionScript,
		"consts/canvas.js":            CanvasProtectionScript,
		"consts/webgl.js":             WebGLProtectionScript,
		"consts/audio.js":             AudioProtectionScript,
		"consts/automation_bypass.js": AutomationBypassScript,
	}
	for f, got := range cases {
		if want := readFixture(t, f); got != want {
			t.Errorf("%s: embedded const diverges from reference (got %d bytes, want %d)", f, len(got), len(want))
		}
	}
}

// TestBuildersGolden verifies each builder is byte-identical to the TS reference output
// for the exact inputs captured in testdata (includes an HTML-escape case, M6).
func TestBuildersGolden(t *testing.T) {
	assertEq := func(t *testing.T, name, got, want string) {
		t.Helper()
		if got != want {
			// find first differing byte for a precise diagnostic
			n := len(got)
			if len(want) < n {
				n = len(want)
			}
			at := n
			for i := range n {
				if got[i] != want[i] {
					at = i
					break
				}
			}
			lo := at - 40
			if lo < 0 {
				lo = 0
			}
			t.Errorf("%s: mismatch at byte %d (got %d bytes, want %d)\n got: %q\nwant: %q",
				name, at, len(got), len(want), safeSlice(got, lo, at+40), safeSlice(want, lo, at+40))
		}
	}

	assertEq(t, "navigator/launch",
		CreateNavigatorScript(NavigatorConfig{Language: "en-US", Platform: "Win32", HardwareConcurrency: 8, DeviceMemory: 8}),
		readFixture(t, "navigator/launch.js"))
	assertEq(t, "navigator/full_escape",
		CreateNavigatorScript(NavigatorConfig{UserAgent: `Mozilla/5.0 <b>&"x"</b>`, Language: "ja-JP", Platform: "MacIntel", HardwareConcurrency: 12, DeviceMemory: 16, Vendor: "Google Inc."}),
		readFixture(t, "navigator/full_escape.js"))
	assertEq(t, "navigator/empty",
		CreateNavigatorScript(NavigatorConfig{}),
		readFixture(t, "navigator/empty.js"))

	assertEq(t, "screen/default",
		CreateScreenScript(ScreenScriptConfig{}),
		readFixture(t, "screen/default.js"))
	assertEq(t, "screen/custom",
		CreateScreenScript(ScreenScriptConfig{Width: 2560, Height: 1440}),
		readFixture(t, "screen/custom.js"))

	assertEq(t, "clienthints/default",
		CreateClientHintsScript(ClientHintsScriptConfig{}),
		readFixture(t, "clienthints/default.js"))
	assertEq(t, "clienthints/full",
		CreateClientHintsScript(ClientHintsScriptConfig{
			Platform: "macOS", PlatformVersion: "14.2.0", Architecture: "arm", Model: "", Mobile: false,
			Brands: []Brand{{Brand: "Chromium", Version: "120"}, {Brand: "Google Chrome", Version: "120"}},
		}),
		readFixture(t, "clienthints/full.js"))

	assertEq(t, "all/default",
		GetAllProtectionScripts(nil),
		readFixture(t, "all/default.js"))
	assertEq(t, "all/launch_nav",
		GetAllProtectionScripts(&AllProtectionOptions{Navigator: &NavigatorConfig{Language: "en-US", Platform: "Win32", HardwareConcurrency: 8, DeviceMemory: 8}}),
		readFixture(t, "all/launch_nav.js"))
	assertEq(t, "all/toggles",
		GetAllProtectionScripts(&AllProtectionOptions{WebRTC: new(true), Canvas: new(false), WebGL: new(true), Audio: new(false)}),
		readFixture(t, "all/toggles.js"))
	assertEq(t, "all/nav_escape",
		GetAllProtectionScripts(&AllProtectionOptions{Navigator: &NavigatorConfig{Language: "ja-JP", Platform: "Mac<>&", HardwareConcurrency: 16, DeviceMemory: 32}}),
		readFixture(t, "all/nav_escape.js"))

	fixedFp := GeneratedFingerprint{
		UserAgent: "Mozilla/5.0 X", Platform: "Win32", Language: "en-US", Languages: []string{"en-US", "en"},
		HardwareConcurrency: 8, DeviceMemory: 8, Vendor: "Google Inc. (ANGLE)",
		Screen:      ScreenInfo{Width: 1920, Height: 1080, AvailWidth: 1920, AvailHeight: 1040, ColorDepth: 24, PixelDepth: 24, DevicePixelRatio: 1},
		WebGL:       WebGLInfo{Vendor: "Google Inc. (ANGLE)", Renderer: "ANGLE (Intel)"},
		ClientHints: ClientHintsInfo{Platform: "Windows", PlatformVersion: "10.0.0", Architecture: "x86", Mobile: false, Brands: []Brand{{Brand: "Chromium", Version: "120"}}},
	}
	assertEq(t, "fpscripts/sample",
		GetFingerprintScripts(fixedFp),
		readFixture(t, "fpscripts/sample.js"))

	// CreateWebGLScript: per-profile UNMASKED vendor/renderer are substituted into
	// the template verbatim; the fixture IS the builder output, locking future drift.
	assertEq(t, "webgl/custom",
		CreateWebGLScript(WebGLScriptConfig{Vendor: "Acme Inc.", Renderer: "AcmeGPU-Model-A-9999"}),
		readFixture(t, "webgl/custom.js"))
	// Escape case: `< > &` MUST stay literal (JSON.stringify parity, no HTML escaping)
	// while `"` and `\` MUST be JSON-escaped — a json.Marshal regression (which HTML-
	// escapes to \u003c) would redden this.
	assertEq(t, "webgl/escape",
		CreateWebGLScript(WebGLScriptConfig{Vendor: `Ac<me>&"Co"`, Renderer: `GPU <x> & "y" \z/`}),
		readFixture(t, "webgl/escape.js"))
}

func safeSlice(s string, lo, hi int) string {
	if lo < 0 {
		lo = 0
	}
	if hi > len(s) {
		hi = len(s)
	}
	if lo > hi {
		lo = hi
	}
	return s[lo:hi]
}
