package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoldenFixtureChecksums is the F3 golden-fixture guard. The pinned set in
// testdata/checksums.txt is the sole post-cutover parity oracle, so this test
// asserts two things:
//  1. every pinned fixture still hashes to its recorded sha256, and
//  2. the set of *.js fixtures on disk EXACTLY equals the pinned set
//     (no fixture silently added or removed).
//
// Fixture-update discipline: any change to an anti-detect JavaScript string,
// builder template, or embedded script constant must (1) regenerate the affected
// testdata/*.js fixture, (2) update testdata/checksums.txt, and (3) be reviewed in
// diff because it changes the browser-injected fingerprint.
func TestGoldenFixtureChecksums(t *testing.T) {
	const dir = "testdata"

	raw, err := os.ReadFile(filepath.Join(dir, "checksums.txt"))
	if err != nil {
		t.Fatalf("read checksums.txt: %v", err)
	}

	pinned := map[string]string{} // slash-relpath -> sha256 hex
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line) // "<hex>  <relpath>"
		if len(fields) != 2 {
			t.Fatalf("malformed checksum line: %q", line)
		}
		pinned[fields[1]] = fields[0]
	}
	if len(pinned) == 0 {
		t.Fatal("checksums.txt pinned no fixtures")
	}

	// (1) Each pinned fixture matches its recorded digest.
	for rel, want := range pinned {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Errorf("pinned fixture unreadable: %s: %v", rel, err)
			continue
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("checksum mismatch for %s:\n  want %s\n  got  %s", rel, want, got)
		}
	}

	// (2) On-disk *.js set == pinned set (excludes checksums.txt / manifest.json,
	// which are not *.js).
	onDisk := map[string]bool{}
	walkErr := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".js") {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		onDisk[filepath.ToSlash(rel)] = true
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk testdata: %v", walkErr)
	}

	for rel := range pinned {
		if !onDisk[rel] {
			t.Errorf("pinned fixture is missing from disk: %s", rel)
		}
	}
	for rel := range onDisk {
		if _, ok := pinned[rel]; !ok {
			t.Errorf("unpinned *.js fixture present (add its sha256 to checksums.txt): %s", rel)
		}
	}
}
