package fingerprint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		"consts/webrtc-disable.js":    WebRTCProtectionDisableScript,
		"consts/webrtc-fake.js":       WebRTCProtectionScript,
		"consts/webrtc-real.js":       WebRTCProtectionRealScript,
		"consts/canvas-noise.js":      CanvasProtectionScript,
		"consts/canvas-real.js":       CanvasProtectionRealScript,
		"consts/webgl.js":             WebGLProtectionScript,
		"consts/audio-noise.js":       AudioProtectionScript,
		"consts/audio-real.js":        AudioProtectionRealScript,
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
	assertEq(t, "all/modes",
		GetAllProtectionScripts(&AllProtectionOptions{WebRTCMode: "disable", CanvasMode: "real", AudioMode: "noise"}),
		readFixture(t, "all/modes.js"))
	assertEq(t, "all/nav_escape",
		GetAllProtectionScripts(&AllProtectionOptions{Navigator: &NavigatorConfig{Language: "ja-JP", Platform: "Mac<>&", HardwareConcurrency: 16, DeviceMemory: 32}}),
		readFixture(t, "all/nav_escape.js"))

	fixedFp := GeneratedFingerprint{
		UserAgent: "Mozilla/5.0 X", Platform: "Win32", Language: "en-US", Languages: []string{"en-US", "en"},
		HardwareConcurrency: 8, DeviceMemory: 8, Vendor: "Google Inc. (ANGLE)",
		AppVersion: "5.0 X", ProductSub: "20030107", MaxTouchPoints: 0, Mobile: false,
		Connection:  NavigatorConnection{EffectiveType: "4g", Downlink: 10, Rtt: 50, SaveData: false},
		Screen:      ScreenInfo{Width: 1920, Height: 1080, AvailWidth: 1920, AvailHeight: 1040, ColorDepth: 24, PixelDepth: 24, DevicePixelRatio: 1},
		WebGL:       WebGLInfo{Vendor: "Google Inc. (ANGLE)", Renderer: "ANGLE (Intel)"},
		ClientHints: ClientHintsInfo{Platform: "Windows", PlatformVersion: "10.0.0", Architecture: "x86", Mobile: false, Brands: []Brand{{Brand: "Chromium", Version: "120"}}, FullVersion: "120.0.0.0"},
		WebGPU:      DefaultWebGPUConfig("intel"),
		Timing:      DefaultTimingConfig(),
	}
	assertEq(t, "fpscripts/sample",
		GetFingerprintScripts(fixedFp),
		readFixture(t, "fpscripts/sample.js"))

	assertEq(t, "navigator/coherence",
		CreateNavigatorScript(NavigatorConfig{
			UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			Language:            "en-US",
			Platform:            "Win32",
			HardwareConcurrency: 8,
			DeviceMemory:        8,
			Vendor:              "Google Inc.",
			AppVersion:          "(Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			ProductSub:          "20030107",
			MaxTouchPoints:      0,
			Mobile:              false,
			Connection:          &NavigatorConnection{EffectiveType: "4g", Downlink: 10, Rtt: 50, SaveData: false},
		}),
		readFixture(t, "navigator/coherence.js"))

	assertEq(t, "clienthints/full_version",
		CreateClientHintsScript(ClientHintsScriptConfig{
			Platform: "Windows", PlatformVersion: "10.0.0", Architecture: "x86", Model: "", Mobile: false,
			Brands:      []Brand{{Brand: "Chromium", Version: "120"}, {Brand: "Google Chrome", Version: "120"}, {Brand: "Not_A Brand", Version: "8"}},
			FullVersion: "120.0.6099.130",
		}),
		readFixture(t, "clienthints/full_version.js"))

	// Escape case (closes security review B-1/M-1): every profile-controlled string
	// field is JSON-encoded via marshalNoEscape, so `'`, `"`, `\`, `<`, `>`, `&`, and a
	// literal `</script>` sequence can never break out of the generated JS string
	// literal. See TestClientHintsScriptNoInjection for the structural safety proof.
	assertEq(t, "clienthints/escape",
		CreateClientHintsScript(ClientHintsScriptConfig{
			Platform: `Plat'"</script>&<>`, PlatformVersion: `1.0'"`, Architecture: `arm'"`,
			Model: `Pixel'"`, FullVersion: `120.0'"`, Mobile: true,
		}),
		readFixture(t, "clienthints/escape.js"))

	assertEq(t, "all/with_clienthints",
		GetAllProtectionScripts(&AllProtectionOptions{
			Navigator:   &NavigatorConfig{Language: "en-US", Platform: "Win32", HardwareConcurrency: 8, DeviceMemory: 8},
			ClientHints: &ClientHintsScriptConfig{Platform: "Windows", PlatformVersion: "10.0.0", Architecture: "x86", Mobile: false, FullVersion: "120.0.6099.71"},
		}),
		readFixture(t, "all/with_clienthints.js"))

	assertEq(t, "webgl/caps",
		CreateWebGLScript(WebGLScriptConfig{
			Vendor:   "Google Inc. (NVIDIA)",
			Renderer: "ANGLE (NVIDIA, NVIDIA GeForce RTX 3080)",
			Caps: &WebGLCaps{
				MaxTextureSize: 32768, MaxCubeMapTextureSize: 32768, MaxRenderbufferSize: 32768,
				MaxVaryingVectors: 31, MaxVertexUniformVectors: 4096,
				MaxViewportDims: [2]int{32768, 32768},
				AliasedLineWidthRange: [2]float64{1, 1}, AliasedPointSizeRange: [2]float64{1, 2047},
				MaxTextureImageUnits: 32, MaxVertexTextureImageUnits: 32, MaxCombinedTextureImageUnits: 192,
				MaxFragmentUniformVectors: 1024, MaxVertexAttribs: 29,
			},
		}),
		readFixture(t, "webgl/caps.js"))

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

	// Permissions builder fixtures.
	assertEq(t, "permissions/default",
		CreatePermissionsScript(DefaultPermissionsConfig("windows")),
		readFixture(t, "permissions/default.js"))
	assertEq(t, "permissions/full",
		CreatePermissionsScript(PermissionsConfig{Camera: "denied", Microphone: "denied", Geolocation: "granted", Notifications: "prompt"}),
		readFixture(t, "permissions/full.js"))

	// Plugins builder fixtures: platform-specific plugin sets.
	assertEq(t, "plugins/win",
		CreatePluginsScript(DefaultPluginsConfig("windows")),
		readFixture(t, "plugins/win.js"))
	assertEq(t, "plugins/mac",
		CreatePluginsScript(DefaultPluginsConfig("macos")),
		readFixture(t, "plugins/mac.js"))
	assertEq(t, "plugins/linux",
		CreatePluginsScript(DefaultPluginsConfig("linux")),
		readFixture(t, "plugins/linux.js"))

	// Fonts builder fixtures: per-OS whitelists.
	assertEq(t, "fonts/win",
		CreateFontsScript(DefaultFontsConfig("windows")),
		readFixture(t, "fonts/win.js"))
	assertEq(t, "fonts/mac",
		CreateFontsScript(DefaultFontsConfig("macos")),
		readFixture(t, "fonts/mac.js"))
	assertEq(t, "fonts/linux",
		CreateFontsScript(DefaultFontsConfig("linux")),
		readFixture(t, "fonts/linux.js"))

	// Combined launch bundle with all three new surfaces.
	assertEq(t, "all/with_permissions_plugins_fonts",
		GetAllProtectionScripts(&AllProtectionOptions{
			Navigator:   &NavigatorConfig{Language: "en-US", Platform: "Win32", HardwareConcurrency: 8, DeviceMemory: 8},
			ClientHints: &ClientHintsScriptConfig{Platform: "Windows", PlatformVersion: "10.0.0", Architecture: "x86", Mobile: false, FullVersion: "120.0.6099.71"},
			Permissions: &PermissionsConfig{Camera: "prompt", Microphone: "prompt", Geolocation: "prompt", Notifications: "default"},
			Plugins:     &PluginsConfig{Plugins: DefaultPluginsConfig("windows").Plugins, MimeTypes: DefaultPluginsConfig("windows").MimeTypes},
			Fonts:       &FontsConfig{Whitelist: DefaultFontsConfig("windows").Whitelist},
		}),
		readFixture(t, "all/with_permissions_plugins_fonts.js"))

	assertEq(t, "webgpu/default",
		CreateWebGPUScript(DefaultWebGPUConfig("")),
		readFixture(t, "webgpu/default.js"))
	assertEq(t, "webgpu/nvidia",
		CreateWebGPUScript(DefaultWebGPUConfig("nvidia")),
		readFixture(t, "webgpu/nvidia.js"))
	assertEq(t, "timing/enabled",
		CreateTimingScript(TimingConfig{Enabled: true, Precision: 1 * time.Millisecond}),
		readFixture(t, "timing/enabled.js"))
	assertEq(t, "timing/disabled",
		CreateTimingScript(TimingConfig{Enabled: false}),
		readFixture(t, "timing/disabled.js"))
	assertEq(t, "all/with_webgpu_timing",
		GetAllProtectionScripts(&AllProtectionOptions{
			Navigator:   &NavigatorConfig{Language: "en-US", Platform: "Win32", HardwareConcurrency: 8, DeviceMemory: 8},
			WebRTCMode:  "fake",
			CanvasMode:  "noise",
			AudioMode:   "noise",
			WebGPU:      &WebGPUConfig{Vendor: "intel", Architecture: "x86", Device: "Intel(R) UHD Graphics 630", Description: "Intel UHD Graphics 630"},
			Timing:      &TimingConfig{Enabled: true, Precision: 1 * time.Millisecond},
			Permissions: &PermissionsConfig{Camera: "prompt", Microphone: "prompt", Geolocation: "prompt", Notifications: "default"},
			Plugins:     &PluginsConfig{Plugins: DefaultPluginsConfig("windows").Plugins, MimeTypes: DefaultPluginsConfig("windows").MimeTypes},
			Fonts:       &FontsConfig{Whitelist: DefaultFontsConfig("windows").Whitelist},
		}),
		readFixture(t, "all/with_webgpu_timing.js"))
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

// TestModeAwareBuilders verifies the mode-aware builders dispatch to the correct
// script variant and fall back to v1.0 defaults for empty or unrecognized modes.
func TestModeAwareBuilders(t *testing.T) {
	assertMode := func(t *testing.T, name, got, want string) {
		t.Helper()
		if got != want {
			t.Errorf("%s: mismatch (got %d bytes, want %d)", name, len(got), len(want))
		}
	}

	// WebRTC modes.
	assertMode(t, "webrtc/disable", CreateWebRTCProtectionScript("disable"), WebRTCProtectionDisableScript)
	assertMode(t, "webrtc/fake", CreateWebRTCProtectionScript("fake"), WebRTCProtectionScript)
	assertMode(t, "webrtc/real", CreateWebRTCProtectionScript("real"), "")
	assertMode(t, "webrtc/empty", CreateWebRTCProtectionScript(""), WebRTCProtectionScript)
	assertMode(t, "webrtc/unknown", CreateWebRTCProtectionScript("nonsense"), WebRTCProtectionScript)
	assertMode(t, "webrtc/case", CreateWebRTCProtectionScript("DISABLE"), WebRTCProtectionDisableScript)

	// Canvas modes.
	assertMode(t, "canvas/noise", CreateCanvasProtectionScript("noise"), CanvasProtectionScript)
	assertMode(t, "canvas/real", CreateCanvasProtectionScript("real"), "")
	assertMode(t, "canvas/empty", CreateCanvasProtectionScript(""), CanvasProtectionScript)
	assertMode(t, "canvas/unknown", CreateCanvasProtectionScript("nonsense"), CanvasProtectionScript)
	assertMode(t, "canvas/case", CreateCanvasProtectionScript("REAL"), "")

	// Audio modes.
	assertMode(t, "audio/noise", CreateAudioProtectionScript("noise"), AudioProtectionScript)
	assertMode(t, "audio/real", CreateAudioProtectionScript("real"), "")
	assertMode(t, "audio/empty", CreateAudioProtectionScript(""), AudioProtectionScript)
	assertMode(t, "audio/unknown", CreateAudioProtectionScript("nonsense"), AudioProtectionScript)
	assertMode(t, "audio/case", CreateAudioProtectionScript("NOISE"), AudioProtectionScript)

	// The *bool toggle still overrides the mode string: disabled surfaces emit nothing.
	allDisabled := GetAllProtectionScripts(&AllProtectionOptions{
		WebRTC: new(false), Canvas: new(false), WebGL: new(false), Audio: new(false),
		WebRTCMode: "disable", CanvasMode: "noise", AudioMode: "noise",
	})
	if strings.Contains(allDisabled, "WebRTC") || strings.Contains(allDisabled, "Canvas") || strings.Contains(allDisabled, "Audio") {
		t.Errorf("disabled surfaces should not emit protection scripts")
	}

	// Real modes omit the surface from the combined bundle while leaving others intact.
	allReal := GetAllProtectionScripts(&AllProtectionOptions{WebRTCMode: "real", CanvasMode: "real", AudioMode: "real"})
	if strings.Contains(allReal, WebRTCProtectionScript) {
		t.Errorf("webrtc=real should not emit the fake WebRTC script")
	}
	if strings.Contains(allReal, CanvasProtectionScript) {
		t.Errorf("canvas=real should not emit the noise canvas script")
	}
	if strings.Contains(allReal, AudioProtectionScript) {
		t.Errorf("audio=real should not emit the noise audio script")
	}
	if !strings.Contains(allReal, WebGLProtectionScript) {
		t.Errorf("real modes should still leave WebGL protection in place")
	}
}

// TestPermissionsBuilder verifies the permissions script contains the configured
// states and preserves the async navigator.permissions.query signature.
func TestPermissionsBuilder(t *testing.T) {
	script := CreatePermissionsScript(PermissionsConfig{
		Camera: "denied", Microphone: "prompt", Geolocation: "granted", Notifications: "default",
	})
	for _, want := range []string{"camera", "microphone", "geolocation", "notifications", "denied", "granted", "navigator.permissions.query"} {
		if !strings.Contains(script, want) {
			t.Errorf("permissions script missing %q", want)
		}
	}
	// Empty config should still produce a script that falls back to the real query.
	empty := CreatePermissionsScript(PermissionsConfig{})
	if !strings.Contains(empty, "navigator.permissions.query") {
		t.Errorf("empty permissions script should still override navigator.permissions.query")
	}
}

// TestPluginsBuilder verifies the plugins script builds array-like objects with
// the expected methods and platform-specific entries.
func TestPluginsBuilder(t *testing.T) {
	win := CreatePluginsScript(DefaultPluginsConfig("windows"))
	mac := CreatePluginsScript(DefaultPluginsConfig("macos"))
	linux := CreatePluginsScript(DefaultPluginsConfig("linux"))

	for _, script := range []string{win, mac, linux} {
		for _, want := range []string{"'plugins'", "'mimeTypes'", "item", "namedItem", "Chrome PDF Plugin", "application/pdf"} {
			if !strings.Contains(script, want) {
				t.Errorf("plugins script missing %q", want)
			}
		}
	}
	if !strings.Contains(win, "Native Client") {
		t.Errorf("windows plugin list should include Native Client")
	}
	if strings.Contains(mac, "Native Client") {
		t.Errorf("macOS plugin list should not include Native Client")
	}
	if strings.Contains(linux, "Native Client") {
		t.Errorf("linux plugin list should not include Native Client")
	}
	if !strings.Contains(win, "refresh") {
		t.Errorf("navigator.plugins should expose refresh()")
	}
}

// TestFontsBuilder verifies the fonts guard contains the whitelist and falls back
// to the real document.fonts.check for non-whitelisted fonts.
func TestFontsBuilder(t *testing.T) {
	win := CreateFontsScript(DefaultFontsConfig("windows"))
	if !strings.Contains(win, "Arial") {
		t.Errorf("windows font whitelist should include Arial")
	}
	if !strings.Contains(win, "document.fonts.check") {
		t.Errorf("fonts script should override document.fonts.check")
	}
	if !strings.Contains(win, "realCheck") {
		t.Errorf("fonts script should capture the real check for fallback")
	}

	empty := CreateFontsScript(FontsConfig{})
	if !strings.Contains(empty, "document.fonts.check") {
		t.Errorf("empty font whitelist script should still override document.fonts.check")
	}
}

// TestAllProtectionIncludesNewScripts verifies that the three new scripts are
// emitted in the expected order between client hints and the automation bypass.
func TestAllProtectionIncludesNewScripts(t *testing.T) {
	all := GetAllProtectionScripts(&AllProtectionOptions{
		Navigator:   &NavigatorConfig{Language: "en-US", Platform: "Win32", HardwareConcurrency: 8, DeviceMemory: 8},
		ClientHints: &ClientHintsScriptConfig{Platform: "Windows"},
		Permissions: &PermissionsConfig{Camera: "denied"},
		Plugins:     &PluginsConfig{Plugins: DefaultPluginsConfig("windows").Plugins, MimeTypes: DefaultPluginsConfig("windows").MimeTypes},
		Fonts:       &FontsConfig{Whitelist: DefaultFontsConfig("windows").Whitelist},
	})

	idxNav := strings.Index(all, "Navigator spoofing enabled")
	idxCH := strings.Index(all, "Client Hints spoofing enabled")
	idxPerm := strings.Index(all, "Permissions spoofing enabled")
	idxPlugins := strings.Index(all, "Plugins spoofing enabled")
	idxFonts := strings.Index(all, "Fonts guard enabled")
	idxBypass := strings.Index(all, "Automation bypass enabled")

	for name, idx := range map[string]int{
		"navigator": idxNav, "client-hints": idxCH, "permissions": idxPerm,
		"plugins": idxPlugins, "fonts": idxFonts, "automation-bypass": idxBypass,
	} {
		if idx < 0 {
			t.Fatalf("missing %s marker", name)
		}
	}

	if !(idxNav < idxCH && idxCH < idxPerm && idxPerm < idxPlugins && idxPlugins < idxFonts && idxFonts < idxBypass) {
		t.Errorf("unexpected script order: nav=%d ch=%d perm=%d plugins=%d fonts=%d bypass=%d",
			idxNav, idxCH, idxPerm, idxPlugins, idxFonts, idxBypass)
	}
}

// TestAllProtectionIncludesWebGPUTiming verifies that WebGPU and Timing scripts are
// emitted in the expected order relative to the other protections.
func TestAllProtectionIncludesWebGPUTiming(t *testing.T) {
	all := GetAllProtectionScripts(
		&AllProtectionOptions{
			Navigator:   &NavigatorConfig{Language: "en-US", Platform: "Win32", HardwareConcurrency: 8, DeviceMemory: 8},
			ClientHints: &ClientHintsScriptConfig{Platform: "Windows"},
			WebGPU:      &WebGPUConfig{Vendor: "nvidia", Architecture: "x86", Device: "NVIDIA GeForce RTX 3080", Description: "NVIDIA GeForce RTX 3080"},
			Timing:      &TimingConfig{Enabled: true, Precision: 1 * time.Millisecond},
			Permissions: &PermissionsConfig{Camera: "denied"},
			Plugins:     &PluginsConfig{Plugins: DefaultPluginsConfig("windows").Plugins, MimeTypes: DefaultPluginsConfig("windows").MimeTypes},
			Fonts:       &FontsConfig{Whitelist: DefaultFontsConfig("windows").Whitelist},
		})

	idxWebRTC := strings.Index(all, "WebRTC protection enabled")
	idxCanvas := strings.Index(all, "Canvas protection enabled")
	idxWebGL := strings.Index(all, "WebGL protection enabled")
	idxAudio := strings.Index(all, "Audio protection enabled")
	idxWebGPU := strings.Index(all, "WebGPU protection enabled")
	idxTiming := strings.Index(all, "Timing spoofing enabled")
	idxNav := strings.Index(all, "Navigator spoofing enabled")
	idxCH := strings.Index(all, "Client Hints spoofing enabled")
	idxPerm := strings.Index(all, "Permissions spoofing enabled")
	idxPlugins := strings.Index(all, "Plugins spoofing enabled")
	idxFonts := strings.Index(all, "Fonts guard enabled")
	idxBypass := strings.Index(all, "Automation bypass enabled")

	for name, idx := range map[string]int{
		"webrtc": idxWebRTC, "canvas": idxCanvas, "webgl": idxWebGL, "audio": idxAudio,
		"webgpu": idxWebGPU, "timing": idxTiming, "navigator": idxNav, "client-hints": idxCH,
		"permissions": idxPerm, "plugins": idxPlugins, "fonts": idxFonts, "automation-bypass": idxBypass,
	} {
		if idx < 0 {
			t.Fatalf("missing %s marker", name)
		}
	}

	if !(idxWebRTC < idxCanvas && idxCanvas < idxWebGL && idxWebGL < idxAudio &&
		idxAudio < idxWebGPU && idxWebGPU < idxTiming &&
		idxTiming < idxNav && idxNav < idxCH && idxCH < idxPerm &&
		idxPerm < idxPlugins && idxPlugins < idxFonts && idxFonts < idxBypass) {
		t.Errorf("unexpected script order: webrtc=%d canvas=%d webgl=%d audio=%d webgpu=%d timing=%d nav=%d ch=%d perm=%d plugins=%d fonts=%d bypass=%d",
			idxWebRTC, idxCanvas, idxWebGL, idxAudio, idxWebGPU, idxTiming, idxNav, idxCH, idxPerm, idxPlugins, idxFonts, idxBypass)
	}
}

// defaults differ in the expected ways.
func TestDefaultConfigsByPlatform(t *testing.T) {
	if len(DefaultPluginsConfig("windows").Plugins) <= len(DefaultPluginsConfig("macos").Plugins) {
		t.Errorf("windows plugin list should be longer than macOS due to Native Client")
	}
	if len(DefaultFontsConfig("windows").Whitelist) == 0 {
		t.Errorf("windows font whitelist should not be empty")
	}
	if len(DefaultFontsConfig("macos").Whitelist) == 0 {
		t.Errorf("macOS font whitelist should not be empty")
	}
	if len(DefaultFontsConfig("linux").Whitelist) == 0 {
		t.Errorf("linux font whitelist should not be empty")
	}
	if DefaultPermissionsConfig("windows").Camera != "prompt" {
		t.Errorf("unexpected default camera permission: %s", DefaultPermissionsConfig("windows").Camera)
	}
}

// TestWebGPUBuilder verifies that the WebGPU builder emits the configured adapter
// info fields and preserves the requestAdapter promise shape.
func TestWebGPUBuilder(t *testing.T) {
	cfg := DefaultWebGPUConfig("nvidia")
	script := CreateWebGPUScript(cfg)
	for _, want := range []string{"\"vendor\":\"nvidia\"", "\"architecture\":\"x86\"", "NVIDIA GeForce RTX 3080", "requestAdapter", "requestAdapterInfo"} {
		if !strings.Contains(script, want) {
			t.Errorf("webgpu script missing %q", want)
		}
	}
	if !strings.Contains(script, "Promise.resolve") {
		t.Errorf("webgpu requestAdapter should return a Promise")
	}

	intel := DefaultWebGPUConfig("")
	if intel.Vendor != "intel" || intel.Device != "Intel(R) UHD Graphics 630" {
		t.Errorf("empty GPU family should fall back to intel defaults, got %+v", intel)
	}
	apple := DefaultWebGPUConfig("apple")
	if apple.Vendor != "apple" || apple.Architecture != "apple" {
		t.Errorf("apple GPU family mismatch, got %+v", apple)
	}
}

// TestTimingBuilder verifies that the enabled timing script contains the expected
// overrides and that the disabled script short-circuits before applying them.
func TestTimingBuilder(t *testing.T) {
	enabled := CreateTimingScript(TimingConfig{Enabled: true, Precision: 100 * time.Microsecond})
	for _, want := range []string{"performance.now =", "Date.now =", "SpoofedDate", `"precisionMs":0.1`} {
		if !strings.Contains(enabled, want) {
			t.Errorf("enabled timing script missing %q", want)
		}
	}
	if !strings.Contains(enabled, "Timing spoofing enabled") {
		t.Errorf("enabled timing script should log that it is enabled")
	}

	disabled := CreateTimingScript(TimingConfig{Enabled: false})
	if !strings.Contains(disabled, "Timing spoofing disabled") {
		t.Errorf("disabled timing script should log that it is disabled")
	}
	if !strings.Contains(disabled, "if (!cfg.enabled)") {
		t.Errorf("disabled timing script should contain an early return guard")
	}

	oneMs := CreateTimingScript(TimingConfig{Enabled: true, Precision: 1 * time.Millisecond})
	if !strings.Contains(oneMs, `"precisionMs":1`) {
		t.Errorf("1ms precision should serialize as 1.0, got %q", oneMs)
	}
}

// TestClientHintsScriptNoInjection is the regression guard for security review
// finding B-1: CreateClientHintsScript used to substitute Platform/PlatformVersion/
// Architecture/Model/FullVersion RAW inside single-quoted JS string literals in
// clienthints.tmpl.js, so a crafted string containing `'` (or `\`) could break out
// of the literal and execute arbitrary JavaScript in every page the profile
// launches (empirically proven in .planning/SECURITY-REVIEW-v1.1.md via a counter-
// increment PoC). The fix routes every field through marshalNoEscape, exactly like
// CreateWebGLScript's vendor/renderer. This test proves the fix holds structurally,
// without depending on a JS runtime being available in CI:
//  1. the JSON-encoded form of the payload (what marshalNoEscape actually produces)
//     is present in the output — i.e. the field IS going through JSON encoding;
//  2. the dangerous substring never appears anywhere OUTSIDE that JSON-encoded
//     literal — i.e. it never escapes into bare, executable JS;
//  3. the exact pre-fix vulnerable shape (`'<payload>'`, manual quotes around a raw
//     value) is absent, so the specific CVE pattern cannot silently regress;
//  4. the JSON-encoded literal parses back to byte-identical original content, so
//     the escaping is round-trip-safe, not merely non-crashing.
func TestClientHintsScriptNoInjection(t *testing.T) {
	const malicious = `x'); alert(1); //<script>&<>\`

	script := CreateClientHintsScript(ClientHintsScriptConfig{
		Platform:        malicious,
		PlatformVersion: malicious,
		Architecture:    malicious,
		Model:           malicious,
		FullVersion:     malicious,
	})

	encoded := marshalNoEscape(malicious)
	if !strings.Contains(script, encoded) {
		t.Fatalf("expected JSON-encoded payload %s to appear in generated script:\n%s", encoded, script)
	}

	// Every occurrence of the dangerous payload must live inside encoded (JSON
	// string) form. Stripping all such occurrences must remove every trace of the
	// attacker-controlled "alert(1)" — if any remained, it escaped its literal.
	stripped := strings.ReplaceAll(script, encoded, "")
	if strings.Contains(stripped, "alert(1)") {
		t.Fatalf("payload leaked outside its JSON-encoded literal — injection not closed:\n%s", script)
	}

	// The specific pre-fix vulnerable pattern (manual single quotes around a raw,
	// unescaped value) must never reappear.
	if brokenPattern := "'" + malicious + "'"; strings.Contains(script, brokenPattern) {
		t.Fatalf("regression: raw payload interpolated inside single-quoted JS literal (CVE B-1 pattern)")
	}
	if strings.Contains(script, "alert(1); //<script>&<>\\'") {
		t.Fatalf("regression: payload broke out of a string literal via a bare trailing quote")
	}

	// The JSON-encoded literal must round-trip to the exact original payload —
	// proving the escaping is correct, not just non-crashing.
	var got string
	if err := json.Unmarshal([]byte(encoded), &got); err != nil {
		t.Fatalf("marshalNoEscape output is not valid JSON: %v", err)
	}
	if got != malicious {
		t.Fatalf("payload round-trip mismatch: got %q want %q", got, malicious)
	}
}
