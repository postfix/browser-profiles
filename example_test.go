package browserprofiles_test

import (
	"fmt"
	"log"
	"os"

	"github.com/go-rod/rod"
	bp "github.com/postfix/browser-profiles"
)

// ExampleCreateSession creates a temporary, in-memory-only session with a
// random fingerprint. Terminate tears down the browser and discards the
// synthetic profile (nothing persists to disk).
func ExampleCreateSession() {
	sess, err := bp.CreateSession(bp.CreateSessionOptions{
		Proxy: &bp.ProxyConfig{Type: "http", Host: "proxy.example.com", Port: 8080},
		// RandomFingerprint defaults to true; timezone auto-detected from the proxy IP.
	})
	if err != nil {
		log.Fatal(err)
	}
	defer sess.Terminate()

	if err := sess.Page.Navigate("https://browserscan.net"); err != nil {
		log.Fatal(err)
	}
	sess.Page.MustWaitLoad()
}

// ExampleCreateSession_persistent creates (or reuses) a real, on-disk profile
// addressed by Name. A second CreateSession call with the same Name adopts
// the existing profile instead of creating a new one; on reuse, any
// Fingerprint/Proxy/Timezone/RandomFingerprint overrides passed here are
// ignored in favor of the profile's stored values.
func ExampleCreateSession_persistent() {
	sess, err := bp.CreateSession(bp.CreateSessionOptions{
		Temporary: new(false),
		Name:      "my-persistent-session",
		Headless:  true,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer sess.Terminate()

	// A second call with Name: "my-persistent-session" reuses this same
	// on-disk profile instead of creating a new one.
	sess.Page.MustNavigate("https://whoer.net")
}

// ExampleWithProfile creates a stored profile and launches it, connecting
// go-rod directly.
func ExampleWithProfile() {
	profiles := bp.NewBrowserProfiles(bp.BrowserProfilesOptions{})

	// Create a persistent profile (stored under ~/.aitofy/browser-profiles/).
	p, err := profiles.Create(bp.ProfileConfig{
		Name:  "My Account",
		Proxy: &bp.ProxyConfig{Type: "http", Host: "proxy.example.com", Port: 8080, Username: "u", Password: "p"},
	})
	if err != nil {
		log.Fatal(err)
	}

	// Launch it and drive it with go-rod.
	sess, err := bp.WithProfile(profiles, p.ID, bp.LaunchOptions{})
	if err != nil {
		log.Fatal(err)
	}
	defer sess.Close()
	sess.Page.MustNavigate("https://whoer.net")
}

// ExampleQuickLaunch launches a real, auto-named persistent profile in one
// call. Terminate additionally deletes the profile since it was auto-named.
func ExampleQuickLaunch() {
	sess, err := bp.QuickLaunch(bp.QuickLaunchOptions{Headless: true})
	if err != nil {
		log.Fatal(err)
	}
	defer sess.Terminate()

	sess.Page.MustNavigate("https://example.com")
}

// ExamplePatchPage applies anti-detect protections to a page obtained
// outside of WithProfile/QuickLaunch/CreateSession — e.g. one connected via
// a raw CDP endpoint.
func ExamplePatchPage() {
	res, err := bp.LaunchChromeStandalone(bp.StandaloneLaunchOptions{Headless: true})
	if err != nil {
		log.Fatal(err)
	}
	defer res.Close()

	page := rod.New().ControlURL(res.WsEndpoint).MustConnect().MustPage("")
	if err := bp.PatchPage(page, bp.PatchPageOptions{}); err != nil {
		log.Fatal(err)
	}
}

// ExampleLaunchChromeStandalone launches Chrome with a synthetic profile and
// a temp user-data-dir, returning a raw CDP endpoint (no stored profile).
func ExampleLaunchChromeStandalone() {
	res, err := bp.LaunchChromeStandalone(bp.StandaloneLaunchOptions{
		Headless: true,
		Proxy:    &bp.ProxyConfig{Type: "socks5", Host: "proxy.example.com", Port: 1080, Username: "u", Password: "p"},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer res.Close()
	// res.WsEndpoint / res.PID / res.Port — connect any CDP client, e.g. rod.New().ControlURL(res.WsEndpoint)
}

// ExampleBrowserProfiles_Launch creates a profile then launches it via the
// BrowserProfiles manager directly (equivalent to, but lower-level than,
// WithProfile).
func ExampleBrowserProfiles_Launch() {
	profiles := bp.NewBrowserProfiles(bp.BrowserProfilesOptions{})
	p, err := profiles.Create(bp.ProfileConfig{Name: "Demo"})
	if err != nil {
		log.Fatal(err)
	}

	lr, err := profiles.Launch(p.ID, bp.LaunchOptions{Headless: true})
	if err != nil {
		log.Fatal(err)
	}
	defer lr.Close()
}

// ExampleBrowserProfiles_Create creates a profile under a throwaway storage
// directory (never the real ~/.aitofy/browser-profiles) and prints its
// caller-supplied Name — the only deterministic field on a freshly created
// profile (ID/CreatedAt/UpdatedAt are random/time-based).
func ExampleBrowserProfiles_Create() {
	dir, err := os.MkdirTemp("", "browserprofiles-example-*")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer os.RemoveAll(dir)

	profiles := bp.NewBrowserProfiles(bp.BrowserProfilesOptions{StoragePath: dir})
	p, err := profiles.Create(bp.ProfileConfig{Name: "Demo"})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(p.Name)
	// Output:
	// Demo
}
