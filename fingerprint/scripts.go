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
	WebRTCProtectionScript        = mustRead("webrtc-fake.js") // default = fake
	WebRTCProtectionDisableScript = mustRead("webrtc-disable.js")
	WebRTCProtectionRealScript    = mustRead("webrtc-real.js") // intentionally empty

	CanvasProtectionScript     = mustRead("canvas-noise.js") // default = noise
	CanvasProtectionRealScript = mustRead("canvas-real.js")  // intentionally empty

	AudioProtectionScript     = mustRead("audio-noise.js") // default = noise
	AudioProtectionRealScript = mustRead("audio-real.js")  // intentionally empty

	WebGLProtectionScript  = mustRead("webgl.js")
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
	UserAgent           string             `json:"userAgent,omitempty"`
	Language            string             `json:"language,omitempty"`
	Platform            string             `json:"platform,omitempty"`
	HardwareConcurrency int                `json:"hardwareConcurrency,omitempty"`
	DeviceMemory        int                `json:"deviceMemory,omitempty"`
	Vendor              string             `json:"vendor,omitempty"`
	AppVersion          string             `json:"appVersion,omitempty"`
	ProductSub          string             `json:"productSub,omitempty"`
	MaxTouchPoints      int                `json:"maxTouchPoints,omitempty"`
	Mobile              bool               `json:"mobile,omitempty"`
	Connection          *NavigatorConnection `json:"connection,omitempty"`
}

type NavigatorConnection struct {
	EffectiveType string  `json:"effectiveType,omitempty"`
	Downlink      float64 `json:"downlink,omitempty"`
	Rtt           int     `json:"rtt,omitempty"`
	SaveData      bool    `json:"saveData,omitempty"`
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
	FullVersion     string
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
	fullVersion := c.FullVersion
	if fullVersion == "" {
		fullVersion = "120.0.6099.71"
	}
	return strings.NewReplacer(
		"%%BRANDSJSON%%", marshalNoEscape(brands),
		"%%PLATFORM%%", platform,
		"%%PVER%%", pver,
		"%%ARCH%%", arch,
		"%%MODEL%%", c.Model, // TS: config.model || '' ; '' stays ''
		"%%MOBILE%%", strconv.FormatBool(c.Mobile),
		"%%UA_FULL_VERSION%%", fullVersion,
	).Replace(clientHintsTmpl)
}

type WebGLCaps struct {
	MaxTextureSize             int
	MaxCubeMapTextureSize      int
	MaxRenderbufferSize        int
	MaxVaryingVectors          int
	MaxVertexUniformVectors    int
	MaxViewportDims            [2]int
	AliasedLineWidthRange      [2]float64
	AliasedPointSizeRange      [2]float64
	MaxTextureImageUnits       int
	MaxVertexTextureImageUnits int
	MaxCombinedTextureImageUnits int
	MaxFragmentUniformVectors  int
	MaxVertexAttribs           int
}

// WebGLScriptConfig configures the per-profile WebGL spoof used by CreateWebGLScript.
type WebGLScriptConfig struct {
	Vendor   string
	Renderer string
	Caps     *WebGLCaps
}

// Static default expressions for the parameterized WebGL script when a field is unset —
// these match the verbatim webgl.js so an all-empty config keeps the original behavior.
const (
	defaultWebGLVendorExpr   = `"Google Inc."`
	defaultWebGLRendererExpr = `randomItem(["ANGLE (Intel, Intel(R) HD Graphics)", "ANGLE (NVIDIA, GeForce GTX 1080)", "ANGLE (AMD, Radeon RX 580)"])`
)

// defaultWebGLCapsExprs are the randomized fallback expressions used when Caps is nil.
const (
	defaultMaxTextureSizeExpr             = `randomPower([14, 15])`
	defaultMaxCubeMapTextureSizeExpr      = `randomPower([14, 15])`
	defaultMaxRenderbufferSizeExpr        = `randomPower([14, 15])`
	defaultMaxVaryingVectorsExpr          = `randomPower([12, 13])`
	defaultMaxVertexUniformVectorsExpr    = `30`
	defaultMaxViewportDimsExpr            = `randomInt32([13, 14, 15])`
	defaultAliasedLineWidthRangeExpr      = `randomFloat32([0, 10, 11, 12, 13])`
	defaultAliasedPointSizeRangeExpr      = `randomFloat32([0, 10, 11, 12, 13])`
	defaultMaxTextureImageUnitsExpr       = `randomPower([1, 2, 3, 4])`
	defaultMaxVertexTextureImageUnitsExpr = `randomPower([1, 2, 3, 4])`
	defaultMaxCombinedTextureImageUnitsExpr = `randomPower([4, 5, 6, 7, 8])`
	defaultMaxFragmentUniformVectorsExpr  = `randomPower([1, 2, 3, 4])`
	defaultMaxVertexAttribsExpr           = `randomPower([10, 11, 12, 13])`
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
	maxTextureSize := defaultMaxTextureSizeExpr
	maxCubeMapTextureSize := defaultMaxCubeMapTextureSizeExpr
	maxRenderbufferSize := defaultMaxRenderbufferSizeExpr
	maxVaryingVectors := defaultMaxVaryingVectorsExpr
	maxVertexUniformVectors := defaultMaxVertexUniformVectorsExpr
	maxViewportDims := defaultMaxViewportDimsExpr
	aliasedLineWidthRange := defaultAliasedLineWidthRangeExpr
	aliasedPointSizeRange := defaultAliasedPointSizeRangeExpr
	maxTextureImageUnits := defaultMaxTextureImageUnitsExpr
	maxVertexTextureImageUnits := defaultMaxVertexTextureImageUnitsExpr
	maxCombinedTextureImageUnits := defaultMaxCombinedTextureImageUnitsExpr
	maxFragmentUniformVectors := defaultMaxFragmentUniformVectorsExpr
	maxVertexAttribs := defaultMaxVertexAttribsExpr
	if c.Caps != nil {
		maxTextureSize = strconv.Itoa(c.Caps.MaxTextureSize)
		maxCubeMapTextureSize = strconv.Itoa(c.Caps.MaxCubeMapTextureSize)
		maxRenderbufferSize = strconv.Itoa(c.Caps.MaxRenderbufferSize)
		maxVaryingVectors = strconv.Itoa(c.Caps.MaxVaryingVectors)
		maxVertexUniformVectors = strconv.Itoa(c.Caps.MaxVertexUniformVectors)
		maxViewportDims = marshalNoEscape(c.Caps.MaxViewportDims)
		aliasedLineWidthRange = marshalNoEscape(c.Caps.AliasedLineWidthRange)
		aliasedPointSizeRange = marshalNoEscape(c.Caps.AliasedPointSizeRange)
		maxTextureImageUnits = strconv.Itoa(c.Caps.MaxTextureImageUnits)
		maxVertexTextureImageUnits = strconv.Itoa(c.Caps.MaxVertexTextureImageUnits)
		maxCombinedTextureImageUnits = strconv.Itoa(c.Caps.MaxCombinedTextureImageUnits)
		maxFragmentUniformVectors = strconv.Itoa(c.Caps.MaxFragmentUniformVectors)
		maxVertexAttribs = strconv.Itoa(c.Caps.MaxVertexAttribs)
	}
	return strings.NewReplacer(
		"%%WEBGL_VENDOR%%", vendor,
		"%%WEBGL_RENDERER%%", renderer,
		"%%MAX_TEXTURE_SIZE%%", maxTextureSize,
		"%%MAX_CUBE_MAP_TEXTURE_SIZE%%", maxCubeMapTextureSize,
		"%%MAX_RENDERBUFFER_SIZE%%", maxRenderbufferSize,
		"%%MAX_VARYING_VECTORS%%", maxVaryingVectors,
		"%%MAX_VERTEX_UNIFORM_VECTORS%%", maxVertexUniformVectors,
		"%%MAX_VIEWPORT_DIMS%%", maxViewportDims,
		"%%ALIASED_LINE_WIDTH_RANGE%%", aliasedLineWidthRange,
		"%%ALIASED_POINT_SIZE_RANGE%%", aliasedPointSizeRange,
		"%%MAX_TEXTURE_IMAGE_UNITS%%", maxTextureImageUnits,
		"%%MAX_VERTEX_TEXTURE_IMAGE_UNITS%%", maxVertexTextureImageUnits,
		"%%MAX_COMBINED_TEXTURE_IMAGE_UNITS%%", maxCombinedTextureImageUnits,
		"%%MAX_FRAGMENT_UNIFORM_VECTORS%%", maxFragmentUniformVectors,
		"%%MAX_VERTEX_ATTRIBS%%", maxVertexAttribs,
	).Replace(webglTmpl)
}

// normalizeMode trims and lowercases a mode string. Empty input returns the default.
// Unrecognized values are returned as-is and must be validated by the caller.
func normalizeMode(mode, defaultMode string) string {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == "" {
		return defaultMode
	}
	return m
}

// CreateWebRTCProtectionScript returns the script for the requested WebRTC mode:
//   "disable"  -> neutralize RTCPeerConnection
//   "fake"     -> filter host/srflx candidates (v1.0 default)
//   "real"     -> empty string (no protection)
//   ""/unknown -> fall back to "fake".
func CreateWebRTCProtectionScript(mode string) string {
	switch normalizeMode(mode, "fake") {
	case "disable":
		return WebRTCProtectionDisableScript
	case "real":
		return WebRTCProtectionRealScript // empty
	default:
		return WebRTCProtectionScript
	}
}

// CreateCanvasProtectionScript returns the script for the requested Canvas mode:
//   "noise"    -> noisify getImageData/toBlob/toDataURL (v1.0 default)
//   "real"     -> empty string (no protection)
//   ""/unknown -> fall back to "noise".
func CreateCanvasProtectionScript(mode string) string {
	switch normalizeMode(mode, "noise") {
	case "real":
		return CanvasProtectionRealScript // empty
	default:
		return CanvasProtectionScript
	}
}

// CreateAudioProtectionScript returns the script for the requested Audio mode:
//   "noise"    -> add noise to AudioBuffer/AnalyserNode (v1.0 default)
//   "real"     -> empty string (no protection)
//   ""/unknown -> fall back to "noise".
func CreateAudioProtectionScript(mode string) string {
	switch normalizeMode(mode, "noise") {
	case "real":
		return AudioProtectionRealScript // empty
	default:
		return AudioProtectionScript
	}
}

// AllProtectionOptions mirrors the TS getAllProtectionScripts options. A nil *bool
// means "enabled" (TS defaults every protection to true). The WebRTCMode, CanvasMode
// and AudioMode fields select the protection variant; they are ignored when the
// corresponding *bool toggle is explicitly false.
type AllProtectionOptions struct {
	WebRTC      *bool
	Canvas      *bool
	WebGL       *bool
	Audio       *bool
	WebRTCMode  string
	CanvasMode  string
	AudioMode   string
	Navigator   *NavigatorConfig
	WebGLConfig *WebGLScriptConfig // per-profile UNMASKED vendor/renderer; nil ⇒ verbatim webgl.js
	ClientHints *ClientHintsScriptConfig
}

func enabled(p *bool) bool { return p == nil || *p }

// GetAllProtectionScripts ports getAllProtectionScripts. Pass nil for all defaults.
func GetAllProtectionScripts(o *AllProtectionOptions) string {
	if o == nil {
		o = &AllProtectionOptions{}
	}
	var scripts []string
	if enabled(o.WebRTC) {
		if s := CreateWebRTCProtectionScript(o.WebRTCMode); s != "" {
			scripts = append(scripts, s)
		}
	}
	if enabled(o.Canvas) {
		if s := CreateCanvasProtectionScript(o.CanvasMode); s != "" {
			scripts = append(scripts, s)
		}
	}
	if enabled(o.WebGL) {
		if o.WebGLConfig != nil && (o.WebGLConfig.Vendor != "" || o.WebGLConfig.Renderer != "") {
			scripts = append(scripts, CreateWebGLScript(*o.WebGLConfig))
		} else {
			scripts = append(scripts, WebGLProtectionScript)
		}
	}
	if enabled(o.Audio) {
		if s := CreateAudioProtectionScript(o.AudioMode); s != "" {
			scripts = append(scripts, s)
		}
	}
	if o.Navigator != nil {
		scripts = append(scripts, CreateNavigatorScript(*o.Navigator))
	}
	if o.ClientHints != nil {
		scripts = append(scripts, CreateClientHintsScript(*o.ClientHints))
	}
	// Always add automation detection bypass.
	scripts = append(scripts, AutomationBypassScript)
	return strings.Join(scripts, "\n\n")
}

// GetFingerprintScripts ports getFingerprintScripts (navigator, screen, client-hints,
// then all protection scripts + automation bypass). The WebRTC/Canvas/Audio mode fields
// on the generated fingerprint select which variant is injected; empty values fall back
// to the v1.0 defaults.
func GetFingerprintScripts(fp GeneratedFingerprint) string {
	scripts := []string{
		CreateNavigatorScript(NavigatorConfig{
			Language:            fp.Language,
			Platform:            fp.Platform,
			HardwareConcurrency: fp.HardwareConcurrency,
			DeviceMemory:        fp.DeviceMemory,
			Vendor:              fp.Vendor,
			AppVersion:          fp.AppVersion,
			ProductSub:          fp.ProductSub,
			MaxTouchPoints:      fp.MaxTouchPoints,
			Mobile:              fp.Mobile,
			Connection:          &fp.Connection,
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
			FullVersion:     fp.ClientHints.FullVersion,
		}),
	}
	if s := CreateWebRTCProtectionScript(fp.WebRTC); s != "" {
		scripts = append(scripts, s)
	}
	if s := CreateCanvasProtectionScript(fp.Canvas); s != "" {
		scripts = append(scripts, s)
	}
	scripts = append(scripts, WebGLProtectionScript)
	if s := CreateAudioProtectionScript(fp.Audio); s != "" {
		scripts = append(scripts, s)
	}
	scripts = append(scripts, AutomationBypassScript)
	return strings.Join(scripts, "\n\n")
}
