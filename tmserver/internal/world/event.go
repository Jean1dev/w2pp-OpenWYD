package world

import (
	"math/rand/v2"
	"net"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
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

func (w *World) allocConn() int {
	for i := 0; i < MaxUser; i++ {
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

// --- disconnect ---

type disconnectEvent struct {
	s   *Session
	err error
}

func (e disconnectEvent) apply(w *World) {
	w.removeSession(e.s)
}

// removeSession tears down a session and frees its slot. It only acts if the
// slot still holds this exact session (guards against a reused slot). Loop-only.
func (w *World) removeSession(s *Session) {
	if s == nil || w.sessions[s.Conn] != s {
		return
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
	for {
		frame, err := fr.ReadFrame()
		if err != nil {
			w.emit(disconnectEvent{s: s, err: err})
			return
		}
		h, payload, _, err := protocol.Decode(frame)
		if err != nil {
			w.emit(disconnectEvent{s: s, err: err})
			return
		}
		if !w.emit(frameEvent{s: s, header: h, payload: payload}) {
			return // world shutting down
		}
	}
}

// writeLoop encodes queued S→C messages and writes them to the socket. The
// per-packet keyword index is random (CPSock.cpp:535) and irrelevant to parity
// (parity-tests.md §4.0), so the global RNG is fine here.
func (w *World) writeLoop(s *Session) {
	for {
		select {
		case of := <-s.out:
			wire, err := protocol.Encode(of.header, of.payload, uint8(rand.IntN(256)))
			if err != nil {
				w.log.Warn("encode failed", "conn", s.Conn, "err", err)
				continue
			}
			if _, err := s.conn.Write(wire); err != nil {
				w.emit(disconnectEvent{s: s, err: err})
				return
			}
		case <-s.closeCh:
			return
		}
	}
}
