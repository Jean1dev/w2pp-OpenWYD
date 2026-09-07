package world

import (
	"context"
	"time"
)

// The chat log's buffer (0034_chat_log).
//
// Chat is the highest-volume thing a game server produces: every sentence
// anybody types, all day. Sending one write per line would put the database in
// the path of typing — and the loop owns all world state alone and never blocks,
// so it cannot wait for any of them either.
//
// So lines pile up in a loop-owned slice and go out in batches. Nothing here
// blocks; the flush is a detached call like the ground log's.

const (
	// chatLote is how many lines force a flush. Chosen against the store's own
	// cap (500) with room to spare, so a batch never bounces for being too big.
	chatLote = 200

	// chatIntervalo is how long a partly-filled buffer waits. On a quiet server
	// this is what gets lines written at all; on a busy one chatLote fires first.
	chatIntervalo = 5 * time.Second

	// chatTeto is where the buffer stops growing when the database is unreachable
	// and flushes keep failing. Past it the OLDEST lines are dropped, because a
	// buffer that grows without limit turns a database outage into a dead server,
	// and because the newest lines are the ones somebody is about to ask about.
	chatTeto = 5000

	// chatTempo caps one flush.
	chatTempo = 10 * time.Second
)

// RegistraChat queues one line for the chat log. Loop-only, never blocks.
//
// Empty text is dropped: the client sends some of its own housekeeping through
// the chat messages, and a log full of blank lines is a log nobody reads.
func (w *World) RegistraChat(l ChatLinha) {
	if l.Texto == "" || l.Character == "" {
		return
	}
	if l.At.IsZero() {
		l.At = time.Now()
	}
	if len(w.chatBuf) == 0 {
		// A janela começa quando o lote começa a encher, não quando o último
		// saiu. Medindo do último envio, um servidor que passou a madrugada
		// calado mandava a primeira frase da manhã sozinha, sem esperar
		// ninguém — e num mundo recém-criado, onde nunca houve envio, TODA
		// primeira frase saía sozinha.
		w.chatUltimo = l.At
	}
	w.chatBuf = append(w.chatBuf, l)

	if len(w.chatBuf) > chatTeto {
		// Drop from the front. Losing the oldest is the lesser harm: those are
		// the lines furthest from whatever incident prompts somebody to look.
		excesso := len(w.chatBuf) - chatTeto
		w.chatBuf = w.chatBuf[excesso:]
		w.chatDescartadas += excesso
	}
	if len(w.chatBuf) >= chatLote {
		w.EsvaziaChat()
	}
}

// EsvaziaChat sends whatever is buffered. Loop-only.
//
// It hands the slice off and starts a fresh one rather than reusing the array:
// the batch travels to another goroutine, and reusing the backing store would
// let the next line overwrite something already in flight.
func (w *World) EsvaziaChat() {
	if len(w.chatBuf) == 0 || w.chatEnviando {
		// One batch in flight at a time. Two would race to write the same lines
		// if the first is slow, and would also multiply the load on a database
		// that is already the reason the first one is slow.
		return
	}
	lote := w.chatBuf
	w.chatBuf = nil
	w.chatEnviando = true

	descartadas := w.chatDescartadas
	w.chatDescartadas = 0

	p := w.persist
	w.GoDetached(func() func(*World) {
		ctx, cancel := context.WithTimeout(context.Background(), chatTempo)
		defer cancel()
		err := p.RecordChat(ctx, lote)
		return func(w *World) {
			w.chatEnviando = false
			if err != nil {
				w.log.Warn("chat log: batch lost", "linhas", len(lote), "err", err)
			}
			if descartadas > 0 {
				// Said out loud, and not once per dropped line: the buffer only
				// overflows when writes have been failing for a while, and one
				// line per drop would bury the reason in its own symptom.
				w.log.Warn("chat log: buffer full, oldest lines dropped",
					"descartadas", descartadas, "teto", chatTeto)
			}
		}
	})
}

// chatTick flushes a buffer that has been waiting long enough. Called from the
// world tick.
//
// The clock runs from the moment the batch started filling (RegistraChat sets
// it), so a line waits at most chatIntervalo no matter how long the server was
// quiet before it.
func (w *World) chatTick(agora time.Time) {
	if len(w.chatBuf) == 0 {
		return
	}
	if agora.Sub(w.chatUltimo) < chatIntervalo {
		return
	}
	w.chatUltimo = agora
	w.EsvaziaChat()
}
