// ============================================================================
// @aitofy/browser-profiles - go-rod convenience layer (Go port of
// src/integrations/puppeteer.ts)
// ============================================================================
//
// Thin convenience constructors over the already-built core (BrowserProfiles,
// LaunchChrome[Standalone], the fingerprint package). Puppeteer's dynamic
// require + native re-exports collapse to go-rod's rod.New().ControlURL(ws).
// connectPuppeteer is dropped (subsumed by ControlURL().Connect()).
//
// M5: protections must reach EVERY page, not just the launch target.
// EvalOnNewDocument is per-target, so installProtections enables CDP target
// discovery and runs a browser-level EachEvent(TargetTargetCreated) loop that
// re-injects the protection bundle into each newly created page. The loop is
// stoppable via the Session.Close/Terminate closures.

package browserprofiles

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"

	"github.com/postfix/browser-profiles/fingerprint"
)

// Session is a connected go-rod browser plus the profile/launch that backs it.
// Close and Terminate both fully tear the session down (we own the browser);
// Close stops the M5 re-injection loop, closes the CDP connection, and closes
// the underlying launch (kills Chrome + tears down any forward proxy).
type Session struct {
	Browser   *rod.Browser
	Page      *rod.Page
	Profile   *StoredProfile
	Launch    *LaunchResult
	Close     func() error
	Terminate func() error
	ID        string
	Temporary bool
}

// QuickLaunchOptions configures QuickLaunch (mirrors the TS quickLaunch options).
type QuickLaunchOptions struct {
	Name        string
	Proxy       *ProxyConfig
	Timezone    string
	Fingerprint *FingerprintConfig
	StoragePath string
	Headless    bool
}

// CreateSessionOptions configures CreateSession. Temporary and RandomFingerprint
// are *bool so nil means the TS default of true.
type CreateSessionOptions struct {
	Temporary         *bool // default true; false => error (persistent not implemented)
	RandomFingerprint *bool // default true
	Proxy             *ProxyConfig
	Timezone          string
	Fingerprint       *FingerprintConfig
	Headless          bool
	ChromePath        string
	Args              []string
}

// PatchPageOptions configures PatchPage. Toggles are *bool so nil means the TS
// default of true. WebGL is a deliberately dead field (declared for TS parity,
// never injected: PatchPage's subset is navigator + webrtc + automation-bypass).
type PatchPageOptions struct {
	Webdriver   *bool // default true
	Languages   []string
	Plugins     *bool // default true
	WebGL       *bool // default true, DEAD (unused, kept for parity)
	WebRTC      *bool // default true
	Chrome      *bool // default true
	Fingerprint *FingerprintConfig
}

// WithProfile launches a stored profile and connects go-rod to it (= TS
// withPuppeteer). It resolves idOrName, launches via the core launcher, connects
// by ws endpoint, installs the M5 protection loop, and returns a Session whose
// Close/Terminate own the browser.
func WithProfile(bp *BrowserProfiles, idOrName string, opts LaunchOptions) (*Session, error) {
	if bp == nil {
		bp = NewBrowserProfiles(BrowserProfilesOptions{})
	}
	profile, err := bp.GetByIdOrName(idOrName)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, fmt.Errorf("Profile not found: %s", idOrName)
	}

	lr, err := bp.Launch(profile.ID, opts)
	if err != nil {
		return nil, err
	}

	sess, err := attachSession(lr.WsEndpoint, profile, lr, lr.Close, profile.ID, false)
	if err != nil {
		_ = lr.Close()
		return nil, err
	}
	return sess, nil
}

// QuickLaunch creates a real persistent profile (auto-named "Quick-<millis>" when
// Name is empty) and launches it via WithProfile. Terminate additionally deletes
// the profile when it was auto-named.
func QuickLaunch(opts QuickLaunchOptions) (*Session, error) {
	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: opts.StoragePath})

	autoNamed := opts.Name == ""
	name := opts.Name
	if name == "" {
		name = fmt.Sprintf("Quick-%d", time.Now().UnixMilli())
	}

	profile, err := bp.Create(ProfileConfig{
		Name:        name,
		Proxy:       opts.Proxy,
		Timezone:    opts.Timezone,
		Fingerprint: opts.Fingerprint,
	})
	if err != nil {
		return nil, err
	}

	sess, err := WithProfile(bp, profile.ID, LaunchOptions{Headless: opts.Headless})
	if err != nil {
		return nil, err
	}

	if autoNamed {
		baseClose := sess.Close
		sess.Terminate = func() error {
			err := baseClose()
			_, _ = bp.Delete(profile.ID)
			return err
		}
	}
	return sess, nil
}

// CreateSession is a lightweight temporary session with a random fingerprint.
//
// [DIVERGENCE] Unlike the TS createSession (which builds its own puppeteer.launch
// path), this UNIFIES with the core launcher: it generates a full fingerprint via
// fingerprint.GenerateFingerprint, maps it onto a FingerprintConfig, and launches
// through LaunchChromeStandalone (temp user-data-dir) so proxy auth flows through
// the same authenticated forward proxy as every other launch. Temporary defaults
// true; Temporary=false returns an error (persistent sessions are deferred).
func CreateSession(opts CreateSessionOptions) (*Session, error) {
	temporary := opts.Temporary == nil || *opts.Temporary
	if !temporary {
		return nil, fmt.Errorf("persistent createSession not implemented")
	}
	random := opts.RandomFingerprint == nil || *opts.RandomFingerprint

	fpc := &FingerprintConfig{Language: "en-US", Platform: "Win32", HardwareConcurrency: 8, DeviceMemory: 8}
	if random {
		gen := fingerprint.GenerateFingerprint(fingerprint.GenerateFingerprintOptions{})
		fpc = &FingerprintConfig{
			UserAgent:           gen.UserAgent,
			Language:            gen.Language,
			Platform:            gen.Platform,
			HardwareConcurrency: gen.HardwareConcurrency,
			DeviceMemory:        gen.DeviceMemory,
			Screen:              &ScreenConfig{Width: gen.Screen.Width, Height: gen.Screen.Height, DeviceScaleFactor: float64(gen.Screen.DevicePixelRatio)},
			WebGL:               &WebGLConfig{Vendor: gen.WebGL.Vendor, Renderer: gen.WebGL.Renderer},
		}
	}
	// Explicit fingerprint fields win over the generated/default values.
	if o := opts.Fingerprint; o != nil {
		if o.UserAgent != "" {
			fpc.UserAgent = o.UserAgent
		}
		if o.Language != "" {
			fpc.Language = o.Language
		}
		if o.Platform != "" {
			fpc.Platform = o.Platform
		}
		if o.HardwareConcurrency != 0 {
			fpc.HardwareConcurrency = o.HardwareConcurrency
		}
		if o.DeviceMemory != 0 {
			fpc.DeviceMemory = o.DeviceMemory
		}
	}

	sessionID := fmt.Sprintf("session-%d-%s", time.Now().UnixMilli(), randHex(3))

	standalone, err := LaunchChromeStandalone(StandaloneLaunchOptions{
		Headless:    opts.Headless,
		ChromePath:  opts.ChromePath,
		Proxy:       opts.Proxy,
		Timezone:    opts.Timezone,
		Fingerprint: fpc,
		Args:        opts.Args,
	})
	if err != nil {
		return nil, err
	}

	profile := &StoredProfile{ProfileConfig: ProfileConfig{
		ID: sessionID, Name: sessionID, Proxy: opts.Proxy, Timezone: opts.Timezone, Fingerprint: fpc,
	}}
	lr := &LaunchResult{
		WsEndpoint: standalone.WsEndpoint,
		PID:        standalone.PID,
		Port:       standalone.Port,
		ProfileID:  sessionID,
		Close:      standalone.Close,
	}

	sess, err := attachSession(standalone.WsEndpoint, profile, lr, standalone.Close, sessionID, true)
	if err != nil {
		_ = standalone.Close()
		return nil, err
	}
	return sess, nil
}

// attachSession connects go-rod to ws, installs the M5 protection loop, resolves
// the working page, and wires Close/Terminate to stop the loop + close both the
// CDP connection and the underlying launch.
func attachSession(ws string, profile *StoredProfile, lr *LaunchResult, launchClose func() error, id string, temporary bool) (*Session, error) {
	browser := rod.New().ControlURL(ws)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("connect cdp: %w", err)
	}

	stop, err := installProtections(browser, profile)
	if err != nil {
		_ = browser.Close()
		return nil, err
	}

	page, err := defaultPage(browser)
	if err != nil {
		stop()
		_ = browser.Close()
		return nil, err
	}

	closeFn := func() error {
		stop()
		_ = browser.Close()
		if launchClose != nil {
			return launchClose()
		}
		return nil
	}

	return &Session{
		Browser:   browser,
		Page:      page,
		Profile:   profile,
		Launch:    lr,
		Close:     closeFn,
		Terminate: closeFn,
		ID:        id,
		Temporary: temporary,
	}, nil
}

// PatchPage applies anti-detect protections to an EXTERNAL go-rod page via
// EvalOnNewDocument. Mirrors the TS patchPage SUBSET: navigator overrides
// (always) + WebRTC leak protection + automation-detection bypass. It never
// injects canvas/webgl/audio protections; the WebGL toggle is a dead field.
// When opts.Fingerprint is provided, the WebRTC mode field ("disable" / "fake" /
// "real") is honored; an empty value falls back to the v1.0 "fake" behavior.
func PatchPage(page *rod.Page, opts PatchPageOptions) error {
	if _, err := page.EvalOnNewDocument(patchPageScript(opts)); err != nil {
		return fmt.Errorf("patch page: %w", err)
	}
	return nil
}

// patchPageScript builds the TS patchPage SUBSET script: navigator overrides (always) +
// WebRTC leak protection + automation-detection bypass. It never emits canvas/webgl/audio
// protections; the WebGL toggle is a dead field. Pure (no page) so it is unit-testable.
func patchPageScript(opts PatchPageOptions) string {
	webdriver := opts.Webdriver == nil || *opts.Webdriver
	plugins := opts.Plugins == nil || *opts.Plugins
	webrtc := opts.WebRTC == nil || *opts.WebRTC
	chromeObj := opts.Chrome == nil || *opts.Chrome
	webrtcMode := "fake"
	if fp := opts.Fingerprint; fp != nil {
		if fp.WebRTC != "" {
			webrtcMode = fp.WebRTC
		}
	}
	_ = opts.WebGL // DEAD: declared for TS parity, never injected (see doc comment).

	language := ""
	platform := "Win32"
	hw, mem := 8, 8
	if fp := opts.Fingerprint; fp != nil {
		language = fp.Language
		if fp.Platform != "" {
			platform = fp.Platform
		}
		if fp.HardwareConcurrency != 0 {
			hw = fp.HardwareConcurrency
		}
		if fp.DeviceMemory != 0 {
			mem = fp.DeviceMemory
		}
	}
	if language == "" && len(opts.Languages) > 0 {
		language = opts.Languages[0]
	}
	if language == "" {
		language = "en-US"
	}

	parts := []string{
		fingerprint.CreateNavigatorScript(fingerprint.NavigatorConfig{
			Language: language, Platform: platform, HardwareConcurrency: hw, DeviceMemory: mem,
		}),
	}
	if webrtc {
		parts = append(parts, fingerprint.CreateWebRTCProtectionScript(webrtcMode))
	}
	if webdriver || chromeObj || plugins {
		parts = append(parts, fingerprint.AutomationBypassScript)
	}
	return strings.Join(parts, "\n\n")
}

// protectionBundle builds the full protection script (navigator override derived
// from the profile fingerprint + all protections + automation bypass), matching
// applyAntiDetect's first-page injection so re-injected tabs are protected
// identically.
func protectionBundle(profile *StoredProfile) string {
	platform := "Win32"
	language := "en-US"
	hw, mem := 8, 8
	var webglCfg *fingerprint.WebGLScriptConfig
	webrtcMode, canvasMode, audioMode := "fake", "noise", "noise"
	if profile != nil && profile.Fingerprint != nil {
		fp := profile.Fingerprint
		if fp.Platform != "" {
			platform = fp.Platform
		}
		if fp.Language != "" {
			language = fp.Language
		}
		if fp.HardwareConcurrency != 0 {
			hw = fp.HardwareConcurrency
		}
		if fp.DeviceMemory != 0 {
			mem = fp.DeviceMemory
		}
		if fp.WebGL != nil {
			webglCfg = &fingerprint.WebGLScriptConfig{Vendor: fp.WebGL.Vendor, Renderer: fp.WebGL.Renderer}
		}
		if fp.WebRTC != "" {
			webrtcMode = fp.WebRTC
		}
		if fp.Canvas != "" {
			canvasMode = fp.Canvas
		}
		if fp.Audio != "" {
			audioMode = fp.Audio
		}
	}
	return fingerprint.GetAllProtectionScripts(&fingerprint.AllProtectionOptions{
		Navigator: &fingerprint.NavigatorConfig{
			Language: language, Platform: platform, HardwareConcurrency: hw, DeviceMemory: mem,
		},
		WebGLConfig: webglCfg,
		WebRTCMode:  webrtcMode,
		CanvasMode:  canvasMode,
		AudioMode:   audioMode,
	})
}

// installProtections is the M5 fix: it injects the protection bundle into every
// currently-open page, enables CDP target discovery, and starts a browser-level
// EachEvent(TargetTargetCreated) loop that re-injects into each newly created
// page. It returns a stop func (cancels the loop) which the Session Close/
// Terminate closures invoke. It does NOT block the caller.
func installProtections(browser *rod.Browser, profile *StoredProfile) (func(), error) {
	bundle := protectionBundle(profile)

	// Cover pages that already exist (the launch target is protected here, and
	// on-new-document injection persists for its subsequent navigations).
	if pages, err := browser.Pages(); err == nil {
		for _, p := range pages {
			_, _ = p.EvalOnNewDocument(bundle)
		}
	}

	// Target discovery must be enabled explicitly: the Target domain has no
	// .enable, so EachEvent cannot auto-enable it, and without discovery
	// targetCreated never fires for new tabs.
	if err := (proto.TargetSetDiscoverTargets{Discover: true}).Call(browser); err != nil {
		return nil, fmt.Errorf("enable target discovery: %w", err)
	}

	watch, cancel := browser.WithCancel()
	wait := watch.EachEvent(func(e *proto.TargetTargetCreated) {
		if e.TargetInfo == nil || e.TargetInfo.Type != proto.TargetTargetInfoTypePage {
			return
		}
		p, err := watch.PageFromTarget(e.TargetInfo.TargetID)
		if err != nil {
			return
		}
		_, _ = p.EvalOnNewDocument(bundle)
	})
	go wait()

	return cancel, nil
}
