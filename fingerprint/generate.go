package fingerprint

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"
)

// Data tables (ported verbatim from fingerprint.ts).

var userAgents = map[string][]string{
	"windows": {
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	},
	"macos": {
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 13_6_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	},
	"linux": {
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	},
}

type resolution struct{ Width, Height int }

var screenResolutions = map[string][]resolution{
	"desktop": {{1920, 1080}, {2560, 1440}, {1366, 768}, {1536, 864}, {1440, 900}, {1680, 1050}},
	"laptop":  {{1366, 768}, {1440, 900}, {1536, 864}},
	"retina":  {{2880, 1800}, {3024, 1964}},
}

var webglRenderers = map[string][]string{
	"intel": {
		"ANGLE (Intel, Intel(R) UHD Graphics 630 Direct3D11 vs_5_0 ps_5_0)",
		"ANGLE (Intel, Intel(R) Iris(TM) Plus Graphics 640 Direct3D11 vs_5_0 ps_5_0)",
		"ANGLE (Intel, Intel(R) HD Graphics 620 Direct3D11 vs_5_0 ps_5_0)",
	},
	"nvidia": {
		"ANGLE (NVIDIA, NVIDIA GeForce GTX 1080 Direct3D11 vs_5_0 ps_5_0)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 3080 Direct3D11 vs_5_0 ps_5_0)",
		"ANGLE (NVIDIA, NVIDIA GeForce RTX 4090 Direct3D11 vs_5_0 ps_5_0)",
	},
	"amd": {
		"ANGLE (AMD, AMD Radeon RX 580 Series Direct3D11 vs_5_0 ps_5_0)",
		"ANGLE (AMD, AMD Radeon RX 6800 XT Direct3D11 vs_5_0 ps_5_0)",
	},
	"apple": {
		"ANGLE (Apple, Apple M1 Pro, OpenGL 4.1)",
		"ANGLE (Apple, Apple M2, OpenGL 4.1)",
		"ANGLE (Apple, Apple M1 Max, OpenGL 4.1)",
	},
}

var platformConfigs = map[string]struct {
	Platform, Vendor, GpuDefault string
}{
	"windows": {"Win32", "Google Inc.", "intel"},
	"macos":   {"MacIntel", "Google Inc.", "apple"},
	"linux":   {"Linux x86_64", "Google Inc.", "intel"},
}

var clientHintsPlatforms = map[string]string{
	"windows": "Windows",
	"macos":   "macOS",
	"linux":   "Linux",
}

var platformVersions = map[string][]string{
	"windows": {"10.0.0", "10.0.19045", "11.0.0"},
	"macos":   {"14.2.0", "13.6.1", "14.0.0"},
	"linux":   {"6.5.0", "5.15.0"},
}

// WebGL caps table by GPU family. Values are realistic for real GPUs; the seed
// selects one of the variants within a family so the same profile is stable while
// different profiles vary.
type webGLCapsVariant struct {
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

var webGLCapsByFamily = map[string][]webGLCapsVariant{
	"intel": {
		{
			MaxTextureSize: 16384, MaxCubeMapTextureSize: 16384, MaxRenderbufferSize: 16384,
			MaxVaryingVectors: 30, MaxVertexUniformVectors: 4096,
			MaxViewportDims: [2]int{16384, 16384},
			AliasedLineWidthRange: [2]float64{1, 1}, AliasedPointSizeRange: [2]float64{1, 255},
			MaxTextureImageUnits: 16, MaxVertexTextureImageUnits: 16, MaxCombinedTextureImageUnits: 32,
			MaxFragmentUniformVectors: 1024, MaxVertexAttribs: 16,
		},
		{
			MaxTextureSize: 8192, MaxCubeMapTextureSize: 8192, MaxRenderbufferSize: 8192,
			MaxVaryingVectors: 30, MaxVertexUniformVectors: 4096,
			MaxViewportDims: [2]int{8192, 8192},
			AliasedLineWidthRange: [2]float64{1, 1}, AliasedPointSizeRange: [2]float64{1, 255},
			MaxTextureImageUnits: 16, MaxVertexTextureImageUnits: 16, MaxCombinedTextureImageUnits: 32,
			MaxFragmentUniformVectors: 1024, MaxVertexAttribs: 16,
		},
	},
	"nvidia": {
		{
			MaxTextureSize: 32768, MaxCubeMapTextureSize: 32768, MaxRenderbufferSize: 32768,
			MaxVaryingVectors: 31, MaxVertexUniformVectors: 4096,
			MaxViewportDims: [2]int{32768, 32768},
			AliasedLineWidthRange: [2]float64{1, 1}, AliasedPointSizeRange: [2]float64{1, 2047},
			MaxTextureImageUnits: 32, MaxVertexTextureImageUnits: 32, MaxCombinedTextureImageUnits: 192,
			MaxFragmentUniformVectors: 1024, MaxVertexAttribs: 29,
		},
		{
			MaxTextureSize: 16384, MaxCubeMapTextureSize: 16384, MaxRenderbufferSize: 16384,
			MaxVaryingVectors: 31, MaxVertexUniformVectors: 4096,
			MaxViewportDims: [2]int{16384, 16384},
			AliasedLineWidthRange: [2]float64{1, 1}, AliasedPointSizeRange: [2]float64{1, 2047},
			MaxTextureImageUnits: 32, MaxVertexTextureImageUnits: 32, MaxCombinedTextureImageUnits: 192,
			MaxFragmentUniformVectors: 1024, MaxVertexAttribs: 29,
		},
	},
	"amd": {
		{
			MaxTextureSize: 16384, MaxCubeMapTextureSize: 16384, MaxRenderbufferSize: 16384,
			MaxVaryingVectors: 30, MaxVertexUniformVectors: 4096,
			MaxViewportDims: [2]int{16384, 16384},
			AliasedLineWidthRange: [2]float64{1, 1}, AliasedPointSizeRange: [2]float64{1, 2047},
			MaxTextureImageUnits: 32, MaxVertexTextureImageUnits: 32, MaxCombinedTextureImageUnits: 192,
			MaxFragmentUniformVectors: 1024, MaxVertexAttribs: 29,
		},
		{
			MaxTextureSize: 8192, MaxCubeMapTextureSize: 8192, MaxRenderbufferSize: 8192,
			MaxVaryingVectors: 30, MaxVertexUniformVectors: 4096,
			MaxViewportDims: [2]int{8192, 8192},
			AliasedLineWidthRange: [2]float64{1, 1}, AliasedPointSizeRange: [2]float64{1, 2047},
			MaxTextureImageUnits: 32, MaxVertexTextureImageUnits: 32, MaxCombinedTextureImageUnits: 192,
			MaxFragmentUniformVectors: 1024, MaxVertexAttribs: 29,
		},
	},
	"apple": {
		{
			MaxTextureSize: 16384, MaxCubeMapTextureSize: 16384, MaxRenderbufferSize: 16384,
			MaxVaryingVectors: 31, MaxVertexUniformVectors: 4096,
			MaxViewportDims: [2]int{16384, 16384},
			AliasedLineWidthRange: [2]float64{1, 1}, AliasedPointSizeRange: [2]float64{1, 511},
			MaxTextureImageUnits: 16, MaxVertexTextureImageUnits: 16, MaxCombinedTextureImageUnits: 32,
			MaxFragmentUniformVectors: 1024, MaxVertexAttribs: 30,
		},
		{
			MaxTextureSize: 8192, MaxCubeMapTextureSize: 8192, MaxRenderbufferSize: 8192,
			MaxVaryingVectors: 31, MaxVertexUniformVectors: 4096,
			MaxViewportDims: [2]int{8192, 8192},
			AliasedLineWidthRange: [2]float64{1, 1}, AliasedPointSizeRange: [2]float64{1, 511},
			MaxTextureImageUnits: 16, MaxVertexTextureImageUnits: 16, MaxCombinedTextureImageUnits: 32,
			MaxFragmentUniformVectors: 1024, MaxVertexAttribs: 30,
		},
	},
}

// djb2Hash returns a deterministic 32-bit hash of s.
func djb2Hash(s string) uint32 {
	var h uint32 = 5381
	for i := range s {
		h = ((h << 5) + h) + uint32(s[i])
	}
	return h
}

// webGLCapsForFamily returns a stable WebGLCaps row for the given GPU family and seed.
func webGLCapsForFamily(family, seed string) WebGLCaps {
	family = strings.ToLower(strings.TrimSpace(family))
	variants := webGLCapsByFamily["intel"]
	if v, ok := webGLCapsByFamily[family]; ok {
		variants = v
	}
	idx := 0
	if len(variants) > 1 {
		idx = int(djb2Hash(seed)) % len(variants)
		if idx < 0 {
			idx = -idx
		}
	}
	v := variants[idx]
	return WebGLCaps{
		MaxTextureSize: v.MaxTextureSize, MaxCubeMapTextureSize: v.MaxCubeMapTextureSize,
		MaxRenderbufferSize: v.MaxRenderbufferSize, MaxVaryingVectors: v.MaxVaryingVectors,
		MaxVertexUniformVectors: v.MaxVertexUniformVectors, MaxViewportDims: v.MaxViewportDims,
		AliasedLineWidthRange: v.AliasedLineWidthRange, AliasedPointSizeRange: v.AliasedPointSizeRange,
		MaxTextureImageUnits: v.MaxTextureImageUnits, MaxVertexTextureImageUnits: v.MaxVertexTextureImageUnits,
		MaxCombinedTextureImageUnits: v.MaxCombinedTextureImageUnits, MaxFragmentUniformVectors: v.MaxFragmentUniformVectors,
		MaxVertexAttribs: v.MaxVertexAttribs,
	}
}

// ParseChromeFullVersion extracts a full Chrome version from a user-agent string.
// If the UA does not contain a Chrome/X.Y.Z.W token, it returns "120.0.0.0".
func ParseChromeFullVersion(ua string) string {
	start := strings.Index(ua, "Chrome/")
	if start == -1 {
		return "120.0.0.0"
	}
	start += len("Chrome/")
	end := strings.IndexAny(ua[start:], " ")
	if end == -1 {
		end = len(ua) - start
	}
	ver := ua[start : start+end]
	// Ensure at least four dotted components.
	parts := strings.Split(ver, ".")
	for len(parts) < 4 {
		parts = append(parts, "0")
	}
	return strings.Join(parts[:4], ".")
}

// appVersionFromUA derives navigator.appVersion from a user-agent string by stripping
// the leading "Mozilla/" prefix. If the prefix is absent, the full UA is returned.
// Real Chrome's appVersion is the user agent without the "Mozilla/" prefix.
func appVersionFromUA(ua string) string {
	const prefix = "Mozilla/"
	if strings.HasPrefix(ua, prefix) {
		return strings.TrimPrefix(ua, prefix)
	}
	return ua
}

// GenerateFingerprintOptions mirrors the TS options. Empty string == "random"/default.
type GenerateFingerprintOptions struct {
	Platform string // "windows" | "macos" | "linux" | "random" (default random)
	Browser  string // "chrome" | "edge" | "brave" (unused in generation, kept for parity)
	Version  int    // major browser version; 0 => random 118..122
	Screen   string // "desktop" | "laptop" | "retina" | "random"
	Gpu      string // "intel" | "nvidia" | "amd" | "apple" | "random"
	Language string // default "en-US"
	Timezone string
	// Overrides mutates the generated fingerprint before return. This is the idiomatic
	// Go equivalent of the TS `overrides: Partial<GeneratedFingerprint>` (Object.assign).
	Overrides func(*GeneratedFingerprint)
}

// ScreenInfo is the generated screen block.
type ScreenInfo struct {
	Width            int `json:"width"`
	Height           int `json:"height"`
	AvailWidth       int `json:"availWidth"`
	AvailHeight      int `json:"availHeight"`
	ColorDepth       int `json:"colorDepth"`
	PixelDepth       int `json:"pixelDepth"`
	DevicePixelRatio int `json:"devicePixelRatio"`
}

// WebGLInfo is the generated WebGL block.
type WebGLInfo struct {
	Vendor   string    `json:"vendor"`
	Renderer string    `json:"renderer"`
	Caps     WebGLCaps `json:"caps"`
}

// ClientHintsInfo is the generated client-hints block.
type ClientHintsInfo struct {
	Platform        string  `json:"platform"`
	PlatformVersion string  `json:"platformVersion"`
	Architecture    string  `json:"architecture"`
	Mobile          bool    `json:"mobile"`
	Brands          []Brand `json:"brands"`
	FullVersion     string  `json:"fullVersion"`
}

// FingerprintMeta is generation metadata.
type FingerprintMeta struct {
	GeneratedAt time.Time `json:"generatedAt"`
	Seed        string    `json:"seed"`
}

// GeneratedFingerprint is a complete generated fingerprint (mirrors the TS interface).
type GeneratedFingerprint struct {
	UserAgent           string          `json:"userAgent"`
	Platform            string          `json:"platform"`
	Language            string          `json:"language"`
	Languages           []string        `json:"languages"`
	HardwareConcurrency int             `json:"hardwareConcurrency"`
	DeviceMemory        int             `json:"deviceMemory"`
	Vendor              string          `json:"vendor"`
	AppVersion          string          `json:"appVersion"`
	ProductSub          string          `json:"productSub"`
	MaxTouchPoints      int             `json:"maxTouchPoints"`
	Mobile              bool            `json:"mobile"`
	Connection          NavigatorConnection `json:"connection"`
	Screen              ScreenInfo      `json:"screen"`
	WebGL               WebGLInfo       `json:"webgl"`
	ClientHints         ClientHintsInfo `json:"clientHints"`
	WebRTC              string          `json:"webrtc"` // "disable" | "fake" | "real"
	Canvas              string          `json:"canvas"` // "noise" | "real"
	Audio               string          `json:"audio"`  // "noise" | "real"
	Meta                FingerprintMeta `json:"meta"`
}

func randomItem[T any](s []T) T { return s[rand.IntN(len(s))] }

func randInt(min, max int) int { return rand.IntN(max-min+1) + min }

const base36 = "0123456789abcdefghijklmnopqrstuvwxyz"

func randBase36(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = base36[rand.IntN(len(base36))]
	}
	return string(b)
}

// GenerateFingerprint builds a realistic, consistent fingerprint (ports generateFingerprint).
func GenerateFingerprint(opts GenerateFingerprintOptions) GeneratedFingerprint {
	seed := fmt.Sprintf("fp-%d-%s", time.Now().UnixMilli(), randBase36(6))

	platforms := []string{"windows", "macos", "linux"}
	selPlat := opts.Platform
	if selPlat == "" || selPlat == "random" {
		selPlat = randomItem(platforms)
	}
	pc := platformConfigs[selPlat]

	userAgent := randomItem(userAgents[selPlat])

	screenTypes := []string{"desktop", "laptop", "retina"}
	selScreen := opts.Screen
	if selScreen == "" || selScreen == "random" {
		selScreen = randomItem(screenTypes)
	}
	res := randomItem(screenResolutions[selScreen])
	devicePixelRatio := 1
	if selScreen == "retina" {
		devicePixelRatio = 2
	}

	selGpu := opts.Gpu
	if selGpu == "" || selGpu == "random" {
		if selPlat == "macos" {
			selGpu = "apple"
		} else {
			selGpu = pc.GpuDefault
		}
	}
	webglRenderer := randomItem(webglRenderers[selGpu])

	language := opts.Language
	if language == "" {
		language = "en-US"
	}
	languages := []string{language, strings.Split(language, "-")[0]}

	coreOptions := []int{4, 6, 8, 12, 16}
	memoryOptions := []int{4, 8, 16, 32}
	if selPlat == "macos" {
		coreOptions = []int{8, 10, 12, 16}
		memoryOptions = []int{8, 16, 32, 64}
	}
	hardwareConcurrency := randomItem(coreOptions)
	deviceMemory := randomItem(memoryOptions)

	version := opts.Version
	if version == 0 {
		version = randInt(118, 122)
	}

	availHeightOffset := 40
	architecture := "x86"
	if selPlat == "macos" {
		availHeightOffset = 25
		architecture = "arm"
	}

	fp := GeneratedFingerprint{
		UserAgent:           userAgent,
		Platform:            pc.Platform,
		Language:            language,
		Languages:           languages,
		HardwareConcurrency: hardwareConcurrency,
		DeviceMemory:        deviceMemory,
		Vendor:              pc.Vendor,
		AppVersion:          appVersionFromUA(userAgent),
		ProductSub:          "20030107",
		MaxTouchPoints:      0,
		Mobile:              false,
		Connection: NavigatorConnection{
			EffectiveType: "4g",
			Downlink:      10,
			Rtt:           50,
			SaveData:      false,
		},
		Screen: ScreenInfo{
			Width:            res.Width,
			Height:           res.Height,
			AvailWidth:       res.Width,
			AvailHeight:      res.Height - availHeightOffset,
			ColorDepth:       24,
			PixelDepth:       24,
			DevicePixelRatio: devicePixelRatio,
		},
		WebGL: WebGLInfo{
			Vendor:   "Google Inc. (ANGLE)",
			Renderer: webglRenderer,
			Caps:     webGLCapsForFamily(selGpu, seed),
		},
		ClientHints: ClientHintsInfo{
			Platform:        clientHintsPlatforms[selPlat],
			PlatformVersion: randomItem(platformVersions[selPlat]),
			Architecture:    architecture,
			Mobile:          false,
			Brands: []Brand{
				{Brand: "Chromium", Version: strconv.Itoa(version)},
				{Brand: "Google Chrome", Version: strconv.Itoa(version)},
				{Brand: "Not_A Brand", Version: "8"},
			},
			FullVersion: ParseChromeFullVersion(userAgent),
		},
		Meta: FingerprintMeta{
			GeneratedAt: time.Now(),
			Seed:        seed,
		},
	}

	if opts.Overrides != nil {
		opts.Overrides(&fp)
	}
	return fp
}
