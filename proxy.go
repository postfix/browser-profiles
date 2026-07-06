package browserprofiles

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	xproxy "golang.org/x/net/proxy"
)

// forwardProxy is a local HTTP proxy that Chrome connects to WITHOUT credentials and
// which forwards to an upstream proxy WITH credentials. It replaces proxy-chain's
// anonymizeProxy (Chrome's --proxy-server cannot carry user:pass), and additionally
// supports authenticated SOCKS5 upstreams (an enhancement over the TS source, which
// throws on SOCKS5).
type forwardProxy struct {
	ln       net.Listener
	localURL string
	closed   bool
	mu       sync.Mutex
}

// Close stops the forward proxy. Idempotent: second call returns nil.
func (f *forwardProxy) Close() error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed || f.ln == nil {
		return nil
	}
	f.closed = true
	return f.ln.Close()
}

// resolveProxy maps a profile's proxy to the --proxy-server value + an optional cleanup.
//   - nil proxy            → "", nil
//   - credential-free      → direct URL (Chrome dials the upstream itself)
//   - authenticated (creds)→ a local forward proxy URL; cleanup tears it down
func resolveProxy(p *ProxyConfig) (serverURL string, cleanup func() error, err error) {
	if p == nil {
		return "", nil, nil
	}
	hasUser, hasPass := p.Username != "", p.Password != ""
	if hasUser != hasPass {
		return "", nil, fmt.Errorf("proxy %s:%s has only one of username/password set (both or neither required)", p.Host, p.Port)
	}
	if !hasUser {
		return buildProxyURL(p), nil, nil
	}
	fp, err := startForwardProxy(p)
	if err != nil {
		return "", nil, err
	}
	return fp.localURL, fp.Close, nil
}

func startForwardProxy(p *ProxyConfig) (*forwardProxy, error) {
	be, err := newProxyBackend(p)
	if err != nil {
		return nil, err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	f := &forwardProxy{ln: ln, localURL: "http://" + ln.Addr().String()}
	go f.serve(be)
	return f, nil
}

func (f *forwardProxy) serve(be proxyBackend) {
	for {
		c, err := f.ln.Accept()
		if err != nil {
			return // listener closed
		}
		go handleProxyClient(c, be)
	}
}

func handleProxyClient(client net.Conn, be proxyBackend) {
	defer client.Close()
	br := bufio.NewReader(client)
	_ = client.SetReadDeadline(time.Now().Add(30 * time.Second))
	req, err := http.ReadRequest(br)
	if err != nil {
		return
	}
	// Request read; clear the deadline so long-lived tunnels/relays aren't killed.
	_ = client.SetReadDeadline(time.Time{})
	if req.Method == http.MethodConnect {
		up, err := be.dial(req.Host)
		if err != nil {
			_, _ = io.WriteString(client, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
			return
		}
		defer up.Close()
		if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
			return
		}
		tunnel(client, up, br)
		return
	}
	be.plain(client, br, req)
}

// tunnel copies bytes both ways between the client (reads via br to include buffered data)
// and the upstream target.
func tunnel(client net.Conn, up net.Conn, br *bufio.Reader) {
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(up, br)
		if tc, ok := up.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(client, up)
		if tc, ok := client.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
		done <- struct{}{}
	}()
	<-done
	<-done
}

// proxyBackend abstracts an upstream proxy (HTTP/HTTPS or SOCKS5).
type proxyBackend interface {
	dial(target string) (net.Conn, error)                       // tunnel to target host:port (CONNECT)
	plain(client net.Conn, br *bufio.Reader, req *http.Request) // absolute-URI HTTP request
}

func newProxyBackend(p *ProxyConfig) (proxyBackend, error) {
	addr := net.JoinHostPort(p.Host, p.Port.String())
	if p.Type == "socks5" {
		var auth *xproxy.Auth
		if p.Username != "" {
			auth = &xproxy.Auth{User: p.Username, Password: p.Password}
		}
		d, err := xproxy.SOCKS5("tcp", addr, auth, xproxy.Direct)
		if err != nil {
			return nil, err
		}
		return &socks5Backend{dialer: d}, nil
	}
	cred := base64.StdEncoding.EncodeToString([]byte(p.Username + ":" + p.Password))
	return &httpBackend{addr: addr, cred: cred}, nil
}

// httpBackend forwards through an upstream HTTP/HTTPS proxy, injecting Proxy-Authorization.
type httpBackend struct {
	addr string
	cred string
}

func (h *httpBackend) dial(target string) (net.Conn, error) {
	c, err := net.DialTimeout("tcp", h.addr, 20*time.Second)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(c, "CONNECT %s HTTP/1.1\r\nHost: %s\r\nProxy-Authorization: Basic %s\r\nProxy-Connection: Keep-Alive\r\n\r\n", target, target, h.cred)
	code, err := readConnectStatus(c)
	if err != nil || code != http.StatusOK {
		_ = c.Close()
		if err == nil {
			err = fmt.Errorf("upstream CONNECT returned status %d", code)
		}
		return nil, err
	}
	return c, nil
}

func (h *httpBackend) plain(client net.Conn, br *bufio.Reader, req *http.Request) {
	c, err := net.DialTimeout("tcp", h.addr, 20*time.Second)
	if err != nil {
		_, _ = io.WriteString(client, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}
	defer c.Close()
	req.Header.Set("Proxy-Authorization", "Basic "+h.cred)
	if err := req.WriteProxy(c); err != nil { // absolute-URI form for the upstream proxy
		return
	}
	_, _ = io.Copy(client, c)
}

// socks5Backend dials targets directly through an upstream SOCKS5 proxy (with optional auth).
type socks5Backend struct {
	dialer xproxy.Dialer
}

func (s *socks5Backend) dial(target string) (net.Conn, error) {
	return s.dialer.Dial("tcp", target)
}

func (s *socks5Backend) plain(client net.Conn, br *bufio.Reader, req *http.Request) {
	host := req.URL.Host
	if host == "" {
		host = req.Host
	}
	if !strings.Contains(host, ":") {
		host += ":80"
	}
	c, err := s.dialer.Dial("tcp", host)
	if err != nil {
		_, _ = io.WriteString(client, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
		return
	}
	defer c.Close()
	req.Header.Del("Proxy-Authorization")
	req.Header.Del("Proxy-Connection")
	if err := req.Write(c); err != nil { // origin-form to the target server
		return
	}
	_, _ = io.Copy(client, c)
}

// readConnectStatus reads a CONNECT response's status code without consuming tunnel bytes
// (byte-at-a-time until the header terminator).
func readConnectStatus(c net.Conn) (int, error) {
	_ = c.SetReadDeadline(time.Now().Add(20 * time.Second))
	defer c.SetReadDeadline(time.Time{})
	var buf []byte
	tmp := make([]byte, 1)
	for !bytes.HasSuffix(buf, []byte("\r\n\r\n")) {
		n, err := c.Read(tmp)
		if err != nil {
			return 0, err
		}
		if n > 0 {
			buf = append(buf, tmp[0])
		}
		if len(buf) > 8192 {
			return 0, fmt.Errorf("CONNECT response headers too large")
		}
	}
	line := string(buf)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	parts := strings.SplitN(strings.TrimSpace(line), " ", 3)
	if len(parts) < 2 {
		return 0, fmt.Errorf("malformed CONNECT status: %q", line)
	}
	return strconv.Atoi(parts[1])
}
