//go:build e2e

// End-to-end smoke against a LIVE stack (docker compose up): a raw CPSock client
// connects to the published tmServer edge, performs the INITCODE handshake and a
// real MSG_AccountLogin, and asserts the server answers MSG_CNFAccountLogin —
// proving the full chain tmServer(CPSock) → dbServer(gRPC/mTLS) → PostgreSQL.
//
// Seed the account first, then:
//
//	W2PP_E2E_ADDR=127.0.0.1:8281 W2PP_E2E_ACCOUNT=smoke W2PP_E2E_PASSWORD=smoke123 \
//	  go test -tags=e2e -run TestE2ESmokeLogin ./tmserver/internal/world/ -v
package world

import (
	"encoding/binary"
	"io"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// liveDial opens a TCP connection to the live tmServer and sends INITCODE.
func liveDial(t *testing.T, addr string) net.Conn {
	t.Helper()
	c, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	var ic [4]byte
	binary.LittleEndian.PutUint32(ic[:], protocol.InitCode)
	if _, err := c.Write(ic[:]); err != nil {
		t.Fatalf("write INITCODE: %v", err)
	}
	return c
}

// liveRead reads one S→C frame (2-byte size prefix, no INITCODE) and decodes it.
func liveRead(t *testing.T, c net.Conn) (protocol.Header, []byte) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	var sz [2]byte
	if _, err := io.ReadFull(c, sz[:]); err != nil {
		t.Fatalf("read size: %v", err)
	}
	size := int(binary.LittleEndian.Uint16(sz[:]))
	buf := make([]byte, size)
	copy(buf, sz[:])
	if _, err := io.ReadFull(c, buf[2:]); err != nil {
		t.Fatalf("read body: %v", err)
	}
	h, payload, _, err := protocol.Decode(buf)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return h, payload
}

func loginFrame(account, password string) []byte {
	var body protocol.MsgAccountLoginBody
	copy(body.AccountPassword[:], password)
	copy(body.AccountName[:], account)
	body.ClientVersion = protocol.AppVersion
	if v := os.Getenv("W2PP_E2E_VERSION"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			body.ClientVersion = int32(n)
		}
	}
	wire, err := protocol.Encode(protocol.Header{Type: protocol.MsgAccountLogin}, body.Encode(), 7)
	if err != nil {
		panic(err)
	}
	return wire
}

// TestE2ESmokeLogin drives a real login against the seeded account and expects
// the character-selection confirmation (CNFAccountLogin). An empty char list is
// fine — the point is that auth round-tripped through gRPC+mTLS to Postgres.
func TestE2ESmokeLogin(t *testing.T) {
	addr := env("W2PP_E2E_ADDR", "127.0.0.1:8281")
	account := env("W2PP_E2E_ACCOUNT", "smoke")
	password := env("W2PP_E2E_PASSWORD", "smoke123")

	c := liveDial(t, addr)
	defer c.Close()
	if _, err := c.Write(loginFrame(account, password)); err != nil {
		t.Fatalf("write login: %v", err)
	}

	h, payload := liveRead(t, c)
	if h.Type != protocol.MsgCNFAccountLogin {
		t.Fatalf("login response Type = %#x, want CNFAccountLogin (%#x); payload=%d bytes",
			h.Type, protocol.MsgCNFAccountLogin, len(payload))
	}
	// The char list lives in the SELCHAR slots (body offset 20+), not a leading
	// count byte; assert the body is full-sized rather than reading payload[0].
	if len(payload) < 1900 {
		t.Fatalf("CNFAccountLogin payload too short: %d bytes", len(payload))
	}
	t.Logf("LOGIN OK — CNFAccountLogin, conn=%d, body=%d bytes", h.ID, len(payload))
}

// TestE2ESmokeBadPassword confirms a wrong password is NOT accepted: the server
// must answer with something other than CNFAccountLogin (a notice panel).
func TestE2ESmokeBadPassword(t *testing.T) {
	addr := env("W2PP_E2E_ADDR", "127.0.0.1:8281")
	account := env("W2PP_E2E_ACCOUNT", "smoke")

	c := liveDial(t, addr)
	defer c.Close()
	if _, err := c.Write(loginFrame(account, "wrong-pass")); err != nil {
		t.Fatalf("write login: %v", err)
	}
	h, _ := liveRead(t, c)
	if h.Type == protocol.MsgCNFAccountLogin {
		t.Fatalf("bad password was ACCEPTED (got CNFAccountLogin) — auth not enforced")
	}
	t.Logf("bad password correctly rejected — response Type=%#x", h.Type)
}
