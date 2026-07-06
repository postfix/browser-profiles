package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	browserprofiles "github.com/postfix/browser-profiles"
)

// runCLI builds a fresh command tree, runs it with args, and returns everything
// written to os.Stdout. The commands print via fmt.Print* to os.Stdout (not
// cmd.OutOrStdout), and cobra's --version template also targets os.Stdout, so we
// swap os.Stdout for an os.Pipe and drain it.
//
// HAPPY paths only: fail/failMsg call os.Exit(1), which would kill the test
// process, so error/not-found flows are never exercised in-process.
func runCLI(t *testing.T, args ...string) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	root := newRootCmd()
	root.SetArgs(args)
	execErr := root.Execute()

	_ = w.Close()
	os.Stdout = orig
	out := <-done
	_ = r.Close()

	if execErr != nil {
		t.Fatalf("Execute(%v): %v", args, execErr)
	}
	return out
}

// useTempProfiles repoints the package-global manager at a fresh temp store for
// the test and restores the original afterwards.
func useTempProfiles(t *testing.T) {
	t.Helper()
	orig := profiles
	profiles = browserprofiles.NewBrowserProfiles(browserprofiles.BrowserProfilesOptions{StoragePath: t.TempDir()})
	t.Cleanup(func() { profiles = orig })
}

func TestCLIVersion(t *testing.T) {
	useTempProfiles(t)
	if got := runCLI(t, "--version"); got != "0.2.12\n" {
		t.Fatalf("--version = %q, want %q", got, "0.2.12\n")
	}
}

func TestCLIListEmptyJSON(t *testing.T) {
	useTempProfiles(t)
	if got := runCLI(t, "list", "--json"); got != "[]\n" {
		t.Fatalf("list --json (empty store) = %q, want %q", got, "[]\n")
	}
}

func TestCLICreateParsesProxyAndLists(t *testing.T) {
	useTempProfiles(t)

	out := runCLI(t, "create", "foo", "-p", "http://user:pass@host:9000", "-t", "America/New_York")
	if !strings.Contains(out, "created") {
		t.Fatalf("create output missing 'created':\n%s", out)
	}
	if !strings.Contains(out, "host:9000") {
		t.Fatalf("create output missing parsed proxy host:9000:\n%s", out)
	}

	listed := runCLI(t, "list", "--json")
	var profs []map[string]any
	if err := json.Unmarshal([]byte(listed), &profs); err != nil {
		t.Fatalf("list --json is not valid JSON: %v\n%s", err, listed)
	}
	if len(profs) != 1 {
		t.Fatalf("want 1 profile after create, got %d:\n%s", len(profs), listed)
	}
	if profs[0]["name"] != "foo" {
		t.Fatalf("listed profile name = %v, want foo", profs[0]["name"])
	}
}

// TestCLICreateDefaultProxyPort pins the proxy-URL parse branch where the port is
// omitted: it resolves to 8080.
func TestCLICreateDefaultProxyPort(t *testing.T) {
	useTempProfiles(t)

	_ = runCLI(t, "create", "bar", "-p", "http://h")
	out := runCLI(t, "info", "bar", "--json")

	var prof map[string]any
	if err := json.Unmarshal([]byte(out), &prof); err != nil {
		t.Fatalf("info --json is not valid JSON: %v\n%s", err, out)
	}
	proxy, ok := prof["proxy"].(map[string]any)
	if !ok {
		t.Fatalf("proxy is missing or not an object in info output:\n%s", out)
	}
	if got, _ := proxy["port"].(float64); got != 8080 {
		t.Fatalf("proxy port = %v, want 8080 (omitted-port default)", proxy["port"])
	}
	if proxy["host"] != "h" {
		t.Fatalf("proxy host = %v, want h", proxy["host"])
	}
}

func TestCLIInfoJSON(t *testing.T) {
	useTempProfiles(t)

	_ = runCLI(t, "create", "baz", "--id", "baz-id-123")
	out := runCLI(t, "info", "baz-id-123", "--json")

	var prof map[string]any
	if err := json.Unmarshal([]byte(out), &prof); err != nil {
		t.Fatalf("info --json is not valid JSON: %v\n%s", err, out)
	}
	if prof["id"] != "baz-id-123" {
		t.Fatalf("info id = %v, want baz-id-123", prof["id"])
	}
	if prof["name"] != "baz" {
		t.Fatalf("info name = %v, want baz", prof["name"])
	}
}

func TestCLIPath(t *testing.T) {
	useTempProfiles(t)
	t.Setenv("HOME", "/tmp/x")
	out := runCLI(t, "path")
	if !strings.Contains(out, "/tmp/x/.aitofy/browser-profiles") {
		t.Fatalf("path output missing $HOME-based storage path:\n%s", out)
	}
}

func TestCLIDeleteForce(t *testing.T) {
	useTempProfiles(t)

	_ = runCLI(t, "create", "todelete")
	out := runCLI(t, "delete", "todelete", "--force")
	if !strings.Contains(out, "deleted") {
		t.Fatalf("delete output missing 'deleted':\n%s", out)
	}

	if got := runCLI(t, "list", "--json"); got != "[]\n" {
		t.Fatalf("after delete, list --json = %q, want []", got)
	}
}
