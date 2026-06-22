package world

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

// defaultStatusBody is served when Config.StatusFile is unset. It mirrors the
// shape of the distribution's Release/Common/serv00.htm (a small numeric status
// list the client parses for the channel-selection screen).
var defaultStatusBody = []byte("4\n-1\n-1\n-1\n-1\n-1\n-1\n-1\n-1\n-1\n")

// handleConn classifies a freshly accepted connection before it becomes a game
// session. The WYD client probes the server over HTTP ("GET /serv00.htm") to
// read channel status, and only then opens a separate CPSock game connection.
// We peek the first bytes: an HTTP method is answered with the status page and
// closed; anything else (the CPSock INITCODE) becomes a normal session, with the
// peeked bytes replayed to the framer. Runs in its own goroutine per connection
// so a slow/idle probe never stalls the accept loop.
func (w *World) handleConn(c net.Conn) {
	ip := c.RemoteAddr().String()
	br := bufio.NewReader(c)
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	pre, err := br.Peek(4)
	if err != nil {
		// A connection that opens but sends no data before timing out/closing —
		// e.g. a port probe, OR a client that expects the SERVER to speak first.
		// Logged (not swallowed) so the client-edge handshake can be mapped.
		w.log.Info("connection sent no data", "ip", ip, "err", err.Error())
		_ = c.Close()
		return
	}
	_ = c.SetReadDeadline(time.Time{}) // clear for the (possibly long-lived) session

	if isHTTPMethod(pre) {
		w.serveStatus(c, br)
		return
	}

	// CPSock: hand to the loop, replaying the peeked bytes via the buffered reader.
	w.log.Info("cpsock connection", "ip", ip, "first4", fmt.Sprintf("% x", pre))
	if !w.emit(connectEvent{conn: &prefixConn{Conn: c, r: br}, ip: ip}) {
		_ = c.Close()
	}
}

// isHTTPMethod reports whether the leading bytes look like an HTTP request. The
// CPSock stream always starts with INITCODE (0x1F11F311 = bytes 11 F3 11 1F), so
// there is no collision with these ASCII method prefixes.
func isHTTPMethod(b []byte) bool {
	switch string(b) {
	case "GET ", "POST", "HEAD":
		return true
	}
	return false
}

// serveStatus answers the client's HTTP status probe with the channel-status
// page, then closes. It reads and logs the request line (the path the client
// asks for at each step of the connect flow) so the edge behaviour can be mapped.
func (w *World) serveStatus(c net.Conn, br *bufio.Reader) {
	defer func() { _ = c.Close() }()
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	reqLine, _ := br.ReadString('\n')
	reqLine = strings.TrimRight(reqLine, "\r\n")

	body := w.statusBody()
	_ = c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	// Mirror a standard web server's response (matches the working snalmir WYD
	// server: Date + Server headers), in case the client's HTTP parser is picky.
	resp := fmt.Sprintf(
		"HTTP/1.1 200 OK\r\nDate: %s\r\nServer: w2pp\r\nContent-Length: %d\r\nConnection: close\r\n\r\n",
		time.Now().UTC().Format(time.RFC1123), len(body))
	if _, err := c.Write(append([]byte(resp), body...)); err != nil {
		w.log.Warn("status probe write failed", "ip", c.RemoteAddr().String(), "err", err)
		return
	}
	w.log.Info("served status probe", "ip", c.RemoteAddr().String(), "req", reqLine, "bytes", len(body))
}

// statusBody returns the status page bytes: the configured file when set and
// readable, otherwise the built-in default. Read per request so the file can be
// edited and re-tested without restarting the server.
func (w *World) statusBody() []byte {
	if w.cfg.StatusFile != "" {
		if b, err := os.ReadFile(w.cfg.StatusFile); err == nil {
			return b
		}
	}
	return defaultStatusBody
}

// prefixConn is a net.Conn whose reads come from a buffered reader (so bytes
// peeked during protocol detection are replayed to the framer) while writes and
// close go straight to the underlying connection.
type prefixConn struct {
	net.Conn
	r io.Reader
}

func (p *prefixConn) Read(b []byte) (int, error) { return p.r.Read(b) }
