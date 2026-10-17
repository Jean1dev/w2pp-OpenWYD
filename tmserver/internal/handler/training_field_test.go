package handler

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// fieldAttrs is an AttributeMap with only the 4×4 block of tile (2100,2032)
// marked as training grounds.
func fieldAttrs() *content.Grid {
	g := &content.Grid{Dim: content.AttributeMapDim, Data: make([]byte, content.AttributeMapDim*content.AttributeMapDim)}
	g.Data[(2032/4)*g.Dim+2100/4] = 0x80
	return g
}

// TestTrainingFieldAt reads the AttributeMap the way GetAttribute does: one
// byte per 4×4 tiles, bit 0x80 marking the training grounds (issue #383).
func TestTrainingFieldAt(t *testing.T) {
	attrs := &content.Grid{Dim: 8, Data: make([]byte, 64)}
	attrs.Data[2*8+3] = 0x80 | 0x02 // tiles x 12..15, y 8..11
	attrs.Data[0] = 0x40
	tests := []struct {
		name  string
		attrs *content.Grid
		x, y  int16
		want  bool
	}{
		{name: "field cell", attrs: attrs, x: 12, y: 8, want: true},
		{name: "same block far corner", attrs: attrs, x: 15, y: 11, want: true},
		{name: "neighbour block", attrs: attrs, x: 16, y: 8, want: false},
		{name: "other attribute bits", attrs: attrs, x: 0, y: 0, want: false},
		{name: "negative", attrs: attrs, x: -1, y: 8, want: false},
		{name: "out of range", attrs: attrs, x: 400, y: 8, want: false},
		{name: "no map", attrs: nil, x: 12, y: 8, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TrainingFieldAt(tt.attrs, tt.x, tt.y); got != tt.want {
				t.Errorf("TrainingFieldAt(%d,%d) = %v, want %v", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestNewbieZoneGuard(t *testing.T) {
	d, w, s, e := startOnePlayer(t)
	for _, tc := range []struct {
		name        string
		attrs       *content.Grid
		level       int32
		classMaster uint8
		x, y        int16
		wantBlocked bool
	}{
		{"displayed 35 enters", fieldAttrs(), 34, classMasterMortal, 2100, 2032, false},
		{"displayed 36 is kicked", fieldAttrs(), 35, classMasterMortal, 2100, 2032, true},
		{"stored 999 is kicked", fieldAttrs(), 999, classMasterMortal, 2100, 2032, true},
		{"GM enters", fieldAttrs(), 1000, classMasterMortal, 2100, 2032, false},
		{"non-mortal is kicked at any level", fieldAttrs(), 10, classMasterArch, 2100, 2032, true},
		{"outside the field", fieldAttrs(), 35, classMasterMortal, 2100, 2080, false},
		{"no attribute map", nil, 35, classMasterMortal, 2100, 2032, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runInLoop(t, w, func() {
				d.attrs = tc.attrs
				w.SetEntityPos(s.Conn, 2100, 2080)
				e.Level, e.ClassMaster = tc.level, tc.classMaster
				panels := w.SentOfType(s, protocol.MsgMessagePanel)

				if got := d.newbieZoneGuard(w, s, e, tc.x, tc.y); got != tc.wantBlocked {
					t.Fatalf("blocked = %v, want %v", got, tc.wantBlocked)
				}
				if !tc.wantBlocked {
					if e.X != 2100 || e.Y != 2080 {
						t.Errorf("allowed move teleported to %d,%d", e.X, e.Y)
					}
					return
				}
				if w.SentOfType(s, protocol.MsgMessagePanel) != panels+1 {
					t.Error("no _NN_Newbie_zone panel")
				}
				if e.X < newbieZoneKickX || e.X > newbieZoneKickX+2 || e.Y < newbieZoneKickY || e.Y > newbieZoneKickY+2 {
					t.Errorf("kicked to %d,%d, want Armia 2091-2093,2101-2103", e.X, e.Y)
				}
			})
		})
	}
}
