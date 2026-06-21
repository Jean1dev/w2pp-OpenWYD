package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// ErrBadInitCode is returned when a connection's first 4 bytes are not InitCode.
// The legacy server drops such connections (protocol-spec.md §1.2).
var ErrBadInitCode = errors.New("protocol: bad INITCODE")

// ErrBadSize is returned when a frame's header Size is out of the accepted range
// [HeaderSize, MaxMessageSize] (protocol-spec.md §1.3, ErrorCode=2).
var ErrBadSize = errors.New("protocol: frame size out of range")

// Framer de-frames the CPSock byte stream from a connection (CPSock::ReadMessage,
// protocol-spec.md §1.3). It consumes and validates the 4-byte INITCODE once,
// then yields Size-delimited frames. Size is validated BEFORE any per-frame
// allocation (guidelines §18.3). A Framer is not safe for concurrent use; use
// one per connection (one I/O goroutine per connection, migration-plan.md §3.5).
type Framer struct {
	r        io.Reader
	buf      []byte
	scratch  []byte
	initDone bool
}

// NewFramer returns a Framer reading from r.
func NewFramer(r io.Reader) *Framer {
	return &Framer{r: r, scratch: make([]byte, 4096)}
}

// fill reads from the underlying reader until at least n bytes are buffered.
// It returns io.EOF only when the stream ends exactly on a frame boundary with
// nothing buffered, and io.ErrUnexpectedEOF when it ends mid-frame.
func (f *Framer) fill(n int) error {
	for len(f.buf) < n {
		m, err := f.r.Read(f.scratch)
		if m > 0 {
			f.buf = append(f.buf, f.scratch[:m]...)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(f.buf) == 0 {
					return io.EOF
				}
				if len(f.buf) < n {
					return io.ErrUnexpectedEOF
				}
				return nil
			}
			return fmt.Errorf("protocol: read: %w", err)
		}
	}
	return nil
}

// ReadFrame returns the next complete obfuscated wire frame (exactly Size bytes).
// On the first call it consumes the INITCODE. It returns io.EOF when the stream
// ends cleanly between frames. The returned slice is owned by the caller (it is
// not retained by the Framer); pass it to Decode.
func (f *Framer) ReadFrame() ([]byte, error) {
	if !f.initDone {
		if err := f.fill(4); err != nil {
			return nil, err
		}
		if binary.LittleEndian.Uint32(f.buf[0:4]) != InitCode {
			return nil, ErrBadInitCode
		}
		f.buf = f.buf[4:]
		f.initDone = true
	}

	if err := f.fill(HeaderSize); err != nil {
		return nil, err
	}
	size := int(binary.LittleEndian.Uint16(f.buf[0:2]))
	if size < HeaderSize || size > MaxMessageSize {
		return nil, fmt.Errorf("%w: %d", ErrBadSize, size)
	}

	if err := f.fill(size); err != nil {
		return nil, err
	}
	frame := make([]byte, size)
	copy(frame, f.buf[:size])
	f.buf = f.buf[size:]
	return frame, nil
}
