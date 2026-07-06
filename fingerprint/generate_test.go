package fingerprint

import (
	"slices"
	"strings"
	"testing"
)

func TestGenerateWindows(t *testing.T) {
	for range 50 {
		fp := GenerateFingerprint(GenerateFingerprintOptions{Platform: "windows", Gpu: "nvidia", Screen: "desktop"})
		if fp.Platform != "Win32" {
			t.Fatalf("platform: %s", fp.Platform)
		}
		if fp.Vendor != "Google Inc." {
			t.Fatalf("vendor: %s", fp.Vendor)
		}
		if !slices.Contains(userAgents["windows"], fp.UserAgent) {
			t.Fatalf("userAgent not from windows table: %s", fp.UserAgent)
		}
		if !slices.Contains(webglRenderers["nvidia"], fp.WebGL.Renderer) {
			t.Fatalf("renderer not from nvidia table: %s", fp.WebGL.Renderer)
		}
		if fp.WebGL.Vendor != "Google Inc. (ANGLE)" {
			t.Fatalf("webgl vendor: %s", fp.WebGL.Vendor)
		}
		if !slices.Contains([]int{4, 6, 8, 12, 16}, fp.HardwareConcurrency) {
			t.Fatalf("cores: %d", fp.HardwareConcurrency)
		}
		if !slices.Contains([]int{4, 8, 16, 32}, fp.DeviceMemory) {
			t.Fatalf("memory: %d", fp.DeviceMemory)
		}
		if fp.Screen.ColorDepth != 24 || fp.Screen.PixelDepth != 24 || fp.Screen.DevicePixelRatio != 1 {
			t.Fatalf("screen defaults: %+v", fp.Screen)
		}
		if fp.Screen.AvailWidth != fp.Screen.Width || fp.Screen.AvailHeight != fp.Screen.Height-40 {
			t.Fatalf("avail dims: %+v", fp.Screen)
		}
		if fp.ClientHints.Platform != "Windows" || fp.ClientHints.Architecture != "x86" || fp.ClientHints.Mobile {
			t.Fatalf("clienthints: %+v", fp.ClientHints)
		}
		if !slices.Contains(platformVersions["windows"], fp.ClientHints.PlatformVersion) {
			t.Fatalf("platformVersion: %s", fp.ClientHints.PlatformVersion)
		}
		if len(fp.ClientHints.Brands) != 3 || fp.ClientHints.Brands[2].Version != "8" {
			t.Fatalf("brands: %+v", fp.ClientHints.Brands)
		}
		if !slices.Equal(fp.Languages, []string{"en-US", "en"}) {
			t.Fatalf("languages: %v", fp.Languages)
		}
	}
}

func TestGenerateMacos(t *testing.T) {
	for range 50 {
		fp := GenerateFingerprint(GenerateFingerprintOptions{Platform: "macos"})
		if fp.Platform != "MacIntel" {
			t.Fatalf("platform: %s", fp.Platform)
		}
		if !slices.Contains(webglRenderers["apple"], fp.WebGL.Renderer) {
			t.Fatalf("mac default gpu should be apple: %s", fp.WebGL.Renderer)
		}
		if fp.ClientHints.Architecture != "arm" {
			t.Fatalf("arch: %s", fp.ClientHints.Architecture)
		}
		if !slices.Contains([]int{8, 10, 12, 16}, fp.HardwareConcurrency) {
			t.Fatalf("mac cores: %d", fp.HardwareConcurrency)
		}
		if !slices.Contains([]int{8, 16, 32, 64}, fp.DeviceMemory) {
			t.Fatalf("mac memory: %d", fp.DeviceMemory)
		}
		if fp.Screen.AvailHeight != fp.Screen.Height-25 {
			t.Fatalf("mac availHeight offset should be 25: %+v", fp.Screen)
		}
	}
}

func TestGenerateRetinaAndVersionLanguage(t *testing.T) {
	for range 20 {
		fp := GenerateFingerprint(GenerateFingerprintOptions{Platform: "windows", Screen: "retina", Version: 120, Language: "ja-JP"})
		if fp.Screen.DevicePixelRatio != 2 {
			t.Fatalf("retina dpr should be 2: %d", fp.Screen.DevicePixelRatio)
		}
		found := false
		for _, r := range screenResolutions["retina"] {
			if r.Width == fp.Screen.Width && r.Height == fp.Screen.Height {
				found = true
			}
		}
		if !found {
			t.Fatalf("resolution not from retina table: %dx%d", fp.Screen.Width, fp.Screen.Height)
		}
		if fp.ClientHints.Brands[0].Version != "120" {
			t.Fatalf("version override: %s", fp.ClientHints.Brands[0].Version)
		}
		if fp.Language != "ja-JP" || !slices.Equal(fp.Languages, []string{"ja-JP", "ja"}) {
			t.Fatalf("language: %s %v", fp.Language, fp.Languages)
		}
	}
}

func TestGenerateOverrides(t *testing.T) {
	fp := GenerateFingerprint(GenerateFingerprintOptions{
		Platform:  "macos",
		Overrides: func(f *GeneratedFingerprint) { f.HardwareConcurrency = 128; f.DeviceMemory = 256 },
	})
	if fp.HardwareConcurrency != 128 || fp.DeviceMemory != 256 {
		t.Fatalf("overrides not applied: %d/%d", fp.HardwareConcurrency, fp.DeviceMemory)
	}
	if fp.Meta.Seed == "" || !strings.HasPrefix(fp.Meta.Seed, "fp-") {
		t.Fatalf("seed: %q", fp.Meta.Seed)
	}
}

// TestGeneratedFingerprintFeedsScripts confirms a generated fp produces a fully
// substituted script bundle (no unresolved %%markers%% leak through).
func TestGeneratedFingerprintFeedsScripts(t *testing.T) {
	fp := GenerateFingerprint(GenerateFingerprintOptions{Platform: "linux"})
	out := GetFingerprintScripts(fp)
	if strings.Contains(out, "%%") {
		t.Fatalf("unresolved marker in output")
	}
	if !strings.Contains(out, "Navigator spoofing enabled") || !strings.Contains(out, "Client Hints spoofing enabled") {
		t.Fatalf("expected builder output missing")
	}
}
