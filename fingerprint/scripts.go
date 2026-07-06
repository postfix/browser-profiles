// Package fingerprint holds the anti-detect browser-side JavaScript (ported verbatim
// from @aitofy/browser-profiles' fingerprint.ts) plus a Go fingerprint generator.
//
// The protection scripts and builder templates are embedded byte-for-byte from the
// TypeScript reference (see scripts/*.js, extracted via the reference implementation),
// so the injected JavaScript is identical to the original. Builders substitute the
// dynamic parts using JSON encoded with SetEscapeHTML(false) to match JS JSON.stringify.
package fingerprint

import (
	"bytes"
	"embed"
	"encoding/json"
	"strconv"
	"strings"
)

//go:embed scripts/*.js
var scriptFS embed.FS

func mustRead(name string) string {
	b, err := scriptFS.ReadFile("scripts/" + name)
	if err != nil {
		panic("fingerprint: embedded script missing: " + name)
	}
	return string(b)
}

// Protection scripts (browser-injected JS), byte-identical to the TS constants.
var (
	WebRTCProtectionScript = mustRead("webrtc.js")
	CanvasProtectionScript = mustRead("canvas.js")
	WebGLProtectionScript  = mustRead("webgl.js")
	AudioProtectionScript  = mustRead("audio.js")
	AutomationBypassScript = mustRead("automation_bypass.js")
	navigatorTmpl          = mustRead("navigator.tmpl.js")
	screenTmpl             = mustRead("screen.tmpl.js")
	clientHintsTmpl        = mustRead("clienthints.tmpl.js")
	webglTmpl              = mustRead("webgl.tmpl.js")
)

// marshalNoEscape mirrors JS JSON.stringify: compact, no HTML escaping of < > &.
func marshalNoEscape(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimRight(b.String(), "\n")
}

// NavigatorConfig mirrors the TS createNavigatorScript config. Field order matches the
// TS type declaration (userAgent, language, platform, hardwareConcurrency, deviceMemory,
// vendor) so JSON.stringify parity holds for the library's own call sites.
type NavigatorConfig struct {
	UserAgent           string `json:"userAgent,omitempty"`
	Language            string `json:"language,omitempty"`
	Platform            string `json:"platform,omitempty"`
	HardwareConcurrency int    `json:"hardwareConcurrency,omitempty"`
	DeviceMemory        int    `json:"deviceMemory,omitempty"`
	Vendor              string `json:"vendor,omitempty"`
}

// CreateNavigatorScript ports createNavigatorScript: embeds JSON.stringify(config).
func CreateNavigatorScript(c NavigatorConfig) string {
	return strings.ReplaceAll(navigatorTmpl, "%%NAVJSON%%", marshalNoEscape(c))
}

// ScreenScriptConfig mirrors the TS createScreenScript config.
type ScreenScriptConfig struct {
	Width            int
	Height           int
	AvailWidth       int
	AvailHeight      int
	ColorDepth       int
	PixelDepth       int
	DevicePixelRatio int
}

// CreateScreenScript ports createScreenScript, including its `|| default` fallbacks
// (0 is treated as "unset", matching JS falsy-number semantics).
func CreateScreenScript(c ScreenScriptConfig) string {
	width := c.Width
	if width == 0 {
		width = 1920
	}
	height := c.Height
	if height == 0 {
		height = 1080
	}
	availWidth := c.AvailWidth
	if availWidth == 0 {
		availWidth = width
	}
	availHeight := c.AvailHeight
	if availHeight == 0 {
		availHeight = height - 40
	}
	colorDepth := c.ColorDepth
	if colorDepth == 0 {
		colorDepth = 24
	}
	pixelDepth := c.PixelDepth
	if pixelDepth == 0 {
		pixelDepth = 24
	}
	dpr := c.DevicePixelRatio
	if dpr == 0 {
		dpr = 1
	}
	return strings.NewReplacer(
		"%%WIDTH%%", strconv.Itoa(width),
		"%%HEIGHT%%", strconv.Itoa(height),
		"%%AVAILW%%", strconv.Itoa(availWidth),
		"%%AVAILH%%", strconv.Itoa(availHeight),
		"%%COLORDEPTH%%", strconv.Itoa(colorDepth),
		"%%PIXELDEPTH%%", strconv.Itoa(pixelDepth),
		"%%DPR%%", strconv.Itoa(dpr),
	).Replace(screenTmpl)
}

// Brand is a Client Hints brand/version pair.
type Brand struct {
	Brand   string `json:"brand"`
	Version string `json:"version"`
}

// ClientHintsScriptConfig mirrors the TS createClientHintsScript config.
type ClientHintsScriptConfig struct {
	Platform        string
	PlatformVersion string
	Architecture    string
	Model           string
	Mobile          bool
	Brands          []Brand
}

var defaultBrands = []Brand{
	{Brand: "Chromium", Version: "120"},
	{Brand: "Google Chrome", Version: "120"},
	{Brand: "Not_A Brand", Version: "8"},
}

// CreateClientHintsScript ports createClientHintsScript with its `|| default` fallbacks.
func CreateClientHintsScript(c ClientHintsScriptConfig) string {
	platform := c.Platform
	if platform == "" {
		platform = "Windows"
	}
	pver := c.PlatformVersion
	if pver == "" {
		pver = "10.0.0"
	}
	arch := c.Architecture
	if arch == "" {
		arch = "x86"
	}
	brands := c.Brands
	if len(brands) == 0 {
		brands = defaultBrands
	}
	return strings.NewReplacer(
		"%%BRANDSJSON%%", marshalNoEscape(brands),
		"%%PLATFORM%%", platform,
		"%%PVER%%", pver,
		"%%ARCH%%", arch,
		"%%MODEL%%", c.Model, // TS: config.model || '' ; '' stays ''
		"%%MOBILE%%", strconv.FormatBool(c.Mobile),
	).Replace(clientHintsTmpl)
}

// WebGLScriptConfig configures the per-profile WebGL spoof used by CreateWebGLScript.
type WebGLScriptConfig struct {
	Vendor   string
	Renderer string
}

// Static default expressions for the parameterized WebGL script when a field is unset —
// these match the verbatim webgl.js so an all-empty config keeps the original behavior.
const (
	defaultWebGLVendorExpr   = `"Google Inc."`
	defaultWebGLRendererExpr = `randomItem(["ANGLE (Intel, Intel(R) HD Graphics)", "ANGLE (NVIDIA, GeForce GTX 1080)", "ANGLE (AMD, Radeon RX 580)"])`
)

// CreateWebGLScript builds a WebGL protection script that spoofs UNMASKED_VENDOR_WEBGL /
// UNMASKED_RENDERER_WEBGL to the profile's configured values (per-profile GPU identity) and
// guarantees the WEBGL_debug_renderer_info extension is present so fingerprinters read those
// values rather than the generic masked VENDOR/RENDERER. Empty fields fall back to the static
// defaults. [DIVERGENCE] the TS reference never parameterized WebGL — a conscious enhancement.
func CreateWebGLScript(c WebGLScriptConfig) string {
	vendor := defaultWebGLVendorExpr
	if c.Vendor != "" {
		vendor = marshalNoEscape(c.Vendor)
	}
	renderer := defaultWebGLRendererExpr
	if c.Renderer != "" {
		renderer = marshalNoEscape(c.Renderer)
	}
	s := strings.ReplaceAll(webglTmpl, "%%WEBGL_VENDOR%%", vendor)
	return strings.ReplaceAll(s, "%%WEBGL_RENDERER%%", renderer)
}

// AllProtectionOptions mirrors the TS getAllProtectionScripts options. A nil *bool
// means "enabled" (TS defaults every protection to true).
type AllProtectionOptions struct {
	WebRTC      *bool
	Canvas      *bool
	WebGL       *bool
	Audio       *bool
	Navigator   *NavigatorConfig
	WebGLConfig *WebGLScriptConfig // per-profile UNMASKED vendor/renderer; nil ⇒ verbatim webgl.js
}

func enabled(p *bool) bool { return p == nil || *p }

// GetAllProtectionScripts ports getAllProtectionScripts. Pass nil for all defaults.
func GetAllProtectionScripts(o *AllProtectionOptions) string {
	if o == nil {
		o = &AllProtectionOptions{}
	}
	var scripts []string
	if enabled(o.WebRTC) {
		scripts = append(scripts, WebRTCProtectionScript)
	}
	if enabled(o.Canvas) {
		scripts = append(scripts, CanvasProtectionScript)
	}
	if enabled(o.WebGL) {
		if o.WebGLConfig != nil && (o.WebGLConfig.Vendor != "" || o.WebGLConfig.Renderer != "") {
			scripts = append(scripts, CreateWebGLScript(*o.WebGLConfig))
		} else {
			scripts = append(scripts, WebGLProtectionScript)
		}
	}
	if enabled(o.Audio) {
		scripts = append(scripts, AudioProtectionScript)
	}
	if o.Navigator != nil {
		scripts = append(scripts, CreateNavigatorScript(*o.Navigator))
	}
	// Always add automation detection bypass.
	scripts = append(scripts, AutomationBypassScript)
	return strings.Join(scripts, "\n\n")
}

// GetFingerprintScripts ports getFingerprintScripts (navigator, screen, client-hints,
// then all protection scripts + automation bypass).
func GetFingerprintScripts(fp GeneratedFingerprint) string {
	scripts := []string{
		CreateNavigatorScript(NavigatorConfig{
			Language:            fp.Language,
			Platform:            fp.Platform,
			HardwareConcurrency: fp.HardwareConcurrency,
			DeviceMemory:        fp.DeviceMemory,
			Vendor:              fp.Vendor,
		}),
		CreateScreenScript(ScreenScriptConfig{
			Width:            fp.Screen.Width,
			Height:           fp.Screen.Height,
			AvailWidth:       fp.Screen.AvailWidth,
			AvailHeight:      fp.Screen.AvailHeight,
			ColorDepth:       fp.Screen.ColorDepth,
			PixelDepth:       fp.Screen.PixelDepth,
			DevicePixelRatio: fp.Screen.DevicePixelRatio,
		}),
		CreateClientHintsScript(ClientHintsScriptConfig{
			Platform:        fp.ClientHints.Platform,
			PlatformVersion: fp.ClientHints.PlatformVersion,
			Architecture:    fp.ClientHints.Architecture,
			Mobile:          fp.ClientHints.Mobile,
			Brands:          fp.ClientHints.Brands,
		}),
		WebRTCProtectionScript,
		CanvasProtectionScript,
		WebGLProtectionScript,
		AudioProtectionScript,
		AutomationBypassScript,
	}
	return strings.Join(scripts, "\n\n")
}
