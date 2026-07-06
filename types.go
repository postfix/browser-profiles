// Package browserprofiles is a self-hosted anti-detect browser-profile manager:
// an open-source AdsPower/Multilogin alternative, driven by go-rod. It is a faithful
// Go port of the TypeScript library @aitofy/browser-profiles v0.2.12.
package browserprofiles

import (
	"bytes"
	"strconv"
	"time"
)

// VERSION is the library version (ported from @aitofy/browser-profiles).
const VERSION = "0.2.12"

// Port mirrors the TS ProxyConfig.port `number | string`: it unmarshals from a JSON
// number OR a JSON string and always marshals as a JSON number.
type Port int

// UnmarshalJSON accepts a JSON number or a quoted numeric string.
func (p *Port) UnmarshalJSON(b []byte) error {
	b = bytes.Trim(b, `"`)
	if len(b) == 0 {
		*p = 0
		return nil
	}
	n, err := strconv.Atoi(string(b))
	if err != nil {
		return err
	}
	*p = Port(n)
	return nil
}

// String renders the port as a decimal string.
func (p Port) String() string { return strconv.Itoa(int(p)) }

// BrowserErrorCode enumerates programmatic error codes (mirrors the TS union).
//
// NOTE: like the TS source, these codes are not wired into throw/return sites —
// BrowserError is an inert, exported type. Error surfaces are plain errors whose
// message strings are the observable contract (the CLI prints err.Error()).
type BrowserErrorCode string

const (
	ErrChromeNotFound  BrowserErrorCode = "CHROME_NOT_FOUND"
	ErrLaunchFailed    BrowserErrorCode = "LAUNCH_FAILED"
	ErrProfileNotFound BrowserErrorCode = "PROFILE_NOT_FOUND"
	ErrProxyError      BrowserErrorCode = "PROXY_ERROR"
	ErrNetwork         BrowserErrorCode = "NETWORK"
	ErrTimeout         BrowserErrorCode = "TIMEOUT"
	ErrCDPError        BrowserErrorCode = "CDP_ERROR"
	ErrInvalidConfig   BrowserErrorCode = "INVALID_CONFIG"
	ErrStorageError    BrowserErrorCode = "STORAGE_ERROR"
	ErrGeoLookupFailed BrowserErrorCode = "GEO_LOOKUP_FAILED"
)

// BrowserError is a structured error for browser operations. Inert/optional
// (see BrowserErrorCode note); implements error and errors.Unwrap.
type BrowserError struct {
	Code      BrowserErrorCode `json:"code"`
	Message   string           `json:"message"`
	Cause     error            `json:"-"`
	ProfileID string           `json:"profileId,omitempty"`
}

func (e *BrowserError) Error() string { return e.Message }
func (e *BrowserError) Unwrap() error { return e.Cause }

// ProxyConfig is a proxy configuration for a browser profile.
type ProxyConfig struct {
	Type     string `json:"type"` // "http" | "https" | "socks5"
	Host     string `json:"host"`
	Port     Port   `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ProfileCookie is a cookie to inject into a browser profile.
type ProfileCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path,omitempty"`
	HTTPOnly bool   `json:"httpOnly,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
	SameSite string `json:"sameSite,omitempty"` // "Strict" | "Lax" | "None"
	Expires  int64  `json:"expires,omitempty"`
}

// ScreenConfig is a screen-resolution configuration.
type ScreenConfig struct {
	Width             int     `json:"width"`
	Height            int     `json:"height"`
	DeviceScaleFactor float64 `json:"deviceScaleFactor,omitempty"`
}

// Brand is a User-Agent Client Hints brand/version pair.
type Brand struct {
	Brand   string `json:"brand"`
	Version string `json:"version"`
}

// NavigatorConnection configures the navigator.connection coherence object.
type NavigatorConnection struct {
	EffectiveType string  `json:"effectiveType,omitempty"`
	Downlink      float64 `json:"downlink,omitempty"`
	Rtt           int     `json:"rtt,omitempty"`
	SaveData      bool    `json:"saveData,omitempty"`
}

// ClientHintsConfig carries the User-Agent Client Hints metadata for a profile.
type ClientHintsConfig struct {
	Platform        string  `json:"platform,omitempty"`
	PlatformVersion string  `json:"platformVersion,omitempty"`
	Architecture    string  `json:"architecture,omitempty"`
	Model           string  `json:"model,omitempty"`
	Mobile          bool    `json:"mobile,omitempty"`
	Brands          []Brand `json:"brands,omitempty"`
	FullVersion     string  `json:"fullVersion,omitempty"`
}

// WebGLCaps are stable numeric WebGL parameter values chosen per GPU family.
type WebGLCaps struct {
	MaxTextureSize             int       `json:"maxTextureSize,omitempty"`
	MaxCubeMapTextureSize      int       `json:"maxCubeMapTextureSize,omitempty"`
	MaxRenderbufferSize        int       `json:"maxRenderbufferSize,omitempty"`
	MaxVaryingVectors          int       `json:"maxVaryingVectors,omitempty"`
	MaxVertexUniformVectors    int       `json:"maxVertexUniformVectors,omitempty"`
	MaxViewportDims            []int     `json:"maxViewportDims,omitempty"`
	AliasedLineWidthRange      []float64 `json:"aliasedLineWidthRange,omitempty"`
	AliasedPointSizeRange      []float64 `json:"aliasedPointSizeRange,omitempty"`
	MaxTextureImageUnits       int       `json:"maxTextureImageUnits,omitempty"`
	MaxVertexTextureImageUnits int       `json:"maxVertexTextureImageUnits,omitempty"`
	MaxCombinedTextureImageUnits int     `json:"maxCombinedTextureImageUnits,omitempty"`
	MaxFragmentUniformVectors  int       `json:"maxFragmentUniformVectors,omitempty"`
	MaxVertexAttribs           int       `json:"maxVertexAttribs,omitempty"`
}

// PluginMimeType describes a single MIME type exposed by a plugin.
type PluginMimeType struct {
	Type          string `json:"type"`
	Description   string `json:"description,omitempty"`
	Suffixes      string `json:"suffixes,omitempty"`
	EnabledPlugin string `json:"enabledPlugin,omitempty"`
}

// PluginInfo describes a single browser plugin entry.
type PluginInfo struct {
	Name        string           `json:"name"`
	Filename    string           `json:"filename"`
	Description string           `json:"description,omitempty"`
	Version     string           `json:"version,omitempty"`
	MimeTypes   []PluginMimeType `json:"mimeTypes,omitempty"`
}

// PluginsConfig carries the platform-specific plugin and MIME-type lists.
type PluginsConfig struct {
	Plugins   []PluginInfo     `json:"plugins,omitempty"`
	MimeTypes []PluginMimeType `json:"mimeTypes,omitempty"`
}

// FontsConfig carries a whitelist of font families that document.fonts.check will
// report as installed. This is a lightweight guard, not a full font spoof: real font
// presence is an OS-level property, so the guard only prevents naive fingerprinters
// from observing the absence of common system fonts.
type FontsConfig struct {
	Whitelist []string `json:"whitelist,omitempty"`
}

// PermissionsConfig carries coherent permission states for the four query names
// most commonly inspected by fingerprinters.
type PermissionsConfig struct {
	Camera        string `json:"camera,omitempty"`
	Microphone    string `json:"microphone,omitempty"`
	Geolocation   string `json:"geolocation,omitempty"`
	Notifications string `json:"notifications,omitempty"`
}

// (F4 resolved): the launcher's WebGL protection (CreateWebGLScript / webgl.tmpl.js) spoofs
// UNMASKED_VENDOR_WEBGL → Vendor and UNMASKED_RENDERER_WEBGL → Renderer and guarantees the
// WEBGL_debug_renderer_info extension is present, so a fingerprinter reads this profile's GPU
// identity. Empty fields fall back to generic defaults. The masked VENDOR/RENDERER (7936/7937)
// stay "WebKit"/"WebKit WebGL" like real Chrome. [DIVERGENCE] the TS reference never wired this.
type WebGLConfig struct {
	Vendor   string     `json:"vendor,omitempty"`
	Renderer string     `json:"renderer,omitempty"`
	Caps     *WebGLCaps `json:"caps,omitempty"`
}

// WebGPUConfig carries the GPU adapter identity returned by navigator.gpu.requestAdapter.
// It mirrors the fingerprint package builder type so JSON encoding omits empty fields.
type WebGPUConfig struct {
	Vendor       string `json:"vendor,omitempty"`
	Architecture string `json:"architecture,omitempty"`
	Device       string `json:"device,omitempty"`
	Description  string `json:"description,omitempty"`
}

// TimingConfig gates optional rounding of performance.now() and Date.now().
// When Enabled is false (the default) the timing script is a no-op. Precision
// is the rounding quantum, e.g. 1ms or 100µs.
type TimingConfig struct {
	Enabled   bool          `json:"enabled,omitempty"`
	Precision time.Duration `json:"precision,omitempty"`
}

// FingerprintConfig is the anti-detect fingerprint configuration.
type FingerprintConfig struct {
	UserAgent           string             `json:"userAgent,omitempty"`
	Language            string             `json:"language,omitempty"`
	Screen              *ScreenConfig      `json:"screen,omitempty"`
	WebGL               *WebGLConfig       `json:"webgl,omitempty"`
	WebGPU              *WebGPUConfig      `json:"webgpu,omitempty"`
	Timing              *TimingConfig      `json:"timing,omitempty"`
	Platform            string             `json:"platform,omitempty"`
	HardwareConcurrency int                `json:"hardwareConcurrency,omitempty"`
	DeviceMemory        int                `json:"deviceMemory,omitempty"`
	WebRTC              string             `json:"webrtc,omitempty"` // "disable" | "fake" | "real"
	Canvas              string             `json:"canvas,omitempty"` // "noise" | "real"
	Audio               string             `json:"audio,omitempty"`  // "noise" | "real"
	AppVersion          string             `json:"appVersion,omitempty"`
	ProductSub          string             `json:"productSub,omitempty"`
	Vendor              string             `json:"vendor,omitempty"`
	MaxTouchPoints      int                `json:"maxTouchPoints,omitempty"`
	Mobile              bool               `json:"mobile,omitempty"`
	Connection          *NavigatorConnection `json:"connection,omitempty"`
	ClientHints         *ClientHintsConfig   `json:"clientHints,omitempty"`
	Permissions         *PermissionsConfig    `json:"permissions,omitempty"`
	Plugins             *PluginsConfig        `json:"plugins,omitempty"`
	Fonts               *FontsConfig          `json:"fonts,omitempty"`
	CPUThrottlingRate   float64               `json:"cpuThrottlingRate,omitempty"`
}

// ProfileConfig is the input configuration for a browser profile.
type ProfileConfig struct {
	ID          string             `json:"id,omitempty"`
	Name        string             `json:"name"`
	Proxy       *ProxyConfig       `json:"proxy"` // nil serializes as null (TS default)
	Timezone    string             `json:"timezone,omitempty"`
	Cookies     []ProfileCookie    `json:"cookies"`
	Fingerprint *FingerprintConfig `json:"fingerprint"`
	StartURLs   []string           `json:"startUrls"`
	Notes       string             `json:"notes,omitempty"`
	GroupID     string             `json:"groupId,omitempty"`
	Tags        []string           `json:"tags"`
}

// StoredProfile is a persisted profile with metadata (embeds ProfileConfig, whose
// fields flatten into the same JSON object, mirroring the TS `extends`).
type StoredProfile struct {
	ProfileConfig
	CreatedAt      int64 `json:"createdAt"`
	UpdatedAt      int64 `json:"updatedAt"`
	LastLaunchedAt int64 `json:"lastLaunchedAt,omitempty"`
}

// Viewport is a browser viewport size.
type Viewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// LaunchOptions are options for launching a browser.
type LaunchOptions struct {
	Headless        bool      `json:"headless,omitempty"`
	ChromePath      string    `json:"chromePath,omitempty"`
	Args            []string  `json:"args,omitempty"`
	Extensions      []string  `json:"extensions,omitempty"`
	DefaultViewport *Viewport `json:"defaultViewport,omitempty"`
	SlowMo          int       `json:"slowMo,omitempty"`
	Timeout         int       `json:"timeout,omitempty"`
}

// BrowserProfilesOptions configures a BrowserProfiles manager.
type BrowserProfilesOptions struct {
	StoragePath     string       `json:"storagePath,omitempty"`
	ChromePath      string       `json:"chromePath,omitempty"`
	DefaultTimezone string       `json:"defaultTimezone,omitempty"`
	DefaultProxy    *ProxyConfig `json:"defaultProxy,omitempty"`
}

// LaunchResult is the result of launching a profile's browser.
type LaunchResult struct {
	WsEndpoint string       `json:"wsEndpoint"`
	PID        int          `json:"pid"`
	Port       int          `json:"port"`
	ProfileID  string       `json:"profileId"`
	Close      func() error `json:"-"`
}

// ProfileGroup organizes profiles.
type ProfileGroup struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	ProfileCount int    `json:"profileCount,omitempty"`
}
