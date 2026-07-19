package world

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// S→C send diagnostics (docs/migration/investigacao-freeze-cliente.md): the
// 18-19/jul client-freeze incident showed the server healthy while the client
// silently stopped processing what it received — and nothing logged the S→C
// side. Every queued frame is counted per type and remembered in a small ring,
// and the whole picture is dumped when the connection closes, so every freeze
// leaves a post-mortem of what the client was sent last. All of it is loop-owned
// session state (enqueue and removeSession both run in the loop), so no locks.

// lastSentRing is how many trailing S→C frames each session remembers. 64 covers
// the final seconds even under a CreateMob burst while costing ~1KB per session.
const lastSentRing = 64

// sentRecord is one remembered S→C frame: type, HEADER.ID (the subject entity —
// which mob/player the message was about), payload length and when it was queued.
type sentRecord struct {
	t   protocol.Type
	id  uint16
	len uint16
	at  int64 // unix milliseconds
}

// noteSend records one queued S→C frame in the session's diagnostics.
func (s *Session) noteSend(h protocol.Header, payloadLen int) {
	s.sentFrames++
	s.sentBytes += uint64(payloadLen + protocol.HeaderSize)
	if s.sentByType == nil {
		s.sentByType = make(map[protocol.Type]uint32, 32)
	}
	s.sentByType[h.Type]++
	s.lastSent[s.lastSentIdx%lastSentRing] = sentRecord{
		t: h.Type, id: h.ID, len: uint16(payloadLen), at: time.Now().UnixMilli(),
	}
	s.lastSentIdx++
}

// SentOfType returns how many S→C frames of type t were queued to s in this
// session. Loop-only; used by handlers to log per-action deltas (e.g. how many
// CreateMob/RemoveMob a teleport produced).
func (w *World) SentOfType(s *Session, t protocol.Type) uint32 { return s.sentByType[t] }

// logSendStats emits the session's S→C post-mortem: totals, the per-type
// breakdown and the trailing frames with their age relative to the close. Called
// from removeSession, so every disconnect (including a user killing a frozen
// client) leaves the evidence in the logs.
func (w *World) logSendStats(s *Session) {
	if s.sentFrames == 0 {
		return
	}
	w.log.Info("session send stats",
		"conn", s.Conn,
		"frames", s.sentFrames,
		"bytes", s.sentBytes,
		"out_high_water", s.outHighWater,
		"by_type", formatByType(s.sentByType),
	)
	w.log.Info("session last sends", "conn", s.Conn, "tail", formatLastSends(&s.lastSent, s.lastSentIdx, time.Now().UnixMilli()))
}

// formatByType renders a per-type count map as "0x0366:120 0x0181:80 …",
// descending by count so the dominant traffic reads first.
func formatByType(m map[protocol.Type]uint32) string {
	type kv struct {
		t protocol.Type
		n uint32
	}
	rows := make([]kv, 0, len(m))
	for t, n := range m {
		rows = append(rows, kv{t, n})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].n != rows[j].n {
			return rows[i].n > rows[j].n
		}
		return rows[i].t < rows[j].t
	})
	var b strings.Builder
	for i, r := range rows {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(formatSendType(r.t))
		b.WriteByte(':')
		b.WriteString(strconv.FormatUint(uint64(r.n), 10))
	}
	return b.String()
}

// formatLastSends renders the ring oldest→newest as
// "0x0165(30000)/504/-830ms …" — type(HEADER.ID)/payload-len/age-before-close.
func formatLastSends(ring *[lastSentRing]sentRecord, next int, nowMs int64) string {
	n := next
	if n > lastSentRing {
		n = lastSentRing
	}
	var b strings.Builder
	for i := 0; i < n; i++ {
		r := ring[(next-n+i)%lastSentRing]
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(formatSendType(r.t))
		b.WriteByte('(')
		b.WriteString(strconv.FormatUint(uint64(r.id), 10))
		b.WriteString(")/")
		b.WriteString(strconv.FormatUint(uint64(r.len), 10))
		b.WriteString("/-")
		b.WriteString(strconv.FormatInt(nowMs-r.at, 10))
		b.WriteString("ms")
	}
	return b.String()
}

// formatSendType renders a message type as 0xNNNN (same shape as the handler
// package's recv-packet formatType; duplicated because world cannot import
// handler).
func formatSendType(t protocol.Type) string {
	const hexdigits = "0123456789abcdef"
	v := uint16(t)
	return "0x" + string([]byte{hexdigits[v>>12&0xf], hexdigits[v>>8&0xf], hexdigits[v>>4&0xf], hexdigits[v&0xf]})
}
