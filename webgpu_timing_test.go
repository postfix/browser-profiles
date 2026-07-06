package browserprofiles

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// TestWebGPUSmoke verifies that navigator.gpu.requestAdapter returns the mocked
// adapter info. It skips when the browser does not expose WebGPU at all.
func TestWebGPUSmoke(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
	profile, err := bp.Create(ProfileConfig{
		ID:   "wgpu-01",
		Name: "wgpu",
		Fingerprint: &FingerprintConfig{
			UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			Platform:   "Win32",
			Language:   "en-US",
			WebGL:      &WebGLConfig{Vendor: "Google Inc. (NVIDIA)", Renderer: "ANGLE (NVIDIA, NVIDIA GeForce RTX 3080)"},
			WebGPU:     &WebGPUConfig{Vendor: "nvidia", Architecture: "x86", Device: "NVIDIA GeForce RTX 3080", Description: "NVIDIA GeForce RTX 3080"},
			ClientHints: &ClientHintsConfig{Platform: "Windows", PlatformVersion: "10.0.0", Architecture: "x86", Mobile: false},
		},
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	sess, err := WithProfile(bp, profile.ID, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("WithProfile: %v", err)
	}
	defer func() { _ = sess.Terminate() }()

	page := sess.Page
	navigate(t, page)

	if got := evalStr(t, page, "() => typeof navigator.gpu"); got == "undefined" {
		t.Skip("browser does not expose navigator.gpu; skipping WebGPU smoke test")
	}

	infoJSON := evalStr(t, page, `() => new Promise(async r => {
		const adapter = await navigator.gpu.requestAdapter();
		r(JSON.stringify({vendor: adapter.info.vendor, architecture: adapter.info.architecture, device: adapter.info.device, description: adapter.info.description}));
	})`)
	var info map[string]any
	if err := json.Unmarshal([]byte(infoJSON), &info); err != nil {
		t.Fatalf("unmarshal adapter info: %v (raw: %q)", err, infoJSON)
	}
	if info["vendor"] != "nvidia" {
		t.Errorf("adapter.info.vendor = %q, want nvidia", info["vendor"])
	}
	if info["device"] != "NVIDIA GeForce RTX 3080" {
		t.Errorf("adapter.info.device = %q, want NVIDIA GeForce RTX 3080", info["device"])
	}
	if info["architecture"] != "x86" {
		t.Errorf("adapter.info.architecture = %q, want x86", info["architecture"])
	}
}

// TestTimingSmoke verifies that enabled timing spoofing rounds performance.now()
// and Date.now() to the configured precision while preserving monotonicity.
func TestTimingSmoke(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
	profile, err := bp.Create(ProfileConfig{
		ID:   "timing-01",
		Name: "timing",
		Fingerprint: &FingerprintConfig{
			UserAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			Platform:      "Win32",
			Language:      "en-US",
			Timing:        &TimingConfig{Enabled: true, Precision: 1 * time.Second},
			ClientHints:   &ClientHintsConfig{Platform: "Windows", PlatformVersion: "10.0.0", Architecture: "x86", Mobile: false},
		},
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	sess, err := WithProfile(bp, profile.ID, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("WithProfile: %v", err)
	}
	defer func() { _ = sess.Terminate() }()

	page := sess.Page
	navigate(t, page)

	perf := evalFloat(t, page, "() => performance.now()")
	if int64(perf)%1000 != 0 {
		t.Errorf("performance.now() = %f, want multiple of 1000ms", perf)
	}

	now := evalInt(t, page, "() => Date.now()")
	if now%1000 != 0 {
		t.Errorf("Date.now() = %d, want last three digits zero", now)
	}

	monotonic := evalBool(t, page, `() => {
		const a = performance.now();
		const b = performance.now();
		return b >= a;
	}`)
	if !monotonic {
		t.Errorf("performance.now() is not monotonic")
	}
}

// TestCPUThrottlingSmoke verifies that a positive CPUThrottlingRate launches
// successfully without breaking the injected anti-detect bundle.
func TestCPUThrottlingSmoke(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
	profile, err := bp.Create(ProfileConfig{
		ID:   "cpu-throttle-01",
		Name: "cpu-throttle",
		Fingerprint: &FingerprintConfig{
			UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			Platform:            "Win32",
			Language:            "en-US",
			HardwareConcurrency: 8,
			CPUThrottlingRate:   2,
			ClientHints:         &ClientHintsConfig{Platform: "Windows", PlatformVersion: "10.0.0", Architecture: "x86", Mobile: false},
		},
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	sess, err := WithProfile(bp, profile.ID, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("WithProfile: %v", err)
	}
	defer func() { _ = sess.Terminate() }()

	page := sess.Page
	navigate(t, page)

	if got := evalInt(t, page, "() => navigator.hardwareConcurrency"); got != 8 {
		t.Errorf("navigator.hardwareConcurrency = %d, want 8", got)
	}
}

// TestWebGPUTimingCrossTab verifies that the WebGPU and timing protections reach
// a newly opened tab via the M5 re-injection loop.
func TestWebGPUTimingCrossTab(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
	profile, err := bp.Create(ProfileConfig{
		ID:   "wgpu-timing-crosstab-01",
		Name: "wgpu-timing-crosstab",
		Fingerprint: &FingerprintConfig{
			UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			Platform:   "Win32",
			Language:   "en-US",
			WebGPU:    &WebGPUConfig{Vendor: "amd", Architecture: "x86", Device: "AMD Radeon RX 580", Description: "AMD Radeon RX 580"},
			Timing:    &TimingConfig{Enabled: true, Precision: 1 * time.Second},
			ClientHints: &ClientHintsConfig{Platform: "Windows", PlatformVersion: "10.0.0", Architecture: "x86", Mobile: false},
		},
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	sess, err := WithProfile(bp, profile.ID, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("WithProfile: %v", err)
	}
	defer func() { _ = sess.Terminate() }()

	p2, err := sess.Browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		t.Fatalf("open new tab: %v", err)
	}
	navigate(t, p2)

	if got := evalStr(t, p2, "() => typeof navigator.gpu"); got == "undefined" {
		t.Skip("browser does not expose navigator.gpu in new tab; skipping cross-tab WebGPU check")
	}

	infoJSON := evalStr(t, p2, `() => new Promise(async r => {
		const adapter = await navigator.gpu.requestAdapter();
		r(JSON.stringify({vendor: adapter.info.vendor, device: adapter.info.device}));
	})`)
	var info map[string]any
	if err := json.Unmarshal([]byte(infoJSON), &info); err != nil {
		t.Fatalf("unmarshal adapter info: %v (raw: %q)", err, infoJSON)
	}
	if info["vendor"] != "amd" {
		t.Errorf("new tab adapter.info.vendor = %q, want amd", info["vendor"])
	}
	if info["device"] != "AMD Radeon RX 580" {
		t.Errorf("new tab adapter.info.device = %q, want AMD Radeon RX 580", info["device"])
	}

	now := evalInt(t, p2, "() => Date.now()")
	if now%1000 != 0 {
		t.Errorf("new tab Date.now() = %d, want last three digits zero", now)
	}
}

// TestWebGPUTimingDetectorContradiction records a self-contained coherence check
// that the WebGPU vendor is consistent with the platform or the WebGL renderer.
func TestWebGPUTimingDetectorContradiction(t *testing.T) {
	status, note, value := "skipped", "Chrome unavailable or coherence check skipped", ""
	defer func() { recordDetectorResult("webgpu_timing_coherence", status, note, value) }()

	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
	profile, err := bp.Create(ProfileConfig{
		ID:   "wgpu-timing-contradiction-01",
		Name: "wgpu-timing-contradiction",
		Fingerprint: &FingerprintConfig{
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			Platform:  "Win32",
			Language:  "en-US",
			WebGL:     &WebGLConfig{Vendor: "Google Inc. (NVIDIA)", Renderer: "ANGLE (NVIDIA, NVIDIA GeForce RTX 3080)"},
			WebGPU:    &WebGPUConfig{Vendor: "nvidia", Architecture: "x86", Device: "NVIDIA GeForce RTX 3080", Description: "NVIDIA GeForce RTX 3080"},
			Timing:    &TimingConfig{Enabled: true, Precision: 1 * time.Second},
			ClientHints: &ClientHintsConfig{Platform: "Windows", PlatformVersion: "10.0.0", Architecture: "x86", Mobile: false},
		},
	})
	if err != nil {
		t.Fatalf("create profile: %v", err)
	}

	sess, err := WithProfile(bp, profile.ID, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("WithProfile: %v", err)
	}
	defer func() { _ = sess.Terminate() }()

	page := sess.Page
	navigate(t, page)

	if evalStr(t, page, "() => typeof navigator.gpu") == "undefined" {
		status = "skipped"
		note = "browser does not expose WebGPU"
		return
	}

	wgpuVendor := evalStr(t, page, `() => new Promise(async r => r((await navigator.gpu.requestAdapter()).info.vendor))`)
	glRenderer := evalStr(t, page, `() => { const c = document.createElement('canvas').getContext('webgl'); const ext = c.getExtension('WEBGL_debug_renderer_info'); return c.getParameter(ext.UNMASKED_RENDERER_WEBGL); }`)

	var contradictions []string
	if wgpuVendor != "nvidia" {
		contradictions = append(contradictions, fmt.Sprintf("webgpu.vendor=%q, want nvidia", wgpuVendor))
	}
	if !strings.Contains(glRenderer, "NVIDIA") {
		contradictions = append(contradictions, fmt.Sprintf("webgl.renderer=%q, want NVIDIA", glRenderer))
	}

	if len(contradictions) > 0 {
		status = "failed"
		note = "WebGPU/WebGL coherence contradictions detected"
		value = strings.Join(contradictions, "; ")
		t.Fatalf("WebGPU coherence contradictions: %s", value)
	}
	status = "passed"
	note = "WebGPU/WebGL coherence self-check passed"
	value = "vendor and renderer aligned"
}

func evalFloat(t *testing.T, p *rod.Page, js string) float64 {
	t.Helper()
	obj, err := p.Eval(js)
	if err != nil {
		t.Fatalf("eval %q: %v", js, err)
	}
	return obj.Value.Num()
}
