// ============================================================================
// @aitofy/browser-profiles - Profile Manager (Go port of src/profile-manager.ts)
// ============================================================================
//
// Filesystem CRUD for anti-detect browser profiles. Browser-free: this file
// contains no go-rod import and no launch/close (that is Phase 4). It reads
// TS-written config.json unchanged and writes semantically equivalent 2-space
// JSON.

package browserprofiles

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// defaultStorageSubpath is joined onto the user's home directory to form
// DEFAULT_STORAGE_PATH (~/.aitofy/browser-profiles), matching the TS source.
var defaultStorageSubpath = []string{".aitofy", "browser-profiles"}

// profileIDPattern mirrors the TS validateProfileId regexp: alphanumeric plus
// hyphen and underscore only.
var profileIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// generateID returns a unique profile/group ID: 8 random bytes hex-encoded
// (16 chars), matching crypto.randomBytes(8).toString('hex').
func generateID() string {
	b := make([]byte, 8)
	// crypto/rand.Read fills the buffer completely; a failure here is
	// catastrophic and cannot be meaningfully recovered from at this layer.
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// validateProfileID validates a custom profile ID: alphanumeric with optional
// hyphens/underscores, length 1..64.
func validateProfileID(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	return profileIDPattern.MatchString(id)
}

// firstN returns the first n characters of s (or all of s when shorter),
// mirroring JS String.prototype.slice(0, n) without panicking.
func firstN(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

// ListOptions filters and paginates List. The zero value means "all profiles,
// no filter, no pagination" (mirrors the optional TS list() argument).
type ListOptions struct {
	GroupID string   // exact-match filter on StoredProfile.GroupID when non-empty
	Tags    []string // overlap filter: keep a profile if it has ANY of these tags
	Limit   int      // 0 (or negative) means "no limit"
	Offset  int      // number of results to skip after sorting
}

// ProfileUpdate is a partial-update patch for Update. Each field is applied to
// the stored profile ONLY when non-nil, so an omitted (nil) field leaves the
// existing value untouched — this distinguishes "unset" from a zero value,
// mirroring the TS Partial<ProfileConfig> semantics (M7).
//
// String-valued fields are cleared by pointing them at the empty string (e.g.
// GroupID -> &"" clears the group; the field has json omitempty so it drops off
// disk). Proxy and Fingerprint are already pointer types on ProfileConfig, so a
// non-nil value replaces them; they cannot be reset to nil through Update (the
// TS source never does this either).
type ProfileUpdate struct {
	Name        *string
	Proxy       *ProxyConfig
	Timezone    *string
	Cookies     *[]ProfileCookie
	Fingerprint *FingerprintConfig
	StartURLs   *[]string
	Notes       *string
	GroupID     *string
	Tags        *[]string
}

// applyUpdate merges the non-nil fields of u onto p.
func applyUpdate(p *StoredProfile, u ProfileUpdate) {
	if u.Name != nil {
		p.Name = *u.Name
	}
	if u.Proxy != nil {
		p.Proxy = u.Proxy
	}
	if u.Timezone != nil {
		p.Timezone = *u.Timezone
	}
	if u.Cookies != nil {
		p.Cookies = *u.Cookies
	}
	if u.Fingerprint != nil {
		p.Fingerprint = u.Fingerprint
	}
	if u.StartURLs != nil {
		p.StartURLs = *u.StartURLs
	}
	if u.Notes != nil {
		p.Notes = *u.Notes
	}
	if u.GroupID != nil {
		p.GroupID = *u.GroupID
	}
	if u.Tags != nil {
		p.Tags = *u.Tags
	}
}

// BrowserProfiles manages anti-detect browser profiles on the filesystem.
//
// The runningBrowsers map, launch/close/getRunning and their name variants live
// in Phase 4 and are intentionally absent here.
type BrowserProfiles struct {
	storagePath  string
	profilesPath string
	groupsPath   string
	options      BrowserProfilesOptions
}

// NewBrowserProfiles constructs a manager. storagePath defaults to
// ~/.aitofy/browser-profiles when opts.StoragePath is empty. chromePath,
// defaultTimezone and defaultProxy are retained on the struct for later phases.
// The storage, profiles and groups directories are created best-effort.
func NewBrowserProfiles(opts BrowserProfilesOptions) *BrowserProfiles {
	storage := opts.StoragePath
	if storage == "" {
		home, _ := os.UserHomeDir()
		storage = filepath.Join(append([]string{home}, defaultStorageSubpath...)...)
	}
	bp := &BrowserProfiles{
		storagePath:  storage,
		profilesPath: filepath.Join(storage, "profiles"),
		groupsPath:   filepath.Join(storage, "groups"),
		options:      opts,
	}
	bp.ensureDirectories()
	return bp
}

// ensureDirectories creates the storage, profiles and groups directories.
func (bp *BrowserProfiles) ensureDirectories() {
	_ = os.MkdirAll(bp.storagePath, 0o700)
	_ = os.MkdirAll(bp.profilesPath, 0o700)
	_ = os.MkdirAll(bp.groupsPath, 0o700)
}

// StoragePath returns the resolved root storage directory.
func (bp *BrowserProfiles) StoragePath() string { return bp.storagePath }

// profileDir returns the profile directory, or "" if id is invalid. Validating in the
// path helpers (not just Create) is the single choke point that prevents path traversal
// via a caller-supplied id (e.g. "../../etc"): callers already treat "" / a bad path as
// "not found", so no read/mutate path can escape the profiles/ subtree.
func (bp *BrowserProfiles) profileDir(id string) string {
	if !validateProfileID(id) {
		return ""
	}
	return filepath.Join(bp.profilesPath, id)
}

// profileDataPath is the Chrome --user-data-dir for a profile ("" if id is invalid).
func (bp *BrowserProfiles) profileDataPath(id string) string {
	if !validateProfileID(id) {
		return ""
	}
	return filepath.Join(bp.profilesPath, id, "data")
}

func (bp *BrowserProfiles) profileConfigPath(id string) string {
	if !validateProfileID(id) {
		return ""
	}
	return filepath.Join(bp.profilesPath, id, "config.json")
}

func (bp *BrowserProfiles) groupPath(id string) string {
	if !validateProfileID(id) {
		return ""
	}
	return filepath.Join(bp.groupsPath, id+".json")
}

// normalizeProfile ensures the on-disk shape always emits [] for the slice
// fields and {} for fingerprint (their json tags omit "omitempty", so a nil
// pointer/slice would render as null). Proxy is deliberately left as-is: a nil
// proxy renders as null, which is the desired shape.
func normalizeProfile(p *StoredProfile) {
	if p.Cookies == nil {
		p.Cookies = []ProfileCookie{}
	}
	if p.StartURLs == nil {
		p.StartURLs = []string{}
	}
	if p.Tags == nil {
		p.Tags = []string{}
	}
	if p.Fingerprint == nil {
		p.Fingerprint = &FingerprintConfig{}
	}
}

// marshalProfile normalizes then renders p as 2-space JSON.
func marshalProfile(p *StoredProfile) ([]byte, error) {
	normalizeProfile(p)
	return json.MarshalIndent(p, "", "  ")
}

// writeProfileConfig writes p to its config.json as 2-space JSON.
func (bp *BrowserProfiles) writeProfileConfig(p *StoredProfile) error {
	data, err := marshalProfile(p)
	if err != nil {
		return err
	}
	return os.WriteFile(bp.profileConfigPath(p.ID), data, 0o600)
}

// Create creates a new browser profile.
//
// If cfg.ID is set it must pass validateProfileID and must not already exist;
// otherwise a fresh 16-char hex ID is generated. Defaults mirror the TS source:
// name "Profile <id[:6]>", timezone from opts/default, proxy from opts/default,
// empty slices, {} fingerprint, createdAt == updatedAt == now (unix millis).
func (bp *BrowserProfiles) Create(cfg ProfileConfig) (*StoredProfile, error) {
	var id string
	if cfg.ID != "" {
		if !validateProfileID(cfg.ID) {
			return nil, fmt.Errorf(
				`Invalid profile ID: "%s". ID must be 1-64 characters, alphanumeric with hyphens and underscores only.`,
				cfg.ID,
			)
		}
		if existing, _ := bp.Get(cfg.ID); existing != nil {
			return nil, fmt.Errorf(`Profile with ID "%s" already exists.`, cfg.ID)
		}
		id = cfg.ID
	} else {
		id = generateID()
	}

	now := time.Now().UnixMilli()

	p := &StoredProfile{ProfileConfig: cfg}
	p.ID = id
	if p.Name == "" {
		p.Name = "Profile " + firstN(id, 6)
	}
	if p.Timezone == "" {
		if bp.options.DefaultTimezone != "" {
			p.Timezone = bp.options.DefaultTimezone
		} else {
			p.Timezone = "America/New_York"
		}
	}
	if p.Proxy == nil {
		p.Proxy = bp.options.DefaultProxy
	}
	// Cookies / Fingerprint / StartURLs / Tags are normalized on write.
	p.CreatedAt = now
	p.UpdatedAt = now

	if err := os.MkdirAll(bp.profileDir(id), 0o700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(bp.profileDataPath(id), 0o700); err != nil {
		return nil, err
	}
	if err := bp.writeProfileConfig(p); err != nil {
		return nil, err
	}
	return p, nil
}

// Get returns the profile with the given ID, or (nil, nil) if it is missing or
// its config.json cannot be read/parsed — mirroring the TS get() which returns
// null on any error.
func (bp *BrowserProfiles) Get(id string) (*StoredProfile, error) {
	data, err := os.ReadFile(bp.profileConfigPath(id))
	if err != nil {
		return nil, nil
	}
	var p StoredProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, nil
	}
	return &p, nil
}

// GetByName returns the first profile whose name matches case-insensitively, or
// (nil, nil) if none. It scans List() output, i.e. createdAt-DESC order (ties
// broken by ascending profile id via the directory scan + stable sort).
func (bp *BrowserProfiles) GetByName(name string) (*StoredProfile, error) {
	all, err := bp.List(ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, p := range all {
		if strings.EqualFold(p.Name, name) {
			return p, nil
		}
	}
	return nil, nil
}

// GetByIdOrName tries Get first, then falls back to GetByName.
func (bp *BrowserProfiles) GetByIdOrName(idOrName string) (*StoredProfile, error) {
	if p, _ := bp.Get(idOrName); p != nil {
		return p, nil
	}
	return bp.GetByName(idOrName)
}

// List returns all profiles matching opts, sorted by CreatedAt descending
// (newest first), then sliced by Offset/Limit (Limit <= 0 means "all").
func (bp *BrowserProfiles) List(opts ListOptions) ([]*StoredProfile, error) {
	profiles := []*StoredProfile{}

	entries, err := os.ReadDir(bp.profilesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return profiles, nil
		}
		return profiles, err
	}

	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(bp.profilesPath, e.Name(), "config.json"))
		if err != nil {
			continue
		}
		var p StoredProfile
		if err := json.Unmarshal(data, &p); err != nil {
			// Skip invalid profiles.
			continue
		}

		if opts.GroupID != "" && p.GroupID != opts.GroupID {
			continue
		}
		if len(opts.Tags) > 0 && !hasAnyTag(p.Tags, opts.Tags) {
			continue
		}

		profiles = append(profiles, &p)
	}

	sort.SliceStable(profiles, func(i, j int) bool {
		return profiles[i].CreatedAt > profiles[j].CreatedAt
	})

	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	if offset > len(profiles) {
		offset = len(profiles)
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = len(profiles)
	}
	end := offset + limit
	if end > len(profiles) {
		end = len(profiles)
	}
	return profiles[offset:end], nil
}

// hasAnyTag reports whether profileTags contains at least one of wanted.
func hasAnyTag(profileTags, wanted []string) bool {
	for _, w := range wanted {
		for _, t := range profileTags {
			if t == w {
				return true
			}
		}
	}
	return false
}

// Update merges the provided patch fields onto the existing profile, forcing
// id/createdAt to be preserved and bumping updatedAt to now. Returns (nil, nil)
// if the profile does not exist.
func (bp *BrowserProfiles) Update(id string, patch ProfileUpdate) (*StoredProfile, error) {
	existing, _ := bp.Get(id)
	if existing == nil {
		return nil, nil
	}

	applyUpdate(existing, patch)
	// id and createdAt are preserved automatically (ProfileUpdate cannot touch
	// them); only updatedAt is refreshed.
	existing.UpdatedAt = time.Now().UnixMilli()

	if err := bp.writeProfileConfig(existing); err != nil {
		return nil, err
	}
	return existing, nil
}

// Delete removes the profile directory recursively. Returns false if the
// profile does not exist.
func (bp *BrowserProfiles) Delete(id string) (bool, error) {
	dir := bp.profileDir(id)
	// Close a tracked running browser for this profile first (mirrors TS delete()).
	CloseBrowser(id)
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if err := os.RemoveAll(dir); err != nil {
		return false, err
	}
	return true, nil
}

// CreateGroup creates a profile group with a fresh ID and writes
// groups/<id>.json (2-space JSON).
func (bp *BrowserProfiles) CreateGroup(name, description string) (*ProfileGroup, error) {
	id := generateID()
	g := &ProfileGroup{ID: id, Name: name, Description: description, ProfileCount: 0}
	data, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(bp.groupPath(id), data, 0o644); err != nil {
		return nil, err
	}
	return g, nil
}

// ListGroups parses all groups/*.json and recomputes each ProfileCount by
// scanning the profiles assigned to that group.
func (bp *BrowserProfiles) ListGroups() ([]*ProfileGroup, error) {
	groups := []*ProfileGroup{}

	entries, err := os.ReadDir(bp.groupsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return groups, nil
		}
		return groups, err
	}

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(bp.groupsPath, e.Name()))
		if err != nil {
			continue
		}
		var g ProfileGroup
		if err := json.Unmarshal(data, &g); err != nil {
			continue
		}
		members, _ := bp.List(ListOptions{GroupID: g.ID})
		g.ProfileCount = len(members)
		groups = append(groups, &g)
	}
	return groups, nil
}

// DeleteGroup removes the group file (profiles are left untouched). Returns
// false if the group does not exist.
func (bp *BrowserProfiles) DeleteGroup(id string) (bool, error) {
	p := bp.groupPath(id)
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if err := os.Remove(p); err != nil {
		return false, err
	}
	return true, nil
}

// MoveToGroup sets a profile's group. An empty groupID clears the assignment
// (mirroring the TS moveToGroup(null)). Returns whether the profile existed.
func (bp *BrowserProfiles) MoveToGroup(profileID, groupID string) (bool, error) {
	gid := groupID
	res, err := bp.Update(profileID, ProfileUpdate{GroupID: &gid})
	if err != nil {
		return false, err
	}
	return res != nil, nil
}

// Duplicate creates a copy of a profile under a new ID. The copy's name is
// newName, or "<original name> (Copy)" when newName is empty. Returns
// (nil, nil) if the source does not exist.
func (bp *BrowserProfiles) Duplicate(id, newName string) (*StoredProfile, error) {
	existing, _ := bp.Get(id)
	if existing == nil {
		return nil, nil
	}
	cfg := existing.ProfileConfig
	cfg.ID = "" // force a new ID
	if newName != "" {
		cfg.Name = newName
	} else {
		cfg.Name = existing.Name + " (Copy)"
	}
	return bp.Create(cfg)
}

// Export returns the profile serialized as 2-space JSON, or ("", nil) if the
// profile does not exist (mirroring the TS export() which returns null).
func (bp *BrowserProfiles) Export(id string) (string, error) {
	p, _ := bp.Get(id)
	if p == nil {
		return "", nil
	}
	data, err := marshalProfile(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Import parses a StoredProfile/ProfileConfig JSON string and creates a new
// profile from it under a fresh ID (avoiding conflicts).
func (bp *BrowserProfiles) Import(jsonStr string) (*StoredProfile, error) {
	var cfg ProfileConfig
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		return nil, err
	}
	cfg.ID = "" // generate a new ID
	return bp.Create(cfg)
}
