package browserprofiles

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestProfileIDTraversalRejected proves the path-traversal fix: mutating/reading methods
// must not escape the profiles/ subtree via a "../" id (the BLOCKER finding).
func TestProfileIDTraversalRejected(t *testing.T) {
	root := t.TempDir()
	bp := NewBrowserProfiles(BrowserProfilesOptions{StoragePath: filepath.Join(root, "store")})

	// A victim directory OUTSIDE the profile store.
	victim := filepath.Join(root, "victim")
	if err := os.MkdirAll(victim, 0o755); err != nil {
		t.Fatal(err)
	}
	victimFile := filepath.Join(victim, "important.txt")
	if err := os.WriteFile(victimFile, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"../victim", "../../victim", "..", "a/../../victim", "foo/bar"} {
		if ok, err := bp.Delete(id); ok || err != nil {
			t.Fatalf("Delete(%q) should be a rejected no-op, got ok=%v err=%v", id, ok, err)
		}
		if p, err := bp.Get(id); p != nil || err != nil {
			t.Fatalf("Get(%q) should return (nil,nil), got p=%v err=%v", id, p, err)
		}
		if ok, _ := bp.DeleteGroup(id); ok {
			t.Fatalf("DeleteGroup(%q) should be a rejected no-op", id)
		}
	}

	// The victim must be untouched.
	if _, err := os.Stat(victimFile); err != nil {
		t.Fatalf("victim file was affected by traversal: %v", err)
	}

	// A valid id still works end-to-end.
	p, err := bp.Create(ProfileConfig{ID: "valid-id_1", Name: "ok"})
	if err != nil {
		t.Fatalf("valid create: %v", err)
	}
	if got, _ := bp.Get(p.ID); got == nil || got.ID != "valid-id_1" {
		t.Fatalf("valid Get failed: %v", got)
	}
	if ok, _ := bp.Delete(p.ID); !ok {
		t.Fatal("valid Delete failed")
	}
}

func TestResolveProxyPartialCredentials(t *testing.T) {
	if _, _, err := resolveProxy(&ProxyConfig{Type: "http", Host: "h", Port: 8080, Username: "u"}); err == nil {
		t.Fatal("username-only proxy should error")
	}
	if _, _, err := resolveProxy(&ProxyConfig{Type: "http", Host: "h", Port: 8080, Password: "p"}); err == nil {
		t.Fatal("password-only proxy should error")
	}
	if url, cleanup, err := resolveProxy(&ProxyConfig{Type: "http", Host: "h", Port: 8080}); err != nil || url != "http://h:8080" || cleanup != nil {
		t.Fatalf("no-creds proxy: url=%q cleanupNil=%v err=%v", url, cleanup == nil, err)
	}
}

func TestBrowserTrackingLifecycle(t *testing.T) {
	const id = "track-test-id"
	closed := 0
	lr := &LaunchResult{ProfileID: id}
	lr.Close = func() error { closed++; untrackBrowser(id); return nil }

	trackBrowser(id, lr)
	if getTracked(id) != lr {
		t.Fatal("getTracked mismatch")
	}
	if !slices.Contains(GetRunningBrowsers(), id) {
		t.Fatal("GetRunningBrowsers missing id")
	}
	if !CloseBrowser(id) {
		t.Fatal("CloseBrowser should return true")
	}
	if closed != 1 {
		t.Fatalf("Close called %d times", closed)
	}
	if getTracked(id) != nil {
		t.Fatal("id still tracked after close")
	}
	if CloseBrowser(id) {
		t.Fatal("second CloseBrowser should return false")
	}
}
