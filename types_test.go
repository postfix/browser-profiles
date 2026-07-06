package browserprofiles

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestPortUnmarshalFromNumberAndString(t *testing.T) {
	var fromNum struct {
		P Port `json:"p"`
	}
	if err := json.Unmarshal([]byte(`{"p":8080}`), &fromNum); err != nil {
		t.Fatalf("number: %v", err)
	}
	if fromNum.P != 8080 {
		t.Fatalf("number: got %d want 8080", fromNum.P)
	}

	var fromStr struct {
		P Port `json:"p"`
	}
	if err := json.Unmarshal([]byte(`{"p":"3128"}`), &fromStr); err != nil {
		t.Fatalf("string: %v", err)
	}
	if fromStr.P != 3128 {
		t.Fatalf("string: got %d want 3128", fromStr.P)
	}
}

func TestPortUnmarshalInvalidString(t *testing.T) {
	var v struct {
		P Port `json:"p"`
	}
	if err := json.Unmarshal([]byte(`{"p":"abc"}`), &v); err == nil {
		t.Fatal("want error for non-numeric port string, got nil")
	}
}

func TestPortMarshalsAsNumber(t *testing.T) {
	b, err := json.Marshal(ProxyConfig{Type: "http", Host: "h", Port: Port(8080)})
	if err != nil {
		t.Fatal(err)
	}
	// Port must serialize as a JSON number, not a quoted string.
	if got := string(b); got != `{"type":"http","host":"h","port":8080}` {
		t.Fatalf("got %s", got)
	}
}

func TestStoredProfileRoundTrip(t *testing.T) {
	in := StoredProfile{
		ProfileConfig: ProfileConfig{
			ID:        "abc123",
			Name:      "Test",
			Proxy:     &ProxyConfig{Type: "socks5", Host: "1.2.3.4", Port: 1080, Username: "u", Password: "p"},
			Timezone:  "America/New_York",
			Cookies:   []ProfileCookie{},
			StartURLs: []string{},
			Tags:      []string{"a"},
		},
		CreatedAt: 100,
		UpdatedAt: 200,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out StoredProfile
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.ID != in.ID || out.Name != in.Name || out.CreatedAt != in.CreatedAt || out.Proxy.Port != 1080 {
		t.Fatalf("round-trip mismatch: %+v", out)
	}
}

func TestBrowserErrorUnwrap(t *testing.T) {
	cause := errors.New("root cause")
	err := &BrowserError{Code: "ERR", Message: "msg", Cause: cause}
	if !errors.Is(err, cause) {
		t.Fatal("errors.Is(BrowserError, cause) = false, want true")
	}
}

func TestBrowserErrorError(t *testing.T) {
	err := &BrowserError{Code: "ERR", Message: "the message"}
	if got := err.Error(); got != "the message" {
		t.Fatalf("Error() = %q, want %q", got, "the message")
	}
}
