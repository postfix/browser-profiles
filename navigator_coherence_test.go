package browserprofiles

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// connectionProps matches the subset of navigator.connection we assert on.
type connectionProps struct {
	EffectiveType string  `json:"effectiveType"`
	Downlink      float64 `json:"downlink"`
	Rtt           int     `json:"rtt"`
	SaveData      bool    `json:"saveData"`
}

// evalBool evaluates a JS expression and returns its boolean result.
func evalBool(t *testing.T, p *rod.Page, js string) bool {
	t.Helper()
	obj, err := p.Eval(js)
	if err != nil {
		t.Fatalf("eval %q: %v", js, err)
	}
	return obj.Value.Bool()
}

// evalJSON evaluates a JS expression and unmarshals the JSON result into v.
func evalJSON(t *testing.T, p *rod.Page, js string, v any) {
	t.Helper()
	obj, err := p.Eval(js)
	if err != nil {
		t.Fatalf("eval %q: %v", js, err)
	}
	if err := json.Unmarshal([]byte(obj.Value.Str()), v); err != nil {
		t.Fatalf("unmarshal %q: %v", js, err)
	}
}

// TestNavigatorCoherenceSmoke verifies that navigator.appVersion, productSub,
// vendor, maxTouchPoints, mobile, and connection are coherent with the generated
// profile and injected on the launch target. Requires real Chrome.
func TestNavigatorCoherenceSmoke(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
	profile, err := bp.Create(ProfileConfig{
		ID:   "nav-coh-01",
		Name: "nav-coh",
		Fingerprint: &FingerprintConfig{
			UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			Platform:            "Win32",
			Language:            "en-US",
			HardwareConcurrency: 8,
			DeviceMemory:        8,
			Vendor:              "Google Inc.",
			AppVersion:          "5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			ProductSub:          "20030107",
			MaxTouchPoints:      0,
			Mobile:              false,
			Connection: &NavigatorConnection{
				EffectiveType: "4g", Downlink: 10, Rtt: 50, SaveData: false,
			},
			ClientHints: &ClientHintsConfig{
				Platform:        "Windows",
				PlatformVersion: "10.0.0",
				Architecture:    "x86",
				Mobile:          false,
				FullVersion:     "120.0.0.0",
				Brands: []Brand{
					{Brand: "Chromium", Version: "120"},
					{Brand: "Google Chrome", Version: "120"},
					{Brand: "Not_A Brand", Version: "8"},
				},
			},
			WebGL: &WebGLConfig{
				Vendor:   "Google Inc. (NVIDIA)",
				Renderer: "ANGLE (NVIDIA, NVIDIA GeForce RTX 3080)",
				Caps: &WebGLCaps{
					MaxTextureSize: 32768, MaxCubeMapTextureSize: 32768, MaxRenderbufferSize: 32768,
					MaxVaryingVectors: 31, MaxVertexUniformVectors: 4096,
					MaxViewportDims: []int{32768, 32768},
					AliasedLineWidthRange: []float64{1, 1}, AliasedPointSizeRange: []float64{1, 2047},
					MaxTextureImageUnits: 32, MaxVertexTextureImageUnits: 32, MaxCombinedTextureImageUnits: 192,
					MaxFragmentUniformVectors: 1024, MaxVertexAttribs: 29,
				},
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

	if got := evalStr(t, page, "() => navigator.userAgent"); !strings.Contains(got, "Windows NT 10.0") {
		t.Errorf("navigator.userAgent = %q, want Windows NT 10.0", got)
	}
	if got := evalStr(t, page, "() => navigator.appVersion"); !strings.HasPrefix(got, "5.0 (") {
		t.Errorf("navigator.appVersion = %q, want 5.0 (...) prefix", got)
	}
	if got := evalStr(t, page, "() => navigator.productSub"); got != "20030107" {
		t.Errorf("navigator.productSub = %q, want 20030107", got)
	}
	if got := evalStr(t, page, "() => navigator.vendor"); got != "Google Inc." {
		t.Errorf("navigator.vendor = %q, want Google Inc.", got)
	}
	if got := evalInt(t, page, "() => navigator.maxTouchPoints"); got != 0 {
		t.Errorf("navigator.maxTouchPoints = %d, want 0", got)
	}
	if got := evalBool(t, page, "() => navigator.userAgentData.mobile"); got {
		t.Errorf("navigator.userAgentData.mobile = %v, want false", got)
	}

	connJSON := evalStr(t, page, `() => { const c = navigator.connection; return c === undefined ? "undefined" : JSON.stringify({effectiveType: c.effectiveType, downlink: c.downlink, rtt: c.rtt, saveData: c.saveData}); }`)
	if connJSON == "undefined" {
		t.Log("navigator.connection is undefined in this Chrome configuration; skipping connection assertion")
	} else {
		var conn connectionProps
		if err := json.Unmarshal([]byte(connJSON), &conn); err != nil {
			t.Fatalf("unmarshal connection: %v (raw: %q)", err, connJSON)
		}
		if conn.EffectiveType != "4g" || conn.Downlink != 10 || conn.Rtt != 50 || conn.SaveData {
			t.Errorf("navigator.connection = %+v, want {4g, 10, 50, false}", conn)
		}
	}

	if got := evalStr(t, page, "() => navigator.userAgentData.platform"); got != "Windows" {
		t.Errorf("navigator.userAgentData.platform = %q, want Windows", got)
	}
	brands := evalStr(t, page, "() => JSON.stringify(navigator.userAgentData.brands)")
	if !strings.Contains(brands, "Chrome") || !strings.Contains(brands, "120") {
		t.Errorf("navigator.userAgentData.brands = %q, want Chrome/120", brands)
	}

	arch := evalStr(t, page, `() => new Promise(r => navigator.userAgentData.getHighEntropyValues(['architecture']).then(v => r(v.architecture)))`)
	if arch != "x86" {
		t.Errorf("getHighEntropyValues architecture = %q, want x86", arch)
	}
	platVer := evalStr(t, page, `() => new Promise(r => navigator.userAgentData.getHighEntropyValues(['platformVersion']).then(v => r(v.platformVersion)))`)
	if platVer != "10.0.0" {
		t.Errorf("getHighEntropyValues platformVersion = %q, want 10.0.0", platVer)
	}

	glVendor := evalStr(t, page, `() => { const c = document.createElement('canvas').getContext('webgl'); const ext = c.getExtension('WEBGL_debug_renderer_info'); return c.getParameter(ext.UNMASKED_VENDOR_WEBGL); }`)
	if glVendor != "Google Inc. (NVIDIA)" {
		t.Errorf("UNMASKED_VENDOR_WEBGL = %q, want Google Inc. (NVIDIA)", glVendor)
	}
	glRenderer := evalStr(t, page, `() => { const c = document.createElement('canvas').getContext('webgl'); const ext = c.getExtension('WEBGL_debug_renderer_info'); return c.getParameter(ext.UNMASKED_RENDERER_WEBGL); }`)
	if glRenderer != "ANGLE (NVIDIA, NVIDIA GeForce RTX 3080)" {
		t.Errorf("UNMASKED_RENDERER_WEBGL = %q, want ANGLE (NVIDIA, NVIDIA GeForce RTX 3080)", glRenderer)
	}
	maxTex := evalInt(t, page, `() => { const c = document.createElement('canvas').getContext('webgl'); return c.getParameter(c.MAX_TEXTURE_SIZE); }`)
	if maxTex != 32768 {
		t.Errorf("MAX_TEXTURE_SIZE = %d, want 32768", maxTex)
	}
	maxVpRaw := evalStr(t, page, `() => { const c = document.createElement('canvas').getContext('webgl'); const v = c.getParameter(c.MAX_VIEWPORT_DIMS); return JSON.stringify(v); }`)
	t.Logf("MAX_VIEWPORT_DIMS raw: %q", maxVpRaw)
	var maxVp []int
	if err := json.Unmarshal([]byte(maxVpRaw), &maxVp); err != nil {
		t.Fatalf("unmarshal MAX_VIEWPORT_DIMS: %v (raw: %q)", err, maxVpRaw)
	}
	if len(maxVp) != 2 || maxVp[0] != 32768 || maxVp[1] != 32768 {
		t.Errorf("MAX_VIEWPORT_DIMS = %v, want [32768, 32768]", maxVp)
	}
}

// TestSecCHUAHeaders verifies that the network request headers carry coherent
// Sec-CH-UA* values matching the injected client-hints metadata. Real Chrome required.
func TestSecCHUAHeaders(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	// Chrome only sends Sec-CH-UA* request headers to origins that have previously
	// delivered an Accept-CH response header. We serve that on the first request
	// and then navigate a second time so the client hints are actually emitted.
	var (
		secChUA       string
		secChUAMobile string
		secChUAPlat   string
		secChUAFull   string
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		secChUA = r.Header.Get("Sec-CH-UA")
		secChUAMobile = r.Header.Get("Sec-CH-UA-Mobile")
		secChUAPlat = r.Header.Get("Sec-CH-UA-Platform")
		secChUAFull = r.Header.Get("Sec-CH-UA-Full-Version-List")
		w.Header().Set("Accept-CH", "Sec-CH-UA, Sec-CH-UA-Mobile, Sec-CH-UA-Platform, Sec-CH-UA-Full-Version-List")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
	profile, err := bp.Create(ProfileConfig{
		ID:   "sec-ch-ua-01",
		Name: "sec-ch-ua",
		Fingerprint: &FingerprintConfig{
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			Platform:  "Win32",
			Language:  "en-US",
			Mobile:    false,
			ClientHints: &ClientHintsConfig{
				Platform:        "Windows",
				PlatformVersion: "10.0.0",
				Architecture:    "x86",
				Mobile:          false,
				FullVersion:     "120.0.0.0",
				Brands: []Brand{
					{Brand: "Chromium", Version: "120"},
					{Brand: "Google Chrome", Version: "120"},
					{Brand: "Not_A Brand", Version: "8"},
				},
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

	if err := sess.Page.Navigate(server.URL + "/"); err != nil {
		t.Fatalf("navigate to test server: %v", err)
	}
	if err := sess.Page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}
	// Second request after Accept-CH has been delivered; this is when Chrome
	// attaches the client-hint headers.
	if err := sess.Page.Navigate(server.URL + "/second"); err != nil {
		t.Fatalf("navigate to test server second request: %v", err)
	}
	if err := sess.Page.WaitLoad(); err != nil {
		t.Fatalf("wait load second request: %v", err)
	}

	if secChUA == "" {
		t.Errorf("Sec-CH-UA header missing")
	}
	if !strings.Contains(secChUA, "Google Chrome") || !strings.Contains(secChUA, "120") {
		t.Errorf("Sec-CH-UA = %q, want Chrome 120 brand", secChUA)
	}
	if secChUAMobile != "?0" {
		t.Errorf("Sec-CH-UA-Mobile = %q, want ?0", secChUAMobile)
	}
	if got := strings.Trim(secChUAPlat, `"`); got != "Windows" {
		t.Errorf("Sec-CH-UA-Platform = %q, want Windows", secChUAPlat)
	}
	if secChUAFull == "" || !strings.Contains(secChUAFull, "120.0.0.0") {
		t.Errorf("Sec-CH-UA-Full-Version-List = %q, want 120.0.0.0", secChUAFull)
	}
}

// TestNavigatorCoherenceCrossTab verifies that the protection bundle (navigator +
// client-hints + WebGL caps) reaches a newly opened tab. Real Chrome required.
func TestNavigatorCoherenceCrossTab(t *testing.T) {
	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
	profile, err := bp.Create(ProfileConfig{
		ID:   "nav-crosstab-01",
		Name: "nav-crosstab",
		Fingerprint: &FingerprintConfig{
			UserAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			Platform:       "Win32",
			Language:       "en-US",
			Vendor:         "Google Inc.",
			ProductSub:     "20030107",
			MaxTouchPoints: 0,
			Mobile:         false,
			Connection: &NavigatorConnection{
				EffectiveType: "4g", Downlink: 10, Rtt: 50, SaveData: false,
			},
			ClientHints: &ClientHintsConfig{
				Platform:        "Windows",
				PlatformVersion: "10.0.0",
				Architecture:    "x86",
				Mobile:          false,
				FullVersion:     "120.0.0.0",
				Brands: []Brand{
					{Brand: "Chromium", Version: "120"},
					{Brand: "Google Chrome", Version: "120"},
					{Brand: "Not_A Brand", Version: "8"},
				},
			},
			WebGL: &WebGLConfig{
				Vendor:   "Google Inc. (NVIDIA)",
				Renderer: "ANGLE (NVIDIA, NVIDIA GeForce RTX 3080)",
				Caps: &WebGLCaps{
					MaxTextureSize: 32768, MaxCubeMapTextureSize: 32768, MaxRenderbufferSize: 32768,
					MaxVaryingVectors: 31, MaxVertexUniformVectors: 4096,
					MaxViewportDims: []int{32768, 32768},
					AliasedLineWidthRange: []float64{1, 1}, AliasedPointSizeRange: []float64{1, 2047},
					MaxTextureImageUnits: 32, MaxVertexTextureImageUnits: 32, MaxCombinedTextureImageUnits: 192,
					MaxFragmentUniformVectors: 1024, MaxVertexAttribs: 29,
				},
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
	navigate(t, p2)

	if got := evalStr(t, p2, "() => navigator.vendor"); got != "Google Inc." {
		t.Errorf("new tab navigator.vendor = %q, want Google Inc.", got)
	}
	if got := evalStr(t, p2, "() => navigator.userAgentData.platform"); got != "Windows" {
		t.Errorf("new tab navigator.userAgentData.platform = %q, want Windows", got)
	}
	maxTex := evalInt(t, p2, `() => { const c = document.createElement('canvas').getContext('webgl'); return c.getParameter(c.MAX_TEXTURE_SIZE); }`)
	if maxTex != 32768 {
		t.Errorf("new tab MAX_TEXTURE_SIZE = %d, want 32768", maxTex)
	}
}

// TestNavigatorCoherenceDetectorContradiction performs a self-contained coherence
// check: navigator.userAgent, navigator.platform, navigator.appVersion, and
// navigator.userAgentData must describe the same platform/browser/version. It
// records a navigator_coherence entry in the detector baseline. Real Chrome required.
func TestNavigatorCoherenceDetectorContradiction(t *testing.T) {
	status, note, value := "skipped", "Chrome unavailable or coherence check skipped", ""
	defer func() { recordDetectorResult("navigator_coherence", status, note, value) }()

	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
	profile, err := bp.Create(ProfileConfig{
		ID:   "nav-contradiction-01",
		Name: "nav-contradiction",
		Fingerprint: &FingerprintConfig{
			UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			Platform:   "Win32",
			Language:   "en-US",
			Vendor:     "Google Inc.",
			AppVersion:  "5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			ProductSub: "20030107",
			Mobile:     false,
			ClientHints: &ClientHintsConfig{
				Platform:     "Windows",
				Architecture: "x86",
				Mobile:       false,
				FullVersion:  "120.0.0.0",
				Brands: []Brand{
					{Brand: "Google Chrome", Version: "120"},
					{Brand: "Chromium", Version: "120"},
					{Brand: "Not_A Brand", Version: "8"},
				},
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

	ua := evalStr(t, page, "() => navigator.userAgent")
	plat := evalStr(t, page, "() => navigator.platform")
	appVer := evalStr(t, page, "() => navigator.appVersion")
	chPlat := evalStr(t, page, "() => navigator.userAgentData.platform")
	chMobile := evalBool(t, page, "() => navigator.userAgentData.mobile")

	var contradictions []string
	if !strings.Contains(ua, "Windows NT 10.0") {
		contradictions = append(contradictions, "userAgent missing Windows NT 10.0")
	}
	if plat != "Win32" {
		contradictions = append(contradictions, fmt.Sprintf("platform=%q, want Win32", plat))
	}
	if !strings.Contains(appVer, "Windows NT 10.0") {
		contradictions = append(contradictions, "appVersion missing Windows NT 10.0")
	}
	if chPlat != "Windows" {
		contradictions = append(contradictions, fmt.Sprintf("userAgentData.platform=%q, want Windows", chPlat))
	}
	if chMobile {
		contradictions = append(contradictions, "userAgentData.mobile is true for desktop")
	}
	if plat == "Win32" && chPlat != "Windows" {
		contradictions = append(contradictions, "platform Win32 inconsistent with userAgentData.platform")
	}

	if len(contradictions) > 0 {
		status = "failed"
		note = "navigator coherence contradictions detected"
		value = strings.Join(contradictions, "; ")
		t.Fatalf("navigator coherence contradictions: %s", value)
	}
	status = "passed"
	note = "navigator coherence self-check passed"
	value = "no contradictions"
}
