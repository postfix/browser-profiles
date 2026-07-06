package browserprofiles

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// withGeoIPBase points geoIPBaseURL at u for the duration of the test and
// restores the production value afterwards.
func withGeoIPBase(t *testing.T, u string) {
	t.Helper()
	orig := geoIPBaseURL
	geoIPBaseURL = u
	t.Cleanup(func() { geoIPBaseURL = orig })
}

// TestDetectTimezoneFromIP exercises the three observable outcomes of the ip-api
// lookup, driven through an httptest server via the geoIPBaseURL seam.
func TestDetectTimezoneFromIP(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantGeo *GeoInfo // nil => expect (nil GeoInfo)
		wantErr bool
	}{
		{
			name:    "success maps every field (Region <- regionName)",
			body:    `{"status":"success","timezone":"America/Chicago","country":"US","regionName":"IL","city":"Chicago"}`,
			wantGeo: &GeoInfo{Timezone: "America/Chicago", Region: "IL", Country: "US", City: "Chicago"},
		},
		{
			name:    "status fail yields nil,nil",
			body:    `{"status":"fail"}`,
			wantGeo: nil,
		},
		{
			name:    "malformed body yields nil,err",
			body:    `{not json`,
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()
			withGeoIPBase(t, srv.URL)

			gi, err := DetectTimezoneFromIP("1.2.3.4")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got gi=%+v err=nil", gi)
				}
				if gi != nil {
					t.Fatalf("want nil GeoInfo alongside error, got %+v", gi)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantGeo == nil {
				if gi != nil {
					t.Fatalf("want nil GeoInfo, got %+v", gi)
				}
				return
			}
			if gi == nil {
				t.Fatal("want non-nil GeoInfo, got nil")
			}
			if *gi != *tc.wantGeo {
				t.Fatalf("GeoInfo = %+v, want %+v", *gi, *tc.wantGeo)
			}
		})
	}
}

// TestResolveEnvTimezone pins the three-way precedence: profile.Timezone beats
// the proxy geo-IP lookup, which beats the system zone.
func TestResolveEnvTimezone(t *testing.T) {
	t.Run("profile timezone wins and geo-IP is never queried", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			// If precedence is broken and geo-IP is consulted, fail loudly.
			t.Errorf("geo-IP must NOT be queried when profile.Timezone is set")
			_, _ = io.WriteString(w, `{"status":"success","timezone":"Should/NotUse"}`)
		}))
		defer srv.Close()
		withGeoIPBase(t, srv.URL)

		p := &StoredProfile{ProfileConfig: ProfileConfig{
			Timezone: "Asia/Tokyo",
			Proxy:    &ProxyConfig{Type: "http", Host: "9.9.9.9", Port: 8080},
		}}
		if got := resolveEnvTimezone(p); got != "Asia/Tokyo" {
			t.Fatalf("resolveEnvTimezone = %q, want Asia/Tokyo", got)
		}
	})

	t.Run("proxy geo-IP used when timezone empty", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"status":"success","timezone":"Europe/Paris"}`)
		}))
		defer srv.Close()
		withGeoIPBase(t, srv.URL)

		p := &StoredProfile{ProfileConfig: ProfileConfig{
			Proxy: &ProxyConfig{Type: "http", Host: "9.9.9.9", Port: 8080},
		}}
		if got := resolveEnvTimezone(p); got != "Europe/Paris" {
			t.Fatalf("resolveEnvTimezone = %q, want Europe/Paris", got)
		}
	})

	t.Run("system zone used when no timezone and no proxy", func(t *testing.T) {
		t.Setenv("TZ", "Foo/Bar")
		p := &StoredProfile{ProfileConfig: ProfileConfig{}}
		if got := resolveEnvTimezone(p); got != "Foo/Bar" {
			t.Fatalf("resolveEnvTimezone = %q, want Foo/Bar", got)
		}
	})
}

// TestSystemTimezoneFromEnv proves systemTimezone honours $TZ.
func TestSystemTimezoneFromEnv(t *testing.T) {
	t.Setenv("TZ", "Foo/Bar")
	if got := systemTimezone(); got != "Foo/Bar" {
		t.Fatalf("systemTimezone = %q, want Foo/Bar", got)
	}
}

// TestAutoDetectTimezoneNilProxy pins the nil-proxy branch: no panic, and the
// documented fallback zone is returned.
func TestAutoDetectTimezoneNilProxy(t *testing.T) {
	if got := AutoDetectTimezone(nil); got != "America/New_York" {
		t.Fatalf("AutoDetectTimezone(nil) = %q, want America/New_York", got)
	}
}

// TestAutoDetectTimezoneWithProxy mocks the geo-IP server and covers both success
// and failure branches.
func TestAutoDetectTimezoneWithProxy(t *testing.T) {
	t.Run("success from proxy IP", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"status":"success","timezone":"Europe/Berlin"}`)
		}))
		defer srv.Close()
		withGeoIPBase(t, srv.URL)

		proxy := &ProxyConfig{Type: "http", Host: "1.2.3.4", Port: 8080}
		if got := AutoDetectTimezone(proxy); got != "Europe/Berlin" {
			t.Fatalf("AutoDetectTimezone = %q, want Europe/Berlin", got)
		}
	})

	t.Run("geo-IP failure falls back to America/New_York", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"status":"fail"}`)
		}))
		defer srv.Close()
		withGeoIPBase(t, srv.URL)

		proxy := &ProxyConfig{Type: "http", Host: "1.2.3.4", Port: 8080}
		if got := AutoDetectTimezone(proxy); got != "America/New_York" {
			t.Fatalf("AutoDetectTimezone fallback = %q, want America/New_York", got)
		}
	})
}
