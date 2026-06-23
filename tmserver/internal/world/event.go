package world

import (
	"errors"
	"math/rand/v2"
	"net"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// Fase 7 disconnect reasons (migration-plan.md §5).
var (
	errRateLimited      = errors.New("world: connection exceeded message rate limit")
	errChecksumMismatch = errors.New("world: rejected frame on checksum mismatch")
)

// event is something the loop applies to world state. Every event is applied in
// the loop goroutine, so apply may mutate the World freely.
type event interface{ apply(*World) }

// emit sends an event to the loop, or returns false if the world is shutting
// down (so connection goroutines stop instead of blocking forever).
func (w *World) emit(ev event) bool {
	select {
	case w.events <- ev:
		return true
	case <-w.done:
		return false
	}
}

// --- connect ---

type connectEvent struct {
	conn net.Conn
	ip   string
}

func (e connectEvent) apply(w *World) {
	conn := w.allocConn()
	if conn < 0 {
		w.log.Warn("server full; rejecting connection", "ip", e.ip)
		_ = e.conn.Close()
		return
	}
	s := &Session{
		Conn:    conn,
		IP:      e.ip,
		Mode:    UserAccept,
		conn:    e.conn,
		out:     make(chan outFrame, w.cfg.OutBuffer),
		closeCh: make(chan struct{}),
	}
	w.sessions[conn] = s
	w.entities[conn] = &Entity{ID: conn, Mode: MobUserDock}
	go w.readLoop(s)
	go w.writeLoop(s)
	w.log.Info("connection accepted", "conn", conn, "ip", e.ip)
}

// allocConn finds a free player slot. Conn 0 is reserved (the original requires
// conn > 0 for login — handlers/_MSG_AccountLogin.md; trade/attack ids also use
// (0, MAX_USER)). The top ADMIN_RESERV slots are reserved too (value UNVERIFIED).
func (w *World) allocConn() int {
	for i := 1; i < MaxUser; i++ {
		if w.sessions[i] == nil {
			return i
		}
	}
	return -1
}

// --- frame ---

type frameEvent struct {
	s       *Session
	header  protocol.Header
	payload []byte
}

func (e frameEvent) apply(w *World) {
	// Ignore frames from a session whose slot was already freed/reused.
	if w.sessions[e.s.Conn] != e.s {
		return
	}
	// Global dispatcher guards (protocol-spec.md §2): Ping is a no-op; the
	// internal SkipCheckTick must never come from a client.
	if e.header.Type == protocol.MsgPing {
		return
	}
	if e.header.ClientTick == protocol.SkipCheckTick {
		return
	}
	w.handler(w, e.s, e.header, e.payload)
}

// --- async callback (DB results re-entering the loop) ---

// callbackEvent carries the result of off-loop work (World.Go) back into the
// loop. It is dropped if the session's slot was freed or reused meanwhile.
type callbackEvent struct {
	conn int
	sess *Session
	cb   func(*World, *Session)
}

func (e callbackEvent) apply(w *World) {
	if w.sessions[e.conn] != e.sess {
		return
	}
	e.cb(w, e.sess)
}

// --- disconnect ---

type disconnectEvent struct {
	s   *Session
	err error
}

func (e disconnectEvent) apply(w *World) {
	// Surface WHY a connection dropped (bad INITCODE, EOF, oversize, rate, checksum)
	// instead of swallowing it — essential to diagnose client-edge handshakes.
	if e.err != nil && w.sessions[e.s.Conn] == e.s {
		w.log.Info("connection drop reason", "conn", e.s.Conn, "ip", e.s.IP, "err", e.err.Error())
	}
	w.removeSession(e.s)
}

// removeSession tears down a session and frees its slot. It only acts if the
// slot still holds this exact session (guards against a reused slot). Loop-only.
func (w *World) removeSession(s *Session) {
	if s == nil || w.sessions[s.Conn] != s {
		return
	}
	// Tell in-view players this entity left (logout), so their clients despawn it.
	if e := w.entities[s.Conn]; e != nil && e.Mode == MobUser {
		body := protocol.EncodeRemoveMobBody(2) // 2 = logout
		w.ForEachInView(s.Conn, func(vs *Session, _ *Entity) {
			w.enqueue(vs, protocol.Header{Type: protocol.MsgRemoveMob, ID: uint16(s.Conn)}, body)
		})
	}
	s.close()
	w.sessions[s.Conn] = nil
	w.entities[s.Conn] = nil
	w.log.Info("connection closed", "conn", s.Conn)
}

// dropSession is removeSession by pointer, used when the loop must shed a slow
// client (full out queue).
func (w *World) dropSession(s *Session) { w.removeSession(s) }

// --- per-connection I/O goroutines (NOT the loop) ---

// readLoop de-frames and decodes CPSock messages from one connection and feeds
// them to the loop as frameEvents. It runs in its own goroutine and never
// touches world state directly.
func (w *World) readLoop(s *Session) {
	fr := protocol.NewFramer(s.conn)
	bucket := newTokenBucket(w.cfg.MaxMsgPerSec, w.cfg.MsgBurst, time.Now())
	for {
		frame, err := fr.ReadFrame()
		if err != nil {
			w.emit(disconnectEvent{s: s, err: err})
			return
		}
		// Fase 7: flood protection — a connection exceeding its rate is dropped.
		if !bucket.allow(time.Now()) {
			w.emit(disconnectEvent{s: s, err: errRateLimited})
			return
		}
		h, payload, mismatch, err := protocol.Decode(frame)
		if err != nil {
			w.emit(disconnectEvent{s: s, err: err})
			return
		}
		// Fase 7: optional rejecting checksum (off by default; legacy is non-rejecting).
		if mismatch && w.cfg.RejectChecksum {
			w.emit(disconnectEvent{s: s, err: errChecksumMismatch})
			return
		}
		if !w.emit(frameEvent{s: s, header: h, payload: payload}) {
			return // world shutting down
		}
	}
}

// writeLoop encodes queued S→C messages and writes them to the socket. The
// writer owns the socket close: on close it flushes whatever is still queued so
// a final notice sent just before close is delivered. The per-packet keyword
// index is random (CPSock.cpp:535) and irrelevant to parity (parity-tests.md
// §4.0), so the global RNG is fine here.
func (w *World) writeLoop(s *Session) {
	defer func() { _ = s.conn.Close() }()
	for {
		select {
		case of := <-s.out:
			if !w.writeFrame(s, of) {
				return
			}
		case <-s.closeCh:
			// Graceful close: drain and write whatever is still queued, then exit
			// (the deferred Close unblocks the reader).
			for {
				select {
				case of := <-s.out:
					if !w.writeFrame(s, of) {
						return
					}
				default:
					return
				}
			}
		}
	}
}

// writeFrame encodes and writes one S→C frame. It returns false (stopping the
// writer) only on a socket write error; an encode error is logged and skipped.
func (w *World) writeFrame(s *Session, of outFrame) bool {
	wire, err := protocol.Encode(of.header, of.payload, uint8(rand.IntN(256)))
	if err != nil {
		w.log.Warn("encode failed", "conn", s.Conn, "err", err)
		return true
	}
	if _, err := s.conn.Write(wire); err != nil {
		w.emit(disconnectEvent{s: s, err: err})
		return false
	}
	return true
}
