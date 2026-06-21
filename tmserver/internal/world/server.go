package world

import (
	"context"
	"errors"
	"net"
)

// Serve runs the world: it starts the accept loop on ln and then runs the game
// loop (blocking) until ctx is cancelled. On cancellation it stops accepting,
// drains/saves sessions and returns ctx.Err().
func (w *World) Serve(ctx context.Context, ln net.Listener) error {
	go w.acceptLoop(ctx, ln)
	return w.Run(ctx)
}

// acceptLoop accepts connections and hands each to the loop as a connectEvent.
// It runs in its own goroutine; closing the listener on ctx cancellation makes
// Accept return so the loop can exit.
func (w *World) acceptLoop(ctx context.Context, ln net.Listener) {
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			w.log.Warn("accept failed", "err", err)
			return
		}
		if !w.emit(connectEvent{conn: c, ip: c.RemoteAddr().String()}) {
			_ = c.Close()
			return
		}
	}
}
