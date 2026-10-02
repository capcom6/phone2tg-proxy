package telegram_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeHost is the API host telebot hardcodes, telebot.DefaultApiURL. The fake
// serves that name, so the URL the client requests is unchanged from production.
const fakeHost = "api.telegram.org"

// fakeBotUsername is the identity the fake API reports for getMe. Nothing else
// can produce it, so seeing it on the bot proves the call was answered locally.
const fakeBotUsername = "test_bot"

// telegramGetMeMethod is the single method telebot calls while building a bot.
const telegramGetMeMethod = "getMe"

// getMeResponse is the Bot API envelope telebot's extractOk decodes: ok=true
// carries the result, so bot.Me is populated from it.
const getMeResponse = `{"ok":true,"result":{"id":424242,"is_bot":true,` +
	`"first_name":"test","username":"test_bot"}}`

// loopbackEphemeral asks the kernel for a free loopback port, which is what
// keeps parallel and repeated runs free of collisions.
const loopbackEphemeral = "127.0.0.1:0"

// certificateSerialNumber is the serial of the single self-signed certificate
// minted per fake. It is not a certificate authority, so the value is
// arbitrary.
const certificateSerialNumber = 1

// certificateValidity is how long the minted certificate stays valid on either
// side of now.
const certificateValidity = time.Hour

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

// fakeAPI is a local stand-in for api.telegram.org plus the SOCKS5 proxy that
// reaches it. It exists so the production graph can boot end to end without a
// single packet leaving the machine: the graph's client still resolves
// api.telegram.org and still speaks TLS to it, but the name is served by a
// loopback listener and its certificate is trusted through the app's own
// httpfx.Config provider.
type fakeAPI struct {
	server   *httptest.Server
	listener net.Listener
	caPEM    string
	proxyURL string
	serving  sync.WaitGroup
}

// newFakeAPI starts the fake API and its proxy. Both are shut down through
// t.Cleanup.
func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()

	caPEM, cert := mintCertificate(t)

	server := httptest.NewUnstartedServer(http.HandlerFunc(serveGetMe))
	serverTLS := new(tls.Config)
	serverTLS.MinVersion = tls.VersionTLS12
	serverTLS.Certificates = []tls.Certificate{cert}
	server.TLS = serverTLS
	server.StartTLS()

	listener, err := net.Listen("tcp", loopbackEphemeral)
	if err != nil {
		server.Close()
		t.Fatalf("listen for the SOCKS5 proxy: %v", err)
	}

	api := &fakeAPI{
		server:   server,
		listener: listener,
		caPEM:    caPEM,
		proxyURL: "socks5://" + listener.Addr().String(),
		serving:  sync.WaitGroup{},
	}

	api.serving.Add(1)

	go api.serveProxy()

	t.Cleanup(api.close)

	return api
}

// ProxyURL is the socks5 URL that routes to the fake API.
func (f *fakeAPI) ProxyURL() string {
	return f.proxyURL
}

// CACertPEM is the certificate to trust in httpfx.Config.
func (f *fakeAPI) CACertPEM() string {
	return f.caPEM
}

// close stops the proxy listener, waits for the accept loop to leave and shuts
// the API down. Connections still parked in the client transport are not waited
// for: they hold no test state, and the test that made them is already over.
func (f *fakeAPI) close() {
	_ = f.listener.Close()
	f.serving.Wait()
	f.server.Close()
}

// serveProxy accepts SOCKS5 connections until the listener is closed.
func (f *fakeAPI) serveProxy() {
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
func (f *fakeAPI) tunnel(client net.Conn) {
	defer func() { _ = client.Close() }()

	if !socksHandshake(client) {
		return
	}

	upstream, err := net.Dial("tcp", f.server.Listener.Addr().String())
	if err != nil {
		_ = writeSocksReply(client, socksReplyDenied)

		return
	}

	defer func() { _ = upstream.Close() }()

	if writeErr := writeSocksReply(client, socksReplyOK); writeErr != nil {
		return
	}

	done := make(chan struct{}, 2)

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

	_, err := client.Write(reply)

	return err
}

// serveGetMe answers the one method telebot calls while building a bot. Any
// other method is a 404, so an unexpected call cannot be mistaken for success.
func serveGetMe(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, "/"+telegramGetMeMethod) {
		http.NotFound(w, r)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, getMeResponse)
}

// mintCertificate returns a PEM root and the matching key pair for a
// self-signed certificate valid for fakeHost. One certificate is both the leaf
// the fake presents and the root the client trusts, which is why it is its own
// issuer.
func mintCertificate(t *testing.T) (string, tls.Certificate) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate the fake API key: %v", err)
	}

	template := new(x509.Certificate)
	template.SerialNumber = big.NewInt(certificateSerialNumber)
	subject := new(pkix.Name)
	subject.CommonName = fakeHost
	template.Subject = *subject
	template.NotBefore = time.Now().Add(-certificateValidity)
	template.NotAfter = time.Now().Add(certificateValidity)
	template.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign
	template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	template.BasicConstraintsValid = true
	template.IsCA = true
	template.DNSNames = []string{fakeHost}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create the fake API certificate: %v", err)
	}

	certBlock := new(pem.Block)
	certBlock.Type = "CERTIFICATE"
	certBlock.Bytes = der
	certPEM := pem.EncodeToMemory(certBlock)

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal the fake API key: %v", err)
	}

	keyBlock := new(pem.Block)
	keyBlock.Type = "EC PRIVATE KEY"
	keyBlock.Bytes = keyDER
	keyPEM := pem.EncodeToMemory(keyBlock)

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("assemble the fake API key pair: %v", err)
	}

	return string(certPEM), cert
}
