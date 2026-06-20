package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

// dripReader returns at most one byte per Read, to exercise partial framing.
type dripReader struct{ data []byte }

func (d *dripReader) Read(p []byte) (int, error) {
	if len(d.data) == 0 {
		return 0, io.EOF
	}
	p[0] = d.data[0]
	d.data = d.data[1:]
	return 1, nil
}

func withInitCode(frames ...[]byte) []byte {
	out := make([]byte, 4)
	binary.LittleEndian.PutUint32(out, InitCode)
	for _, f := range frames {
		out = append(out, f...)
	}
	return out
}

func mustEncode(t *testing.T, ty Type, payload []byte, k uint8) []byte {
	t.Helper()
	w, err := Encode(Header{Type: ty, ID: 1}, payload, k)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return w
}

func TestFramerInitCodeGate(t *testing.T) {
	bad := bytes.Repeat([]byte{0xAA}, 16)
	f := NewFramer(bytes.NewReader(bad))
	if _, err := f.ReadFrame(); !errors.Is(err, ErrBadInitCode) {
		t.Errorf("ReadFrame without INITCODE = %v, want ErrBadInitCode", err)
	}
}

func TestFramerSingleFrame(t *testing.T) {
	frame := mustEncode(t, MsgAction, []byte{1, 2, 3, 4}, 7)
	f := NewFramer(bytes.NewReader(withInitCode(frame)))
	got, err := f.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if !bytes.Equal(got, frame) {
		t.Errorf("frame = % x, want % x", got, frame)
	}
	if _, err := f.ReadFrame(); !errors.Is(err, io.EOF) {
		t.Errorf("second ReadFrame = %v, want io.EOF", err)
	}
}

func TestFramerPartialAndGlued(t *testing.T) {
	f1 := mustEncode(t, MsgAction, []byte{1, 2, 3}, 7)
	f2 := mustEncode(t, MsgGetItem, bytes.Repeat([]byte{9}, 30), 200)
	// Two frames glued together, delivered one byte at a time.
	stream := withInitCode(f1, f2)
	f := NewFramer(&dripReader{data: stream})

	got1, err := f.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame 1: %v", err)
	}
	if !bytes.Equal(got1, f1) {
		t.Errorf("frame 1 mismatch")
	}
	got2, err := f.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame 2: %v", err)
	}
	if !bytes.Equal(got2, f2) {
		t.Errorf("frame 2 mismatch")
	}
	if _, err := f.ReadFrame(); !errors.Is(err, io.EOF) {
		t.Errorf("trailing ReadFrame = %v, want io.EOF", err)
	}
}

func TestFramerSizeBounds(t *testing.T) {
	hdr := func(size uint16) []byte {
		b := make([]byte, HeaderSize)
		binary.LittleEndian.PutUint16(b, size)
		return b
	}
	tests := map[string]uint16{
		"oversize":  MaxMessageSize + 1,
		"undersize": HeaderSize - 1,
	}
	for name, size := range tests {
		t.Run(name, func(t *testing.T) {
			f := NewFramer(bytes.NewReader(withInitCode(hdr(size))))
			if _, err := f.ReadFrame(); !errors.Is(err, ErrBadSize) {
				t.Errorf("ReadFrame = %v, want ErrBadSize", err)
			}
		})
	}
}

func TestFramerTruncatedFrame(t *testing.T) {
	frame := mustEncode(t, MsgAction, bytes.Repeat([]byte{5}, 20), 1)
	truncated := withInitCode(frame)[:len(frame)] // drop the last few bytes
	f := NewFramer(bytes.NewReader(truncated))
	if _, err := f.ReadFrame(); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("ReadFrame on truncated stream = %v, want io.ErrUnexpectedEOF", err)
	}
}

// TestFramerEndToEnd is the in-process sanity check from the plan: build a real
// MSG_AccountLogin frame, push the whole stream (INITCODE + frame) through the
// Framer one byte at a time, then Decode — exercising
// initcode→framing→decode→transform→checksum end to end.
func TestFramerEndToEnd(t *testing.T) {
	body := MsgAccountLoginBody{ClientVersion: AppVersion}
	copy(body.AccountName[:], "tester")
	copy(body.AccountPassword[:], "secret")
	wire, err := Encode(Header{Type: MsgAccountLogin, ID: 3, ClientTick: 555}, body.Encode(), 123)
	if err != nil {
		t.Fatal(err)
	}

	f := NewFramer(&dripReader{data: withInitCode(wire)})
	frame, err := f.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	h, payload, mismatch, err := Decode(frame)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if mismatch {
		t.Errorf("checksum mismatch on round-trip")
	}
	if h.Type != MsgAccountLogin || h.ID != 3 || h.ClientTick != 555 {
		t.Errorf("header = %+v", h)
	}
	var got MsgAccountLoginBody
	if err := got.Decode(payload); err != nil {
		t.Fatalf("body Decode: %v", err)
	}
	if got.ClientVersion != AppVersion {
		t.Errorf("ClientVersion = %d, want %d", got.ClientVersion, AppVersion)
	}
	if cTrimNUL(got.AccountName[:]) != "tester" {
		t.Errorf("AccountName = %q", cTrimNUL(got.AccountName[:]))
	}
}
