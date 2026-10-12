// Package telegramtest provides the offline fake Bot API and SOCKS5 harness
// re-homed from pkg/telegram: a loopback HTTP server that answers getMe plus
// a SOCKS5 tunnel to it, so the production client path can be exercised end
// to end without a single packet leaving the machine.
//
// The server is plain HTTP on purpose: telegofx builds the production client
// (a fasthttp SOCKS5 client from telegofx.Config.ProxyURL) and exposes no CA
// seam, and macOS ignores SSL_CERT_FILE, so an offline TLS hop could never be
// trusted. Tests point telego at it with telego.WithAPIServer, the only
// deviation from the production URL.
package telegramtest

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeHost is the API host telego hardcodes, telego's defaultBotAPIServer.
// The production URL keeps this host; the SOCKS tunnel ignores the requested
// destination and dials the loopback fake, so the name never resolves.
const fakeHost = "api.telegram.org"

// APIServerURL is the plain-HTTP form of telego's default API server. Tests
// pass it to telego.WithAPIServer so the offline client speaks plain HTTP to
// the tunnel instead of TLS, which telegofx's fasthttp client cannot be
// configured to trust.
const APIServerURL = "http://" + fakeHost

// FakeBotUsername is the identity the fake API reports for getMe. Nothing else
// can produce it, so seeing it on a bot's GetMe result proves the call was
// answered locally.
const FakeBotUsername = "test_bot"

// telegramGetMeMethod is the single method the fake answers while building a
// bot identity.
const telegramGetMeMethod = "getMe"

// getMeResponse is the Bot API envelope the fake serves: ok=true carries the
// result, so bot.GetMe decodes it into the fake identity.
const getMeResponse = `{"ok":true,"result":{"id":424242,"is_bot":true,` +
	`"first_name":"test","username":"` + FakeBotUsername + `"}}`

// loopbackEphemeral asks the kernel for a free loopback port, which is what
// keeps parallel and repeated runs free of collisions.
const loopbackEphemeral = "127.0.0.1:0"

// tunnelDoneSlots is the two copy goroutines of a tunnel (upstream to client
// and client to upstream), each signalling completion exactly once.
const tunnelDoneSlots = 2

// SOCKS5 protocol constants, RFC 1928. Only the no-auth method and the CONNECT
// command are implemented, which is the whole surface x/net/proxy uses for a
// proxy URL that carries no credentials.
const (
	socksVersion     = 0x05
	socksAuthNone    = 0x00
	socksCmdConnect  = 0x01
	socksReplyOK     = 0x00
	socksReplyDenied = 0x01
	socksAtypIPv4    = 0x01
	socksAtypDomain  = 0x03
	socksAtypIPv6    = 0x04
	socksPortLen     = 2
	socksReplyLen    = 10
)

// FakeAPI is a local stand-in for api.telegram.org plus the SOCKS5 proxy that
// reaches it. It exists so the production graph can boot end to end without a
// single packet leaving the machine: the graph's client still resolves
// fakeHost through the proxy and still speaks the Bot API, but the tunnel
// answers every CONNECT with the loopback listener instead.
type FakeAPI struct {
	server   *httptest.Server
	listener net.Listener
	proxyURL string
	serving  sync.WaitGroup
}

// NewFakeAPI starts the fake API and its proxy. Both are shut down through
// t.Cleanup.
func NewFakeAPI(t *testing.T) *FakeAPI {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(serveGetMe))

	listenerCfg := new(net.ListenConfig)

	listener, err := listenerCfg.Listen(context.Background(), "tcp", loopbackEphemeral)
	if err != nil {
		server.Close()
		t.Fatalf("listen for the SOCKS5 proxy: %v", err)
	}

	api := &FakeAPI{
		server:   server,
		listener: listener,
		proxyURL: "socks5://" + listener.Addr().String(),
		serving:  sync.WaitGroup{},
	}

	api.serving.Add(1)

	go api.serveProxy()

	t.Cleanup(api.close)

	return api
}

// ProxyURL is the socks5 URL that routes to the fake API.
func (f *FakeAPI) ProxyURL() string {
	return f.proxyURL
}

// close stops the proxy listener, waits for the accept loop to leave and shuts
// the API down. Connections still parked in the client transport are not waited
// for: they hold no test state, and the test that made them is already over.
func (f *FakeAPI) close() {
	_ = f.listener.Close()
	f.serving.Wait()
	f.server.Close()
}

// serveProxy accepts SOCKS5 connections until the listener is closed.
func (f *FakeAPI) serveProxy() {
	defer f.serving.Done()

	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}

		go f.tunnel(conn)
	}
}

// tunnel answers one CONNECT and then copies bytes in both directions. The
// destination the client asks for is deliberately ignored: every CONNECT is
// answered with the local fake API, so the request never leaves the machine.
func (f *FakeAPI) tunnel(client net.Conn) {
	defer func() { _ = client.Close() }()

	if !socksHandshake(client) {
		return
	}

	dialer := new(net.Dialer)

	upstream, err := dialer.DialContext(context.Background(), "tcp", f.server.Listener.Addr().String())
	if err != nil {
		_ = writeSocksReply(client, socksReplyDenied)

		return
	}

	defer func() { _ = upstream.Close() }()

	if writeErr := writeSocksReply(client, socksReplyOK); writeErr != nil {
		return
	}

	done := make(chan struct{}, tunnelDoneSlots)

	go func() {
		_, _ = io.Copy(upstream, client)
		done <- struct{}{}
	}()

	go func() {
		_, _ = io.Copy(client, upstream)
		done <- struct{}{}
	}()

	<-done
}

// socksHandshake reads the greeting and the CONNECT request, answering both. It
// reports whether the client got as far as asking for a tunnel.
func socksHandshake(client net.Conn) bool {
	var greeting [2]byte

	if _, err := io.ReadFull(client, greeting[:]); err != nil {
		return false
	}

	if greeting[0] != socksVersion {
		return false
	}

	methods := make([]byte, greeting[1])
	if _, err := io.ReadFull(client, methods); err != nil {
		return false
	}

	if _, err := client.Write([]byte{socksVersion, socksAuthNone}); err != nil {
		return false
	}

	var request [4]byte
	if _, err := io.ReadFull(client, request[:]); err != nil {
		return false
	}

	if request[0] != socksVersion || request[1] != socksCmdConnect {
		return false
	}

	return readSocksAddress(client, request[3])
}

// readSocksAddress consumes the destination the client asked to reach. The
// trailing port is present in every address type, so it is consumed with it.
func readSocksAddress(client net.Conn, atyp byte) bool {
	var length int

	switch atyp {
	case socksAtypIPv4:
		length = net.IPv4len
	case socksAtypIPv6:
		length = net.IPv6len
	case socksAtypDomain:
		var size [1]byte

		if _, err := io.ReadFull(client, size[:]); err != nil {
			return false
		}

		length = int(size[0])
	default:
		return false
	}

	if _, err := io.ReadFull(client, make([]byte, length+socksPortLen)); err != nil {
		return false
	}

	return true
}

// writeSocksReply answers a CONNECT with a bound IPv4 reply.
func writeSocksReply(client net.Conn, code byte) error {
	reply := make([]byte, socksReplyLen)
	reply[0] = socksVersion
	reply[1] = code
	reply[3] = socksAtypIPv4

	if _, err := client.Write(reply); err != nil {
		return fmt.Errorf("write socks reply: %w", err)
	}

	return nil
}

// serveGetMe answers the one method a test asks for while reading a bot
// identity. Any other method is a 404, so an unexpected call cannot be
// mistaken for success.
func serveGetMe(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, "/"+telegramGetMeMethod) {
		http.NotFound(w, r)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, getMeResponse)
}
