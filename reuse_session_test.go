package browserprofiles

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/postfix/browser-profiles/fingerprint"
)

// serverPort extracts the TCP port an httptest server is listening on.
func serverPort(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url %q: %v", srv.URL, err)
	}
	_, p := hostPort(t, u.Host)
	return int(p)
}

// TestTryConnectExisting covers the /json/version probe: only a live 200 carrying
// a non-empty webSocketDebuggerUrl yields a reconnect; every other outcome is nil.
func TestTryConnectExisting(t *testing.T) {
	t.Run("live 200 with ws returns probed endpoint", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/json/version" {
				t.Errorf("probed unexpected path %q, want /json/version", r.URL.Path)
			}
			_, _ = io.WriteString(w, `{"webSocketDebuggerUrl":"ws://x/y"}`)
		}))
		defer srv.Close()
		port := serverPort(t, srv)

		got := tryConnectExisting(&browserLockInfo{Port: port})
		if got == nil {
			t.Fatal("tryConnectExisting = nil, want non-nil for a live browser")
		}
		if got.WsEndpoint != "ws://x/y" {
			t.Fatalf("WsEndpoint = %q, want ws://x/y", got.WsEndpoint)
		}
		if got.Port != port {
			t.Fatalf("Port = %d, want %d", got.Port, port)
		}
	})

	t.Run("dead port returns nil", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		port := serverPort(t, srv)
		srv.Close() // nothing is listening on port now
		if got := tryConnectExisting(&browserLockInfo{Port: port}); got != nil {
			t.Fatalf("dead port: got %+v, want nil", got)
		}
	})

	t.Run("http 500 returns nil", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		if got := tryConnectExisting(&browserLockInfo{Port: serverPort(t, srv)}); got != nil {
			t.Fatalf("500 response: got %+v, want nil", got)
		}
	})

	t.Run("empty webSocketDebuggerUrl returns nil", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{}`)
		}))
		defer srv.Close()
		if got := tryConnectExisting(&browserLockInfo{Port: serverPort(t, srv)}); got != nil {
			t.Fatalf("empty ws: got %+v, want nil", got)
		}
	})
}

// TestCreateSessionPersistentNotImplemented pins the deferred-feature contract:
// Temporary=false errors out BEFORE any launch (so it stays Chrome-free).
func TestCreateSessionPersistentNotImplemented(t *testing.T) {
	sess, err := CreateSession(CreateSessionOptions{Temporary: new(false)})
	if err == nil {
		t.Fatal("want error for Temporary=false, got nil")
	}
	if sess != nil {
		t.Fatalf("want nil session, got %+v", sess)
	}
	if !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("error = %q, want it to contain 'not implemented'", err.Error())
	}
}

// TestPatchPageScriptSubset pins the documented PatchPage SUBSET: navigator
// override (always) + WebRTC + automation bypass, and NEVER canvas/webgl/audio.
func TestPatchPageScriptSubset(t *testing.T) {
	got := patchPageScript(PatchPageOptions{})

	wantNav := fingerprint.CreateNavigatorScript(fingerprint.NavigatorConfig{
		Language: "en-US", Platform: "Win32", HardwareConcurrency: 8, DeviceMemory: 8,
	})
	if !strings.Contains(got, wantNav) {
		t.Fatal("default patchPageScript is missing the navigator override")
	}
	if !strings.Contains(got, fingerprint.WebRTCProtectionScript) {
		t.Fatal("default patchPageScript is missing WebRTC protection")
	}
	if !strings.Contains(got, fingerprint.AutomationBypassScript) {
		t.Fatal("default patchPageScript is missing automation bypass")
	}
	for name, script := range map[string]string{
		"canvas": fingerprint.CanvasProtectionScript,
		"webgl":  fingerprint.WebGLProtectionScript,
		"audio":  fingerprint.AudioProtectionScript,
	} {
		if strings.Contains(got, script) {
			t.Errorf("patchPageScript must NOT contain %s protection (subset violated)", name)
		}
	}
}

// TestPatchPageScriptWebGLDead pins the deliberately dead WebGL toggle: flipping
// it must not change the output, and WebGL protection is never emitted.
func TestPatchPageScriptWebGLDead(t *testing.T) {
	on := patchPageScript(PatchPageOptions{WebGL: new(true)})
	off := patchPageScript(PatchPageOptions{WebGL: new(false)})
	if on != off {
		t.Fatal("WebGL toggle is documented dead but altered the output")
	}
	if strings.Contains(on, fingerprint.WebGLProtectionScript) {
		t.Fatal("WebGL protection injected despite the dead flag")
	}
}

// TestPatchPageScriptWebRTCOff pins the WebRTC toggle: WebRTC=false drops WebRTC
// protection while automation bypass (webdriver/chrome/plugins default true) stays.
func TestPatchPageScriptWebRTCOff(t *testing.T) {
	got := patchPageScript(PatchPageOptions{WebRTC: new(false)})
	if strings.Contains(got, fingerprint.WebRTCProtectionScript) {
		t.Fatal("WebRTC=false must omit WebRTC protection")
	}
	if !strings.Contains(got, fingerprint.AutomationBypassScript) {
		t.Fatal("automation bypass should remain when only WebRTC is disabled")
	}
}
