package browserprofiles

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// detectorBaselinePath is the canonical location for the Phase 09 detector-oracle
// baseline snapshot. It is recorded by the oracle tests and consumed by later phases
// as a before/after comparison artifact.
const detectorBaselinePath = ".planning/data/09-detector-baseline.json"

type detectorResult struct {
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
	Value  string `json:"value,omitempty"`
}

var (
	baselineMu     sync.Mutex
	baselineLoaded bool
	baselineData   map[string]detectorResult
)

// loadBaseline reads the existing baseline file (or creates an empty map) so that
// each oracle test can update its own key without clobbering the others.
func loadBaseline() map[string]detectorResult {
	baselineMu.Lock()
	defer baselineMu.Unlock()
	if baselineLoaded {
		return baselineData
	}
	baselineData = map[string]detectorResult{}
	if b, err := os.ReadFile(detectorBaselinePath); err == nil {
		_ = json.Unmarshal(b, &baselineData)
	}
	baselineLoaded = true
	return baselineData
}

// saveBaseline persists the current baseline map to detectorBaselinePath.
func saveBaseline(data map[string]detectorResult) error {
	if err := os.MkdirAll(filepath.Dir(detectorBaselinePath), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(detectorBaselinePath, b, 0o644)
}

// recordDetectorResult records a single detector entry in the shared baseline file.
// It is safe for concurrent use by the oracle tests.
func recordDetectorResult(name, status, note, value string) {
	data := loadBaseline()
	baselineMu.Lock()
	data[name] = detectorResult{Status: status, Note: note, Value: value}
	baselineMu.Unlock()
	_ = saveBaseline(data)
}

// networkOraclesEnabled reports whether the operator has explicitly opted into running
// tests that hit third-party detector sites. Default is disabled (skip-safe).
func networkOraclesEnabled() bool {
	return os.Getenv("BROWSER_PROFILES_RUN_NETWORK_ORACLES") == "1"
}

// detectorPageReachable returns a non-nil error if the given URL cannot be reached
// within a short timeout. Used to skip gracefully when the network is unavailable.
func detectorPageReachable(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

// launchDetectorProfile launches a temporary, deterministic profile and returns a
// Session and its cleanup closure. It skips if Chrome is unavailable.
func launchDetectorProfile(t *testing.T) *Session {
	t.Helper()
	requireChrome(t)
	profile := &FingerprintConfig{
		Platform:            "Win32",
		Language:            "en-US",
		HardwareConcurrency: 8,
		DeviceMemory:        8,
	}
	sess, err := CreateSession(CreateSessionOptions{
		Headless:          true,
		RandomFingerprint: new(bool),
		Fingerprint:       profile,
	})
	if err != nil {
		t.Fatalf("launch detector profile: %v", err)
	}
	t.Cleanup(func() { _ = sess.Terminate() })
	return sess
}

// TestCreepJSOracle navigates to the CreepJS demo page, waits for a stable score
// selector, and records the observed trust score. It is skipped when Chrome or the
// network is unavailable, and when network oracles are not explicitly enabled.
func TestCreepJSOracle(t *testing.T) {
	status, note, value := "skipped", "Chrome unavailable or network oracles disabled", ""
	defer func() { recordDetectorResult("creepjs", status, note, value) }()

	if !networkOraclesEnabled() {
		status = "skipped"
		note = "BROWSER_PROFILES_RUN_NETWORK_ORACLES is not set to 1"
		t.Skip(note)
	}
	if err := detectorPageReachable("https://abrahamjuliot.github.io/creepjs/"); err != nil {
		status = "skipped"
		note = fmt.Sprintf("CreepJS page unreachable: %v", err)
		t.Skip(note)
	}

	sess := launchDetectorProfile(t)
	page := sess.Page
	if err := page.Navigate("https://abrahamjuliot.github.io/creepjs/"); err != nil {
		status = "skipped"
		note = fmt.Sprintf("navigate to CreepJS: %v", err)
		t.Skip(note)
	}
	if err := page.WaitLoad(); err != nil {
		status = "skipped"
		note = fmt.Sprintf("wait CreepJS load: %v", err)
		t.Skip(note)
	}
	// Wait for the score element to stabilise. CreepJS renders the trust score in a
	// element with id "fingerprint" or similar; if it is absent we record unstable.
	text := ""
	if el, err := page.Timeout(15 * time.Second).Element("#fingerprint"); err == nil {
		text, _ = el.Text()
	}
	if text == "" {
		if body, err := page.Timeout(15 * time.Second).Eval(`() => document.body.innerText`); err == nil {
			text = body.Value.Str()
		}
	}
	// Extract a compact score token: "trust score: NN%" or "unstable".
	score := "unstable"
	if idx := strings.Index(strings.ToLower(text), "trust score"); idx >= 0 {
		rest := text[idx:]
		if end := strings.IndexAny(rest, "\n\r"); end > 0 {
			rest = rest[:end]
		}
		score = strings.TrimSpace(rest)
	}
	status = "passed"
	note = "CreepJS trust score observed"
	value = score
	t.Logf("CreepJS score: %s", score)
}

// TestBrowserLeaksOracle navigates to BrowserLeaks Canvas and records the observed
// canvas fingerprint hash. It skips when Chrome/network is unavailable or when
// network oracles are disabled.
func TestBrowserLeaksOracle(t *testing.T) {
	status, note, value := "skipped", "Chrome unavailable or network oracles disabled", ""
	defer func() { recordDetectorResult("browserleaks", status, note, value) }()

	if !networkOraclesEnabled() {
		status = "skipped"
		note = "BROWSER_PROFILES_RUN_NETWORK_ORACLES is not set to 1"
		t.Skip(note)
	}
	if err := detectorPageReachable("https://browserleaks.com/canvas"); err != nil {
		status = "skipped"
		note = fmt.Sprintf("BrowserLeaks page unreachable: %v", err)
		t.Skip(note)
	}

	sess := launchDetectorProfile(t)
	page := sess.Page
	if err := page.Navigate("https://browserleaks.com/canvas"); err != nil {
		status = "skipped"
		note = fmt.Sprintf("navigate to BrowserLeaks: %v", err)
		t.Skip(note)
	}
	if err := page.WaitLoad(); err != nil {
		status = "skipped"
		note = fmt.Sprintf("wait BrowserLeaks load: %v", err)
		t.Skip(note)
	}

	// The Canvas page renders a hash in a element with id "hash" or class "hash".
	hash := ""
	for _, sel := range []string{"#hash", ".hash", "[id*=hash]", "[class*=hash]"} {
		if el, err := page.Timeout(15 * time.Second).Element(sel); err == nil {
			if txt, _ := el.Text(); txt != "" {
				hash = strings.TrimSpace(txt)
				break
			}
		}
	}
	if hash == "" {
		status = "skipped"
		note = "BrowserLeaks canvas hash element not found"
		t.Skip(note)
	}
	status = "passed"
	note = "BrowserLeaks Canvas hash observed"
	value = hash
	t.Logf("BrowserLeaks canvas hash: %s", hash)
}
