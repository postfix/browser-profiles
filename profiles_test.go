package browserprofiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// newTestBP returns a BrowserProfiles rooted at a throwaway temp dir. It never
// touches the real ~/.aitofy.
func newTestBP(t *testing.T) *BrowserProfiles {
	t.Helper()
	return NewBrowserProfiles(BrowserProfilesOptions{StoragePath: t.TempDir()})
}

// seedProfile creates a profile and forces a deterministic CreatedAt so ordering
// assertions do not depend on wall-clock timing.
func seedProfile(t *testing.T, bp *BrowserProfiles, name, groupID string, tags []string, createdAt int64) *StoredProfile {
	t.Helper()
	p, err := bp.Create(ProfileConfig{Name: name, GroupID: groupID, Tags: tags})
	if err != nil {
		t.Fatalf("seed create %q: %v", name, err)
	}
	p.CreatedAt = createdAt
	if err := bp.writeProfileConfig(p); err != nil {
		t.Fatalf("seed write %q: %v", name, err)
	}
	return p
}

// ---------------------------------------------------------------------------
// Group 1: Create writes config.json (2-space) + data/ with correct defaults.
// ---------------------------------------------------------------------------

func TestCreate(t *testing.T) {
	bp := newTestBP(t)

	p, err := bp.Create(ProfileConfig{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(p.ID) != 16 {
		t.Fatalf("generated id len = %d, want 16 (%q)", len(p.ID), p.ID)
	}
	if want := "Profile " + p.ID[:6]; p.Name != want {
		t.Fatalf("default name = %q, want %q", p.Name, want)
	}
	if p.Timezone != "America/New_York" {
		t.Fatalf("default timezone = %q, want America/New_York", p.Timezone)
	}
	if p.CreatedAt == 0 || p.CreatedAt != p.UpdatedAt {
		t.Fatalf("timestamps = (%d,%d), want equal non-zero", p.CreatedAt, p.UpdatedAt)
	}

	cfgPath := bp.profileConfigPath(p.ID)
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("config.json not at %s: %v", cfgPath, err)
	}
	dataInfo, err := os.Stat(bp.profileDataPath(p.ID))
	if err != nil || !dataInfo.IsDir() {
		t.Fatalf("data/ dir missing or not a dir: %v", err)
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	// 2-space indentation of top-level keys.
	if !strings.Contains(content, "\n  \"id\":") {
		t.Fatalf("config.json is not 2-space indented:\n%s", content)
	}
	// On-disk shape parity: proxy null, empty slices [], fingerprint {}.
	for _, want := range []string{
		`"proxy": null`,
		`"cookies": []`,
		`"fingerprint": {}`,
		`"startUrls": []`,
		`"tags": []`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("config.json missing %q:\n%s", want, content)
		}
	}
	// notes/groupId/lastLaunchedAt are omitted when unset.
	for _, absent := range []string{`"notes"`, `"groupId"`, `"lastLaunchedAt"`} {
		if strings.Contains(content, absent) {
			t.Fatalf("config.json should omit %s:\n%s", absent, content)
		}
	}
}

func TestCreateRespectsDefaultsFromOptions(t *testing.T) {
	proxy := &ProxyConfig{Type: "http", Host: "p.example", Port: 3128}
	bp := NewBrowserProfiles(BrowserProfilesOptions{
		StoragePath:     t.TempDir(),
		DefaultTimezone: "Europe/Berlin",
		DefaultProxy:    proxy,
	})
	p, err := bp.Create(ProfileConfig{Name: "X"})
	if err != nil {
		t.Fatal(err)
	}
	if p.Timezone != "Europe/Berlin" {
		t.Fatalf("timezone = %q, want Europe/Berlin", p.Timezone)
	}
	if p.Proxy == nil || p.Proxy.Host != "p.example" || p.Proxy.Port != 3128 {
		t.Fatalf("default proxy not applied: %+v", p.Proxy)
	}
}

// ---------------------------------------------------------------------------
// Group 2: A hand-written TS-style config.json reads back intact.
// ---------------------------------------------------------------------------

func TestReadTSConfig(t *testing.T) {
	bp := newTestBP(t)
	const id = "a1b2c3d4e5f60718"

	raw, err := os.ReadFile(filepath.Join("testdata", "ts-config.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	dir := filepath.Join(bp.StoragePath(), "profiles", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	p, err := bp.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if p == nil {
		t.Fatal("Get returned nil for TS fixture")
	}
	if p.ID != id {
		t.Fatalf("id = %q", p.ID)
	}
	if p.Name != "Google Account" {
		t.Fatalf("name = %q", p.Name)
	}
	if p.Timezone != "America/New_York" {
		t.Fatalf("timezone = %q", p.Timezone)
	}
	if p.Proxy == nil || p.Proxy.Type != "http" || p.Proxy.Host != "proxy.example.com" ||
		p.Proxy.Port != 8080 || p.Proxy.Username != "proxyuser" || p.Proxy.Password != "s3cret" {
		t.Fatalf("proxy mismatch: %+v", p.Proxy)
	}
	if len(p.Cookies) != 1 {
		t.Fatalf("cookies len = %d, want 1", len(p.Cookies))
	}
	c := p.Cookies[0]
	if c.Name != "SID" || c.Value != "abc123" || c.Domain != ".google.com" ||
		!c.HTTPOnly || !c.Secure || c.SameSite != "Lax" || c.Expires != 1893456000 {
		t.Fatalf("cookie mismatch: %+v", c)
	}
	if p.Fingerprint == nil || p.Fingerprint.Platform != "Win32" ||
		p.Fingerprint.HardwareConcurrency != 8 || p.Fingerprint.DeviceMemory != 8 ||
		p.Fingerprint.WebRTC != "disable" || p.Fingerprint.Canvas != "noise" || p.Fingerprint.Audio != "noise" {
		t.Fatalf("fingerprint mismatch: %+v", p.Fingerprint)
	}
	if p.Fingerprint.Screen == nil || p.Fingerprint.Screen.Width != 1920 || p.Fingerprint.Screen.Height != 1080 {
		t.Fatalf("fingerprint.screen mismatch: %+v", p.Fingerprint.Screen)
	}
	if p.Fingerprint.WebGL == nil || p.Fingerprint.WebGL.Vendor != "Google Inc. (NVIDIA)" {
		t.Fatalf("fingerprint.webgl mismatch: %+v", p.Fingerprint.WebGL)
	}
	if !reflect.DeepEqual(p.StartURLs, []string{"https://accounts.google.com", "https://mail.google.com"}) {
		t.Fatalf("startUrls = %v", p.StartURLs)
	}
	if !reflect.DeepEqual(p.Tags, []string{"work", "google"}) {
		t.Fatalf("tags = %v", p.Tags)
	}
	if p.CreatedAt != 1710000000000 || p.UpdatedAt != 1710000500000 {
		t.Fatalf("timestamps = (%d,%d)", p.CreatedAt, p.UpdatedAt)
	}
}

// ---------------------------------------------------------------------------
// Group 3: Marshal -> Unmarshal round-trip on a fully-populated profile.
// ---------------------------------------------------------------------------

func TestRoundTrip(t *testing.T) {
	in := StoredProfile{
		ProfileConfig: ProfileConfig{
			ID:       "round-trip-id",
			Name:     "Full Profile",
			Proxy:    &ProxyConfig{Type: "socks5", Host: "1.2.3.4", Port: 1080, Username: "u", Password: "p"},
			Timezone: "America/New_York",
			Cookies: []ProfileCookie{
				{Name: "c", Value: "v", Domain: ".x.com", Path: "/", HTTPOnly: true, Secure: true, SameSite: "None", Expires: 42},
			},
			Fingerprint: &FingerprintConfig{
				UserAgent:           "UA",
				Language:            "en-US",
				Screen:              &ScreenConfig{Width: 1280, Height: 720, DeviceScaleFactor: 2},
				WebGL:               &WebGLConfig{Vendor: "V", Renderer: "R"},
				Platform:            "Linux x86_64",
				HardwareConcurrency: 16,
				DeviceMemory:        16,
				WebRTC:              "fake",
				Canvas:              "noise",
				Audio:               "real",
			},
			StartURLs: []string{"https://a", "https://b"},
			Notes:     "some notes",
			GroupID:   "grp-1",
			Tags:      []string{"a", "b"},
		},
		CreatedAt:      111,
		UpdatedAt:      222,
		LastLaunchedAt: 333,
	}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out StoredProfile
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\n in = %+v\nout = %+v", in, out)
	}
}

// ---------------------------------------------------------------------------
// Group 4: ID validation (valid/invalid/length) + duplicate rejection.
// ---------------------------------------------------------------------------

func TestValidateProfileID(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"", false},
		{"a", true},
		{"google-main", true},
		{"abc_123-XYZ", true},
		{strings.Repeat("a", 64), true},
		{strings.Repeat("a", 65), false},
		{"bad id!", false},
		{"with space", false},
		{"dot.name", false}, // period is NOT permitted by the regexp
		{"slash/name", false},
	}
	for _, c := range cases {
		if got := validateProfileID(c.id); got != c.want {
			t.Errorf("validateProfileID(%q) = %v, want %v", c.id, got, c.want)
		}
	}
}

func TestCreateIDValidationAndDuplicate(t *testing.T) {
	bp := newTestBP(t)

	// Invalid custom id -> exact TS error string.
	_, err := bp.Create(ProfileConfig{ID: "bad id!", Name: "x"})
	if err == nil {
		t.Fatal("expected error for invalid id")
	}
	wantMsg := `Invalid profile ID: "bad id!". ID must be 1-64 characters, alphanumeric with hyphens and underscores only.`
	if err.Error() != wantMsg {
		t.Fatalf("invalid-id error = %q\nwant %q", err.Error(), wantMsg)
	}

	// Valid custom id accepted.
	p, err := bp.Create(ProfileConfig{ID: "google-main", Name: "Google"})
	if err != nil {
		t.Fatalf("valid custom id: %v", err)
	}
	if p.ID != "google-main" {
		t.Fatalf("id = %q, want google-main", p.ID)
	}

	// Duplicate id rejected with exact TS error string.
	_, err = bp.Create(ProfileConfig{ID: "google-main", Name: "again"})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
	wantDup := `Profile with ID "google-main" already exists.`
	if err.Error() != wantDup {
		t.Fatalf("duplicate error = %q\nwant %q", err.Error(), wantDup)
	}
}

// ---------------------------------------------------------------------------
// Group 5: GetByName case-insensitive + List filter/sort/paginate.
// ---------------------------------------------------------------------------

func TestGetByName(t *testing.T) {
	bp := newTestBP(t)
	if _, err := bp.Create(ProfileConfig{Name: "Alpha One"}); err != nil {
		t.Fatal(err)
	}
	got, err := bp.GetByName("alpha one")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Name != "Alpha One" {
		t.Fatalf("GetByName case-insensitive failed: %+v", got)
	}
	miss, err := bp.GetByName("nope")
	if err != nil {
		t.Fatal(err)
	}
	if miss != nil {
		t.Fatalf("GetByName miss = %+v, want nil", miss)
	}
}

func TestList(t *testing.T) {
	bp := newTestBP(t)
	seedProfile(t, bp, "A", "g1", []string{"x", "y"}, 300)
	seedProfile(t, bp, "B", "g1", []string{"y"}, 200)
	seedProfile(t, bp, "C", "g2", []string{"z"}, 100)

	names := func(ps []*StoredProfile) []string {
		out := make([]string, len(ps))
		for i, p := range ps {
			out[i] = p.Name
		}
		return out
	}

	// All, sorted by createdAt DESC.
	all, _ := bp.List(ListOptions{})
	if got := names(all); !reflect.DeepEqual(got, []string{"A", "B", "C"}) {
		t.Fatalf("List all order = %v, want [A B C]", got)
	}

	// groupId exact filter.
	g1, _ := bp.List(ListOptions{GroupID: "g1"})
	if got := names(g1); !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Fatalf("List g1 = %v, want [A B]", got)
	}

	// tags overlap (any).
	tagX, _ := bp.List(ListOptions{Tags: []string{"x"}})
	if got := names(tagX); !reflect.DeepEqual(got, []string{"A"}) {
		t.Fatalf("List tag x = %v, want [A]", got)
	}
	tagY, _ := bp.List(ListOptions{Tags: []string{"y"}})
	if got := names(tagY); !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Fatalf("List tag y = %v, want [A B]", got)
	}
	tagZX, _ := bp.List(ListOptions{Tags: []string{"z", "x"}})
	if got := names(tagZX); !reflect.DeepEqual(got, []string{"A", "C"}) {
		t.Fatalf("List tag {z,x} = %v, want [A C]", got)
	}

	// pagination.
	lim, _ := bp.List(ListOptions{Limit: 2})
	if got := names(lim); !reflect.DeepEqual(got, []string{"A", "B"}) {
		t.Fatalf("List limit 2 = %v, want [A B]", got)
	}
	off, _ := bp.List(ListOptions{Offset: 1})
	if got := names(off); !reflect.DeepEqual(got, []string{"B", "C"}) {
		t.Fatalf("List offset 1 = %v, want [B C]", got)
	}
	offLim, _ := bp.List(ListOptions{Offset: 1, Limit: 1})
	if got := names(offLim); !reflect.DeepEqual(got, []string{"B"}) {
		t.Fatalf("List offset 1 limit 1 = %v, want [B]", got)
	}
	// offset past the end yields empty.
	past, _ := bp.List(ListOptions{Offset: 99})
	if len(past) != 0 {
		t.Fatalf("List offset 99 = %v, want empty", names(past))
	}
}

// ---------------------------------------------------------------------------
// Group 6: Update merge, Delete, Groups, MoveToGroup, Duplicate, Export/Import.
// ---------------------------------------------------------------------------

func TestUpdateMergesOnlyProvidedFields(t *testing.T) {
	bp := newTestBP(t)
	p, err := bp.Create(ProfileConfig{Name: "Orig", Timezone: "Europe/London", Tags: []string{"a"}})
	if err != nil {
		t.Fatal(err)
	}
	// Force an old createdAt so the updatedAt bump is observable.
	p.CreatedAt = 1000
	if err := bp.writeProfileConfig(p); err != nil {
		t.Fatal(err)
	}

	newName := "Renamed"
	updated, err := bp.Update(p.ID, ProfileUpdate{Name: &newName})
	if err != nil {
		t.Fatal(err)
	}
	if updated == nil {
		t.Fatal("Update returned nil for existing profile")
	}
	if updated.Name != "Renamed" {
		t.Fatalf("name = %q, want Renamed", updated.Name)
	}
	if updated.Timezone != "Europe/London" {
		t.Fatalf("timezone changed to %q; should be untouched", updated.Timezone)
	}
	if !reflect.DeepEqual(updated.Tags, []string{"a"}) {
		t.Fatalf("tags changed to %v; should be untouched", updated.Tags)
	}
	if updated.ID != p.ID {
		t.Fatalf("id changed: %q -> %q", p.ID, updated.ID)
	}
	if updated.CreatedAt != 1000 {
		t.Fatalf("createdAt = %d, want preserved 1000", updated.CreatedAt)
	}
	if updated.UpdatedAt <= 1000 {
		t.Fatalf("updatedAt = %d, want bumped above 1000", updated.UpdatedAt)
	}

	// Persisted to disk.
	reloaded, _ := bp.Get(p.ID)
	if reloaded == nil || reloaded.Name != "Renamed" || reloaded.Timezone != "Europe/London" {
		t.Fatalf("reloaded mismatch: %+v", reloaded)
	}

	// Missing profile -> (nil, nil).
	miss, err := bp.Update("does-not-exist", ProfileUpdate{Name: &newName})
	if err != nil || miss != nil {
		t.Fatalf("Update missing = (%v, %v), want (nil, nil)", miss, err)
	}
}

func TestDelete(t *testing.T) {
	bp := newTestBP(t)
	p, err := bp.Create(ProfileConfig{Name: "ToDelete"})
	if err != nil {
		t.Fatal(err)
	}
	ok, err := bp.Delete(p.ID)
	if err != nil || !ok {
		t.Fatalf("Delete = (%v, %v), want (true, nil)", ok, err)
	}
	if _, err := os.Stat(bp.profileDir(p.ID)); !os.IsNotExist(err) {
		t.Fatalf("profile dir still present: %v", err)
	}
	if got, _ := bp.Get(p.ID); got != nil {
		t.Fatalf("Get after delete = %+v, want nil", got)
	}
	// Deleting again returns false.
	ok, err = bp.Delete(p.ID)
	if err != nil || ok {
		t.Fatalf("second Delete = (%v, %v), want (false, nil)", ok, err)
	}
}

func TestGroups(t *testing.T) {
	bp := newTestBP(t)

	g, err := bp.CreateGroup("Group1", "the first group")
	if err != nil {
		t.Fatal(err)
	}
	if len(g.ID) != 16 {
		t.Fatalf("group id len = %d, want 16", len(g.ID))
	}
	if _, err := os.Stat(bp.groupPath(g.ID)); err != nil {
		t.Fatalf("group file missing: %v", err)
	}

	// Two profiles in the group.
	m1, _ := bp.Create(ProfileConfig{Name: "M1", GroupID: g.ID})
	m2, _ := bp.Create(ProfileConfig{Name: "M2", GroupID: g.ID})

	groups, err := bp.ListGroups()
	if err != nil {
		t.Fatal(err)
	}
	var found *ProfileGroup
	for _, gg := range groups {
		if gg.ID == g.ID {
			found = gg
		}
	}
	if found == nil {
		t.Fatal("group not listed")
	}
	if found.ProfileCount != 2 {
		t.Fatalf("profileCount = %d, want 2", found.ProfileCount)
	}
	if found.Description != "the first group" {
		t.Fatalf("description = %q", found.Description)
	}

	// DeleteGroup removes the group file but keeps profiles.
	ok, err := bp.DeleteGroup(g.ID)
	if err != nil || !ok {
		t.Fatalf("DeleteGroup = (%v, %v), want (true, nil)", ok, err)
	}
	if _, err := os.Stat(bp.groupPath(g.ID)); !os.IsNotExist(err) {
		t.Fatalf("group file still present: %v", err)
	}
	if got, _ := bp.Get(m1.ID); got == nil {
		t.Fatal("profile M1 was deleted with the group")
	}
	if got, _ := bp.Get(m2.ID); got == nil {
		t.Fatal("profile M2 was deleted with the group")
	}
	// Deleting a missing group returns false.
	ok, err = bp.DeleteGroup(g.ID)
	if err != nil || ok {
		t.Fatalf("second DeleteGroup = (%v, %v), want (false, nil)", ok, err)
	}
}

func TestMoveToGroup(t *testing.T) {
	bp := newTestBP(t)
	p, _ := bp.Create(ProfileConfig{Name: "Mover"})
	g, _ := bp.CreateGroup("Dest", "")

	ok, err := bp.MoveToGroup(p.ID, g.ID)
	if err != nil || !ok {
		t.Fatalf("MoveToGroup = (%v, %v), want (true, nil)", ok, err)
	}
	got, _ := bp.Get(p.ID)
	if got.GroupID != g.ID {
		t.Fatalf("groupId = %q, want %q", got.GroupID, g.ID)
	}
	inGroup, _ := bp.List(ListOptions{GroupID: g.ID})
	if len(inGroup) != 1 || inGroup[0].ID != p.ID {
		t.Fatalf("List by group did not find moved profile: %+v", inGroup)
	}

	// Clearing the group with "" removes the field on disk.
	ok, err = bp.MoveToGroup(p.ID, "")
	if err != nil || !ok {
		t.Fatalf("MoveToGroup(clear) = (%v, %v), want (true, nil)", ok, err)
	}
	got, _ = bp.Get(p.ID)
	if got.GroupID != "" {
		t.Fatalf("groupId after clear = %q, want empty", got.GroupID)
	}
	raw, _ := os.ReadFile(bp.profileConfigPath(p.ID))
	if strings.Contains(string(raw), `"groupId"`) {
		t.Fatalf("cleared groupId still on disk:\n%s", raw)
	}

	// Moving a missing profile returns false.
	ok, err = bp.MoveToGroup("missing", g.ID)
	if err != nil || ok {
		t.Fatalf("MoveToGroup(missing) = (%v, %v), want (false, nil)", ok, err)
	}
}

func TestDuplicate(t *testing.T) {
	bp := newTestBP(t)
	src, _ := bp.Create(ProfileConfig{Name: "Source", Tags: []string{"t"}})

	dup, err := bp.Duplicate(src.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if dup == nil {
		t.Fatal("Duplicate returned nil")
	}
	if dup.ID == src.ID {
		t.Fatalf("duplicate reused source id %q", dup.ID)
	}
	if dup.Name != "Source (Copy)" {
		t.Fatalf("duplicate name = %q, want 'Source (Copy)'", dup.Name)
	}
	if !reflect.DeepEqual(dup.Tags, []string{"t"}) {
		t.Fatalf("duplicate tags = %v, want [t]", dup.Tags)
	}

	dup2, err := bp.Duplicate(src.ID, "Custom Name")
	if err != nil {
		t.Fatal(err)
	}
	if dup2.Name != "Custom Name" {
		t.Fatalf("duplicate custom name = %q", dup2.Name)
	}

	// Missing source -> (nil, nil).
	miss, err := bp.Duplicate("missing", "")
	if err != nil || miss != nil {
		t.Fatalf("Duplicate missing = (%v, %v), want (nil, nil)", miss, err)
	}
}

func TestExportImport(t *testing.T) {
	bp := newTestBP(t)
	src, _ := bp.Create(ProfileConfig{Name: "Exportable", Timezone: "Asia/Tokyo", Tags: []string{"t1"}})

	s, err := bp.Export(src.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "Exportable") || !strings.Contains(s, "\n  \"id\":") {
		t.Fatalf("export output unexpected:\n%s", s)
	}

	// Missing profile exports "".
	empty, err := bp.Export("missing")
	if err != nil || empty != "" {
		t.Fatalf("Export missing = (%q, %v), want (\"\", nil)", empty, err)
	}

	imp, err := bp.Import(s)
	if err != nil {
		t.Fatal(err)
	}
	if imp.ID == src.ID {
		t.Fatalf("import reused id %q", imp.ID)
	}
	if imp.Name != "Exportable" || imp.Timezone != "Asia/Tokyo" {
		t.Fatalf("import fields mismatch: %+v", imp)
	}
	if !reflect.DeepEqual(imp.Tags, []string{"t1"}) {
		t.Fatalf("import tags = %v, want [t1]", imp.Tags)
	}

	// Invalid JSON -> error.
	if _, err := bp.Import("{not json"); err == nil {
		t.Fatal("Import invalid json: expected error")
	}
}

// ---------------------------------------------------------------------------
// Group 7: Launch error paths (Chrome-free).
// ---------------------------------------------------------------------------

func TestLaunchByNameNotFound(t *testing.T) {
	bp := newTestBP(t)
	_, err := bp.LaunchByName("does-not-exist", LaunchOptions{})
	if err == nil || !strings.Contains(err.Error(), "Profile not found: does-not-exist") {
		t.Fatalf("LaunchByName missing: %v", err)
	}
}

func TestLaunchByIdOrNameNotFound(t *testing.T) {
	bp := newTestBP(t)
	_, err := bp.LaunchByIdOrName("missing", LaunchOptions{})
	if err == nil || !strings.Contains(err.Error(), "Profile not found: missing") {
		t.Fatalf("LaunchByIdOrName missing: %v", err)
	}
}

func TestLaunchMissingProfile(t *testing.T) {
	bp := newTestBP(t)
	_, err := bp.Launch("missing", LaunchOptions{})
	if err == nil || !strings.Contains(err.Error(), "Profile not found: missing") {
		t.Fatalf("Launch missing: %v", err)
	}
}

func TestLaunchByNameSuccessSkipsWithoutChrome(t *testing.T) {
	bp := newTestBP(t)
	_, _ = bp.Create(ProfileConfig{Name: "exists"})
	if _, err := GetChromePath(""); err != nil {
		t.Skip("no Chrome available; skip the real-launch success path")
	}
	lr, err := bp.LaunchByName("exists", LaunchOptions{})
	if err != nil {
		t.Fatalf("LaunchByName success: %v", err)
	}
	_ = lr.Close()
}

// ---------------------------------------------------------------------------
// Group 8: Group and update edge paths.
// ---------------------------------------------------------------------------

func TestCreateGroupDuplicate(t *testing.T) {
	bp := newTestBP(t)
	g1, err := bp.CreateGroup("same", "")
	if err != nil {
		t.Fatal(err)
	}
	g2, err := bp.CreateGroup("same", "")
	if err != nil {
		t.Fatal(err)
	}
	if g1.ID == g2.ID {
		t.Fatal("duplicate group names must still get different IDs")
	}
}

func TestListGroupsEmpty(t *testing.T) {
	bp := newTestBP(t)
	groups, err := bp.ListGroups()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 {
		t.Fatalf("ListGroups empty = %d, want 0", len(groups))
	}
}

func TestDeleteGroupMissing(t *testing.T) {
	bp := newTestBP(t)
	ok, err := bp.DeleteGroup("missing")
	if err != nil || ok {
		t.Fatalf("DeleteGroup missing = (%v, %v), want (false, nil)", ok, err)
	}
}

func TestMoveToGroupMissing(t *testing.T) {
	bp := newTestBP(t)
	ok, err := bp.MoveToGroup("missing", "")
	if err != nil || ok {
		t.Fatalf("MoveToGroup missing = (%v, %v), want (false, nil)", ok, err)
	}
}

func TestApplyUpdateClearNotes(t *testing.T) {
	bp := newTestBP(t)
	p, _ := bp.Create(ProfileConfig{Name: "notes", Notes: "original"})
	empty := ""
	bp.Update(p.ID, ProfileUpdate{Notes: &empty})
	reloaded, _ := bp.Get(p.ID)
	if reloaded == nil || reloaded.Notes != "" {
		t.Fatalf("notes not cleared, got %q", reloaded.Notes)
	}

	// Non-pointer nil leaves notes untouched.
	p2, _ := bp.Create(ProfileConfig{Name: "notes2", Notes: "keep"})
	bp.Update(p2.ID, ProfileUpdate{})
	reloaded2, _ := bp.Get(p2.ID)
	if reloaded2 == nil || reloaded2.Notes != "keep" {
		t.Fatalf("notes should be untouched, got %q", reloaded2.Notes)
	}
}

func TestApplyUpdateEmptyStartUrls(t *testing.T) {
	bp := newTestBP(t)
	p, _ := bp.Create(ProfileConfig{Name: "urls"})
	urls := []string{"https://a"}
	bp.Update(p.ID, ProfileUpdate{StartURLs: &urls})
	reloaded, _ := bp.Get(p.ID)
	if reloaded == nil || len(reloaded.StartURLs) != 1 || reloaded.StartURLs[0] != "https://a" {
		t.Fatalf("StartURLs not updated: %v", reloaded.StartURLs)
	}
}

func TestApplyUpdateClearGroupIDAndTags(t *testing.T) {
	bp := newTestBP(t)
	p, _ := bp.Create(ProfileConfig{Name: "gtags", GroupID: "g1", Tags: []string{"t1"}})
	empty := ""
	newTags := []string{"t2"}
	bp.Update(p.ID, ProfileUpdate{GroupID: &empty, Tags: &newTags})
	reloaded, _ := bp.Get(p.ID)
	if reloaded == nil || reloaded.GroupID != "" {
		t.Fatalf("GroupID not cleared, got %q", reloaded.GroupID)
	}
	if len(reloaded.Tags) != 1 || reloaded.Tags[0] != "t2" {
		t.Fatalf("Tags not updated: %v", reloaded.Tags)
	}
}

func TestProfileDataPathInvalidID(t *testing.T) {
	bp := newTestBP(t)
	if got := bp.profileDataPath("../etc"); got != "" {
		t.Fatalf("profileDataPath invalid = %q, want empty", got)
	}
}

func TestLaunchOrchestrationClose(t *testing.T) {
	bp := newTestBP(t)
	if err := bp.Close("any-id"); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := bp.CloseAll(); err != nil {
		t.Fatalf("CloseAll returned error: %v", err)
	}
}

func TestListGroupsWithBadFiles(t *testing.T) {
	bp := newTestBP(t)
	g, _ := bp.CreateGroup("g", "")
	// Write a non-JSON file into groups dir to cover the skip-invalid branch.
	_ = os.WriteFile(filepath.Join(bp.groupsPath, "bad.json"), []byte("not json"), 0o644)
	// Write a non-json file to exercise non-.json skip branch.
	_ = os.WriteFile(filepath.Join(bp.groupsPath, "skip.txt"), []byte("x"), 0o644)
	groups, err := bp.ListGroups()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, gg := range groups {
		if gg.ID == g.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("valid group missing, got %+v", groups)
	}
}
