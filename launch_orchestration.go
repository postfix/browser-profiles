package browserprofiles

import (
	"fmt"
	"time"
)

// Launch launches a browser for the profile with the given ID. It mirrors the TS
// BrowserProfiles.launch: resolve the profile (error "Profile not found: <id>" if missing),
// delegate to LaunchChrome with the profile's data dir, then bump updatedAt and record
// lastLaunchedAt (a two-step config.json write, matching the source).
func (bp *BrowserProfiles) Launch(id string, opts LaunchOptions) (*LaunchResult, error) {
	profile, err := bp.Get(id)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, fmt.Errorf("Profile not found: %s", id)
	}
	if opts.ChromePath == "" {
		opts.ChromePath = bp.options.ChromePath
	}
	lr, err := LaunchChrome(profile, bp.profileDataPath(id), opts)
	if err != nil {
		return nil, err
	}
	// updatedAt bump (empty patch) then a separate lastLaunchedAt write — the TS double-write.
	_, _ = bp.Update(id, ProfileUpdate{})
	if p, _ := bp.Get(id); p != nil {
		p.LastLaunchedAt = time.Now().UnixMilli()
		_ = bp.writeProfileConfig(p)
	}
	return lr, nil
}

// LaunchByName launches the first profile matching name (case-insensitive).
func (bp *BrowserProfiles) LaunchByName(name string, opts LaunchOptions) (*LaunchResult, error) {
	p, err := bp.GetByName(name)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("Profile not found: %s", name)
	}
	return bp.Launch(p.ID, opts)
}

// LaunchByIdOrName launches by ID, falling back to name.
func (bp *BrowserProfiles) LaunchByIdOrName(idOrName string, opts LaunchOptions) (*LaunchResult, error) {
	p, err := bp.GetByIdOrName(idOrName)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("Profile not found: %s", idOrName)
	}
	return bp.Launch(p.ID, opts)
}

// Close closes the running browser for the given profile ID (no-op if not running).
func (bp *BrowserProfiles) Close(id string) error {
	CloseBrowser(id)
	return nil
}

// CloseAll closes all running browsers.
func (bp *BrowserProfiles) CloseAll() error {
	CloseAllBrowsers()
	return nil
}

// GetRunning returns the profile IDs of currently running browsers.
func (bp *BrowserProfiles) GetRunning() []string {
	return GetRunningBrowsers()
}
