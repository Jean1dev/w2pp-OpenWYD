package world

import (
	"net"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// outFrame is a logical S→C message queued to a session's writer goroutine,
// which encodes it (CPSock) just before writing.
type outFrame struct {
	header  protocol.Header
	payload []byte
}

// Session is a player's connection/session state (CUser subset,
// domain-model.md §2.1). It is owned by the loop goroutine; the conn/out/closeCh
// plumbing is shared with this session's reader and writer goroutines only.
type Session struct {
	Conn        int // index into pUser/pMob; also HEADER.ID on the wire
	AccountName string
	AccountID   int64
	Slot        int
	Mode        Mode
	IP          string

	conn    net.Conn
	out     chan outFrame
	closeCh chan struct{}
	closed  bool
}

// close stops the session's I/O goroutines: closing closeCh ends the writer and
// closing the socket unblocks the reader. Idempotent; loop-only.
func (s *Session) close() {
	if s.closed {
		return
	}
	s.closed = true
	close(s.closeCh)
	_ = s.conn.Close()
}

// Entity is a world entity (CMob subset, domain-model.md §2.2). Players
// (ID < MaxUser) and mobs (ID >= MaxUser) share this type and the same index
// space. Phase 3 carries only the minimum; full STRUCT_MOB state arrives with
// the handlers (Phase 4).
type Entity struct {
	ID    int
	Mode  EntityMode
	Name  string
	X     int16
	Y     int16
	HP    int32
	MaxHP int32
}

// IsPlayer reports whether an entity index belongs to a player (domain-model.md
// §1: id < MaxUser ⇒ player).
func IsPlayer(id int) bool { return id >= 0 && id < MaxUser }
