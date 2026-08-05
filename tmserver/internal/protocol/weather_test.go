package protocol

import (
	"encoding/binary"
	"testing"
)

// TestEncodeUpdateWeather pins the MSG_UpdateWeather layout: one little-endian
// int32 at body offset 0, 16 bytes total with the header (Basedef.h:2167-2173).
func TestEncodeUpdateWeather(t *testing.T) {
	for _, weather := range []int32{0, 1, 2} {
		body := EncodeUpdateWeather(weather)
		if len(body) != MsgUpdateWeatherBodySize {
			t.Fatalf("weather %d: body len = %d, want %d", weather, len(body), MsgUpdateWeatherBodySize)
		}
		if got := int32(binary.LittleEndian.Uint32(body[0:4])); got != weather {
			t.Errorf("weather %d: CurrentWeather @0 = %d", weather, got)
		}
	}
	if MsgUpdateWeatherBodySize+HeaderSize != 16 {
		t.Errorf("total packet = %d, want 16", MsgUpdateWeatherBodySize+HeaderSize)
	}
	if MsgUpdateWeather != 0x018B {
		t.Errorf("MsgUpdateWeather = %#04x, want 0x018B (139|FLAG_GAME2CLIENT)", uint16(MsgUpdateWeather))
	}
}
