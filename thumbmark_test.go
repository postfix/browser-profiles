package browserprofiles

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// Independent-fingerprinter ORACLE test (real go-rod + real Chrome).
//
// The other integration tests prove our anti-detect overrides are readable via
// direct navigator.* probes we author. This test instead runs a vendored,
// third-party fingerprinting library — ThumbmarkJS — INSIDE a launched protected
// profile and asserts that an INDEPENDENT observer sees our spoof and computes a
// distinct identity per profile. That is the property that actually matters: a
// real fingerprinter, not our own probe, must be fooled.
//
// Loading mechanics that are load-bearing (get them wrong => false pass or hang):
//   - EvalOnNewDocument(umd) is registered on the page and THEN we navigate, so
//     BOTH our on-new-document anti-detect scripts AND ThumbmarkJS are present in
//     the measured document (a raw page.Eval of the UMD IIFE would not persist
//     across the navigate that arms our overrides).
//   - ThumbmarkJS defaults to stabilize:["private","iframe"], which fingerprints
//     inside an ephemeral iframe — a DIFFERENT context that need not carry our
//     page-level prototype overrides. We pass stabilize:[] so it measures THIS
//     (anti-detected) page.
//
// Like the other Bucket A tests, the running-browser tracking map is package
// GLOBAL, so this test never runs t.Parallel(), tears down every launch via
// t.Cleanup, and adds a CloseAllBrowsers safety net.
// ============================================================================

const thumbmarkUMDPath = "testdata/thumbmarkjs/thumbmark.umd.js"

// thumbmarkUMDSHA256 pins the vendored bundle to the exact artifact recorded in
// testdata/thumbmarkjs/PROVENANCE.md — the ThumbmarkJS build whose behavior the
// oracle test below was written against.
const thumbmarkUMDSHA256 = "5ada488a4a77730944f5e76b0d4590b4caedf74893ebc362eb6e606f301bc0d8"

// TestThumbmarkVendorBundleIntegrity (Chrome-free) fails if the vendored oracle
// drifts from its pinned provenance hash — a silent corruption or an un-reviewed
// CDN refresh would change what the independent fingerprinter measures and could
// invalidate the oracle test without anyone noticing. Guards the supply chain of
// a third-party artifact we do NOT compile but DO trust in tests.
func TestThumbmarkVendorBundleIntegrity(t *testing.T) {
	data, err := os.ReadFile(thumbmarkUMDPath)
	if err != nil {
		t.Fatalf("read vendored ThumbmarkJS: %v", err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != thumbmarkUMDSHA256 {
		t.Fatalf("vendored ThumbmarkJS sha256 = %s, want %s (bundle drifted from PROVENANCE.md; re-verify the oracle before updating the pin)",
			got, thumbmarkUMDSHA256)
	}
}

// thumbmarkResult is the JSON shape we build in-page from ThumbmarkJS's
// {thumbmark, components} result before shipping it back over CDP.
type thumbmarkResult struct {
	Hash       string          `json:"hash"`
	Components json.RawMessage `json:"components"`
}

// oracleObservation bundles one profile's independent-fingerprinter run: the
// ThumbmarkJS result plus a direct read of the UNMASKED per-profile WebGL identity.
// ThumbmarkJS only consults the WEBGL_debug_renderer_info extension when the masked
// RENDERER/VENDOR (getParameter 7936/7937) is empty; our spoof keeps those non-empty
// ("WebKit"/"WebKit WebGL"), so the per-profile UNMASKED renderer is proven by reading
// getParameter(UNMASKED_RENDERER_WEBGL) directly when ThumbmarkJS surfaces the mask.
type oracleObservation struct {
	res                    thumbmarkResult
	directUnmaskedRenderer string
	directUnmaskedVendor   string
}

// runThumbmarkOracle launches a protected profile with the given fingerprint,
// arms ThumbmarkJS on the next document, navigates, cross-checks the ground-truth
// navigator override, then runs the independent fingerprinter and returns its
// observed result. All teardown is registered on t.
func runThumbmarkOracle(t *testing.T, bp *BrowserProfiles, umd, id, name string, cfg *FingerprintConfig) oracleObservation {
	t.Helper()

	profile, err := bp.Create(ProfileConfig{ID: id, Name: name, Fingerprint: cfg})
	if err != nil {
		t.Fatalf("%s: create profile: %v", id, err)
	}

	sess, err := WithProfile(bp, profile.ID, LaunchOptions{Headless: true})
	if err != nil {
		t.Fatalf("%s: WithProfile: %v", id, err)
	}
	t.Cleanup(func() { _ = sess.Terminate() })

	page := sess.Page

	// Register the fingerprinter to load on the NEXT document, then navigate so
	// it co-resides with our anti-detect overrides in the measured context.
	if _, err := page.EvalOnNewDocument(umd); err != nil {
		t.Fatalf("%s: register ThumbmarkJS: %v", id, err)
	}
	navigate(t, page)

	// Ground truth: prove the anti-detect navigator override is live in THIS
	// document before we trust what the independent observer reads from it.
	if hw := evalInt(t, page, "() => navigator.hardwareConcurrency"); hw != cfg.HardwareConcurrency {
		t.Fatalf("%s: ground-truth navigator.hardwareConcurrency = %d, want %d (anti-detect not active in measured context)",
			id, hw, cfg.HardwareConcurrency)
	}

	// Run ThumbmarkJS and await its promise (Eval sets AwaitPromise). stabilize:[]
	// keeps the measurement in this page's context; audio+permissions are excluded
	// because they carry NONE of our spoofs and are the flaky/slow components in
	// headless — hardware/system/webgl (which do carry our overrides) are kept.
	const js = `() => new window.ThumbmarkJS.Thumbmark({
		stabilize: [],
		logging: false,
		timeout: 15000,
		exclude: ["audio", "permissions"]
	}).get().then(r => JSON.stringify({hash: r.thumbmark, components: r.components}))`

	obj, err := page.Timeout(60 * time.Second).Eval(js)
	if err != nil {
		t.Fatalf("%s: ThumbmarkJS get(): %v", id, err)
	}

	var res thumbmarkResult
	if err := json.Unmarshal([]byte(obj.Value.Str()), &res); err != nil {
		t.Fatalf("%s: unmarshal ThumbmarkJS result: %v\nraw=%s", id, err, obj.Value.Str())
	}
	if res.Hash == "" {
		t.Fatalf("%s: ThumbmarkJS returned empty thumbmark hash; components=%s", id, res.Components)
	}

	// Direct read of the UNMASKED per-profile GPU identity our WebGL spoof controls via
	// getParameter(37445/37446) behind the WEBGL_debug_renderer_info extension. This is
	// the ground truth for the F4 fix: the static path would return a random ANGLE(...)
	// string here, whereas the per-profile builder returns exactly our configured values.
	const unmaskedJS = `() => {
		const c = document.createElement('canvas').getContext('webgl') ||
			document.createElement('canvas').getContext('experimental-webgl');
		if (!c) return '';
		const e = c.getExtension('WEBGL_debug_renderer_info');
		if (!e) return '';
		return JSON.stringify({
			renderer: String(c.getParameter(e.UNMASKED_RENDERER_WEBGL) || ''),
			vendor: String(c.getParameter(e.UNMASKED_VENDOR_WEBGL) || '')
		});
	}`
	var unmasked struct{ Renderer, Vendor string }
	if err := json.Unmarshal([]byte(evalStr(t, page, unmaskedJS)), &unmasked); err != nil {
		t.Fatalf("%s: read UNMASKED webgl: %v", id, err)
	}
	return oracleObservation{res: res, directUnmaskedRenderer: unmasked.Renderer, directUnmaskedVendor: unmasked.Vendor}
}

// findNumberField recursively searches the components JSON for the FIRST field
// literally named key whose value is numeric (or a numeric string), returning it.
// This ties a ThumbmarkJS-surfaced value to our config without hard-coding the
// exact nesting path (robust to ThumbmarkJS restructuring, still precise on key).
func findNumberField(raw json.RawMessage, key string) (float64, bool) {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, false
	}
	return walkForNumber(v, key)
}

func walkForNumber(v interface{}, key string) (float64, bool) {
	switch node := v.(type) {
	case map[string]interface{}:
		if child, ok := node[key]; ok {
			if n, ok := asNumber(child); ok {
				return n, true
			}
		}
		for _, child := range node {
			if n, ok := walkForNumber(child, key); ok {
				return n, true
			}
		}
	case []interface{}:
		for _, child := range node {
			if n, ok := walkForNumber(child, key); ok {
				return n, true
			}
		}
	}
	return 0, false
}

func asNumber(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	}
	return 0, false
}

// findStringField recursively searches the components JSON for the FIRST field
// literally named key whose value is a non-empty string. Used to REPORT exactly what
// ThumbmarkJS surfaced for a key (e.g. the videocard renderer) without hard-coding the
// nesting path.
func findStringField(raw json.RawMessage, key string) (string, bool) {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", false
	}
	return walkForString(v, key)
}

func walkForString(v interface{}, key string) (string, bool) {
	switch node := v.(type) {
	case map[string]interface{}:
		if child, ok := node[key]; ok {
			if s, ok := child.(string); ok && s != "" {
				return s, true
			}
		}
		for _, child := range node {
			if s, ok := walkForString(child, key); ok {
				return s, true
			}
		}
	case []interface{}:
		for _, child := range node {
			if s, ok := walkForString(child, key); ok {
				return s, true
			}
		}
	}
	return "", false
}

// componentsContain reports whether the raw components JSON text contains sub.
func componentsContain(raw json.RawMessage, sub string) bool {
	return strings.Contains(string(raw), sub)
}

// TestThumbmarkOracleObservesFingerprint proves the anti-detect works against an
// independent fingerprinter:
//   - PRIMARY (observability): ThumbmarkJS reads back a value we injected —
//     navigator.hardwareConcurrency==12 and deviceMemory==16 surface in profile
//     A's components, and the observed platform is our spoofed "MacIntel".
//   - SECONDARY (distinct identity): profile B (differently configured, SAME
//     machine + Chrome) yields a different ThumbmarkJS hash, and its components
//     carry our "Win32" spoof and never leak "MacIntel".
//   - WEBGL (per-profile GPU): each profile injects a distinct UNMASKED renderer;
//     proven either via ThumbmarkJS components (preferred) or, when ThumbmarkJS reads
//     the masked RENDERER, via a direct getParameter(UNMASKED_RENDERER_WEBGL) read.
func TestThumbmarkOracleObservesFingerprint(t *testing.T) {
	status, note, value := "skipped", "Chrome unavailable or test skipped", ""
	defer func() { recordDetectorResult("thumbmarkjs", status, note, value) }()

	requireChrome(t)
	t.Cleanup(func() { CloseAllBrowsers() })

	umd, err := os.ReadFile(thumbmarkUMDPath)
	if err != nil {
		t.Fatalf("read vendored ThumbmarkJS: %v", err)
	}

	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})

	// Distinctive, non-colliding spoofs so substring/value checks are unambiguous.
	cfgA := &FingerprintConfig{
		Platform: "MacIntel", Language: "en-US",
		HardwareConcurrency: 12, DeviceMemory: 16,
		WebGL: &WebGLConfig{Vendor: "Acme Inc.", Renderer: "AcmeGPU-Model-A-9999"},
	}
	cfgB := &FingerprintConfig{
		Platform: "Win32", Language: "en-US",
		HardwareConcurrency: 4, DeviceMemory: 8,
		WebGL: &WebGLConfig{Vendor: "Beta LLC", Renderer: "BetaGPU-Model-B-1111"},
	}

	obsA := runThumbmarkOracle(t, bp, string(umd), "thumbmark-oracle-a", "oracle-A", cfgA)
	obsB := runThumbmarkOracle(t, bp, string(umd), "thumbmark-oracle-b", "oracle-B", cfgB)
	resA, resB := obsA.res, obsB.res

	// Discovery aid: dump the observed structure so the exact keys ThumbmarkJS
	// surfaces are visible in `go test -v` and to future maintainers.
	t.Logf("profile A: hash=%s", resA.Hash)
	t.Logf("profile A: components=%s", resA.Components)
	t.Logf("profile B: hash=%s", resB.Hash)
	t.Logf("profile B: components=%s", resB.Components)

	// -- PRIMARY: independent observer read our injected hardware config. --------
	if hw, ok := findNumberField(resA.Components, "hardwareConcurrency"); !ok || int(hw) != 12 {
		t.Fatalf("profile A: ThumbmarkJS hardwareConcurrency = %v (found=%v), want 12 (our injected value not observed)", hw, ok)
	}
	if mem, ok := findNumberField(resA.Components, "deviceMemory"); !ok || int(mem) != 16 {
		t.Fatalf("profile A: ThumbmarkJS deviceMemory = %v (found=%v), want 16 (our injected value not observed)", mem, ok)
	}
	// Our spoofed platform is surfaced by the independent observer.
	if !componentsContain(resA.Components, "MacIntel") {
		t.Fatalf("profile A: ThumbmarkJS components do not contain spoofed platform \"MacIntel\": %s", resA.Components)
	}

	// -- SECONDARY: distinct per-profile identity from the independent view. -----
	// Same machine + same Chrome; the ONLY difference is our per-profile injection,
	// so a different independent-fingerprint hash proves the injection changes what
	// a real fingerprinter computes.
	if resA.Hash == resB.Hash {
		t.Fatalf("profile A and B produced the SAME ThumbmarkJS hash %q; per-profile anti-detect did not change the independent fingerprint", resA.Hash)
	}
	// Profile B carries our "Win32" spoof and never leaks A's "MacIntel".
	if !componentsContain(resB.Components, "Win32") {
		t.Fatalf("profile B: ThumbmarkJS components do not contain spoofed platform \"Win32\": %s", resB.Components)
	}
	if componentsContain(resB.Components, "MacIntel") {
		t.Fatalf("profile B: ThumbmarkJS components leaked profile A's platform \"MacIntel\": %s", resB.Components)
	}
	// B's hardware config is observed distinctly from A's.
	if hw, ok := findNumberField(resB.Components, "hardwareConcurrency"); !ok || int(hw) != 4 {
		t.Fatalf("profile B: ThumbmarkJS hardwareConcurrency = %v (found=%v), want 4", hw, ok)
	}

	// -- WEBGL: per-profile UNMASKED GPU identity is now injected (F4 fix). --------
	// Distinct renderers per profile; prove an independent view distinguishes them.
	const rendererA, rendererB = "AcmeGPU-Model-A-9999", "BetaGPU-Model-B-1111"
	if componentsContain(resA.Components, rendererA) {
		// Best case: the independent observer itself surfaced our per-profile renderer.
		tmA, _ := findStringField(resA.Components, "renderer")
		t.Logf("ThumbmarkJS OBSERVED per-profile WebGL renderer: A videocard.renderer=%q", tmA)
		if !componentsContain(resB.Components, rendererB) {
			t.Fatalf("profile B: ThumbmarkJS did not surface per-profile WebGL renderer %q: %s", rendererB, resB.Components)
		}
		if componentsContain(resB.Components, rendererA) {
			t.Fatalf("profile B: ThumbmarkJS leaked profile A's WebGL renderer %q: %s", rendererA, resB.Components)
		}
	} else {
		// ThumbmarkJS reads the MASKED RENDERER (getParameter 7937) and consults the
		// WEBGL_debug_renderer_info extension ONLY when the mask is empty, so it surfaces
		// the generic "WebKit WebGL". The extension IS present (F4), so prove the
		// per-profile injection is live by reading UNMASKED directly, and report exactly
		// what ThumbmarkJS surfaced so the architect can decide whether to also
		// parameterize the masked 7937.
		tmA, _ := findStringField(resA.Components, "renderer")
		tmB, _ := findStringField(resB.Components, "renderer")
		t.Logf("ThumbmarkJS surfaced MASKED videocard.renderer A=%q B=%q (reads getParameter(7937), not UNMASKED)", tmA, tmB)
		if obsA.directUnmaskedRenderer != rendererA {
			t.Fatalf("profile A: direct UNMASKED_RENDERER_WEBGL = %q, want %q (per-profile WebGL injection not live)", obsA.directUnmaskedRenderer, rendererA)
		}
		if obsB.directUnmaskedRenderer != rendererB {
			t.Fatalf("profile B: direct UNMASKED_RENDERER_WEBGL = %q, want %q", obsB.directUnmaskedRenderer, rendererB)
		}
		if obsB.directUnmaskedRenderer == rendererA {
			t.Fatalf("profile B: direct UNMASKED renderer leaked profile A's %q", rendererA)
		}
		t.Logf("direct UNMASKED per-profile identity VERIFIED: A renderer=%q vendor=%q; B renderer=%q vendor=%q",
			obsA.directUnmaskedRenderer, obsA.directUnmaskedVendor, obsB.directUnmaskedRenderer, obsB.directUnmaskedVendor)
	}
	status = "passed"
	note = "ThumbmarkJS observed distinct per-profile fingerprints"
	value = resA.Hash + " / " + resB.Hash
	t.Logf("ThumbmarkJS baseline recorded: A=%s B=%s", resA.Hash, resB.Hash)
}
