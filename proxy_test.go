package browserprofiles

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func hostPort(t *testing.T, addr string) (string, Port) {
	t.Helper()
	h, ps, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := strconv.Atoi(ps)
	return h, Port(p)
}

func basicAuth(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

// TestForwardProxyHTTPPlain: plain HTTP request through an authenticated HTTP upstream proxy.
func TestForwardProxyHTTPPlain(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "TARGET_OK")
	}))
	defer target.Close()

	sawAuth := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Proxy-Authorization") == basicAuth("user", "pass") {
			sawAuth = true
		}
		resp, err := http.Get(r.URL.String()) // absolute-URI proxy request
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		_, _ = w.Write(b)
	}))
	defer upstream.Close()

	uu, _ := url.Parse(upstream.URL)
	h, p := hostPort(t, uu.Host)
	localURL, cleanup, err := resolveProxy(&ProxyConfig{Type: "http", Host: h, Port: p, Username: "user", Password: "pass"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	pu, _ := url.Parse(localURL)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(pu)}}
	resp, err := client.Get(target.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "TARGET_OK" {
		t.Fatalf("body = %q", body)
	}
	if !sawAuth {
		t.Fatal("upstream did not see injected Proxy-Authorization")
	}
}

// TestForwardProxyHTTPConnect: HTTPS (CONNECT) through an authenticated HTTP upstream proxy.
func TestForwardProxyHTTPConnect(t *testing.T) {
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "TLS_OK")
	}))
	defer target.Close()

	sawAuth := false
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				br := bufio.NewReader(c)
				req, err := http.ReadRequest(br)
				if err != nil || req.Method != http.MethodConnect {
					return
				}
				if req.Header.Get("Proxy-Authorization") != basicAuth("user", "pass") {
					_, _ = io.WriteString(c, "HTTP/1.1 407 Proxy Authentication Required\r\n\r\n")
					return
				}
				sawAuth = true
				tc, err := net.Dial("tcp", req.Host)
				if err != nil {
					_, _ = io.WriteString(c, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
					return
				}
				defer tc.Close()
				_, _ = io.WriteString(c, "HTTP/1.1 200 Connection Established\r\n\r\n")
				tunnel(c, tc, br)
			}(c)
		}
	}()

	h, p := hostPort(t, ln.Addr().String())
	localURL, cleanup, err := resolveProxy(&ProxyConfig{Type: "http", Host: h, Port: p, Username: "user", Password: "pass"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	pu, _ := url.Parse(localURL)
	client := &http.Client{Transport: &http.Transport{
		Proxy:           http.ProxyURL(pu),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	resp, err := client.Get(target.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "TLS_OK" {
		t.Fatalf("body = %q", body)
	}
	if !sawAuth {
		t.Fatal("upstream did not see CONNECT Proxy-Authorization")
	}
}

// fakeSOCKS5 is a minimal SOCKS5 server: username/password auth + CONNECT.
func fakeSOCKS5(t *testing.T, wantUser, wantPass string, authSeen *bool) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				br := bufio.NewReader(c)
				ver, _ := br.ReadByte()
				nm, _ := br.ReadByte()
				methods := make([]byte, nm)
				if _, err := io.ReadFull(br, methods); err != nil || ver != 5 {
					return
				}
				_, _ = c.Write([]byte{5, 2}) // choose username/password
				av, _ := br.ReadByte()
				ul, _ := br.ReadByte()
				u := make([]byte, ul)
				io.ReadFull(br, u)
				pl, _ := br.ReadByte()
				p := make([]byte, pl)
				io.ReadFull(br, p)
				_ = av
				if string(u) != wantUser || string(p) != wantPass {
					_, _ = c.Write([]byte{1, 1})
					return
				}
				*authSeen = true
				_, _ = c.Write([]byte{1, 0})
				hdr := make([]byte, 4)
				if _, err := io.ReadFull(br, hdr); err != nil || hdr[1] != 1 {
					return
				}
				var host string
				switch hdr[3] {
				case 1:
					a := make([]byte, 4)
					io.ReadFull(br, a)
					host = net.IP(a).String()
				case 3:
					l, _ := br.ReadByte()
					d := make([]byte, l)
					io.ReadFull(br, d)
					host = string(d)
				case 4:
					a := make([]byte, 16)
					io.ReadFull(br, a)
					host = net.IP(a).String()
				}
				pb := make([]byte, 2)
				io.ReadFull(br, pb)
				port := int(pb[0])<<8 | int(pb[1])
				tc, err := net.Dial("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
				if err != nil {
					_, _ = c.Write([]byte{5, 1, 0, 1, 0, 0, 0, 0, 0, 0})
					return
				}
				defer tc.Close()
				_, _ = c.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0})
				tunnel(c, tc, br)
			}(c)
		}
	}()
	return ln
}

// TestForwardProxySOCKS5Connect: HTTPS (CONNECT) through an authenticated SOCKS5 upstream
// (an enhancement over the TS source, which throws on SOCKS5).
func TestForwardProxySOCKS5Connect(t *testing.T) {
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "SOCKS_OK")
	}))
	defer target.Close()

	authSeen := false
	ln := fakeSOCKS5(t, "user", "pass", &authSeen)
	defer ln.Close()

	h, p := hostPort(t, ln.Addr().String())
	localURL, cleanup, err := resolveProxy(&ProxyConfig{Type: "socks5", Host: h, Port: p, Username: "user", Password: "pass"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	pu, _ := url.Parse(localURL)
	client := &http.Client{Transport: &http.Transport{
		Proxy:           http.ProxyURL(pu),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
	resp, err := client.Get(target.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "SOCKS_OK" {
		t.Fatalf("body = %q", body)
	}
	if !authSeen {
		t.Fatal("SOCKS5 upstream did not see username/password auth")
	}
}

func TestNewProxyBackendKinds(t *testing.T) {
	if be, err := newProxyBackend(&ProxyConfig{Type: "socks5", Host: "h", Port: 1080, Username: "u", Password: "p"}); err != nil {
		t.Fatal(err)
	} else if _, ok := be.(*socks5Backend); !ok {
		t.Fatalf("socks5 → %T", be)
	}
	if be, err := newProxyBackend(&ProxyConfig{Type: "http", Host: "h", Port: 8080, Username: "u", Password: "p"}); err != nil {
		t.Fatal(err)
	} else if _, ok := be.(*httpBackend); !ok {
		t.Fatalf("http → %T", be)
	}
}
