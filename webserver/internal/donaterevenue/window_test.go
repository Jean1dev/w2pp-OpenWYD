package donaterevenue

import (
	"testing"
	"time"
)

func TestMaskCPF(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"eleven digits", "12345678909", "***.456.789-**"},
		{"all zeroes", "00000000000", "***.000.000-**"},
		{"ten digits", "1234567890", ""},
		{"twelve digits", "123456789012", ""},
		{"empty", "", ""},
		{"already formatted", "123.456.789-09", ""},
		{"letters", "1234567890a", ""},
		{"spaces", "123456789 9", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := maskCPF(tc.in); got != tc.want {
				t.Errorf("maskCPF(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeWindow(t *testing.T) {
	const day = 24 * time.Hour
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC).Unix()

	t.Run("explicit range is preserved", func(t *testing.T) {
		from, to := base, base+7*int64(day/time.Second)
		w, echo, ok := normalizeWindow(from, to)
		if !ok {
			t.Fatal("normalizeWindow returned not-ok for a valid range")
		}
		if w.From.Unix() != from || w.To.Unix() != to {
			t.Errorf("window = [%d,%d), want [%d,%d)", w.From.Unix(), w.To.Unix(), from, to)
		}
		if echo.FromUnix != from || echo.ToUnix != to {
			t.Errorf("echo = %+v, want the same bounds", echo)
		}
	})

	t.Run("zero to means now", func(t *testing.T) {
		before := time.Now().UTC()
		w, _, ok := normalizeWindow(base, 0)
		if !ok {
			t.Fatal("not ok")
		}
		if w.To.Before(before.Add(-time.Minute)) {
			t.Errorf("To = %v, want ~now", w.To)
		}
	})

	t.Run("both zero means last 30 days", func(t *testing.T) {
		w, _, ok := normalizeWindow(0, 0)
		if !ok {
			t.Fatal("not ok")
		}
		span := w.To.Sub(w.From)
		if span < 29*day || span > 31*day {
			t.Errorf("span = %v, want ~30 days", span)
		}
	})

	t.Run("rejects", func(t *testing.T) {
		cases := []struct {
			name     string
			from, to int64
		}{
			{"from equals to", base, base},
			{"from after to", base + 10, base},
			{"negative from", -1, base},
			{"negative to", base, -1},
			{"wider than max", base, base + (maxWindowDays+1)*int64(day/time.Second)},
		}
		for _, tc := range cases {
			if _, _, ok := normalizeWindow(tc.from, tc.to); ok {
				t.Errorf("%s: normalizeWindow(%d,%d) accepted, want rejected", tc.name, tc.from, tc.to)
			}
		}
	})

	t.Run("accepts exactly max span", func(t *testing.T) {
		to := base + maxWindowDays*int64(day/time.Second)
		if _, _, ok := normalizeWindow(base, to); !ok {
			t.Error("exactly maxWindowDays was rejected, want accepted")
		}
	})
}

func TestNormalizePage(t *testing.T) {
	cases := []struct {
		inLimit, inOffset     int
		wantLimit, wantOffset int
	}{
		{0, 0, defaultLimit, 0},
		{-1, 0, defaultLimit, 0},
		{maxLimit + 1, 0, maxLimit, 0},
		{10_000, 0, maxLimit, 0},
		{10, -3, 10, 0},
		{maxLimit, 20, maxLimit, 20},
	}
	for _, tc := range cases {
		gotL, gotO := normalizePage(tc.inLimit, tc.inOffset)
		if gotL != tc.wantLimit || gotO != tc.wantOffset {
			t.Errorf("normalizePage(%d,%d) = (%d,%d), want (%d,%d)",
				tc.inLimit, tc.inOffset, gotL, gotO, tc.wantLimit, tc.wantOffset)
		}
	}
}

func TestNormalizeSearchLimit(t *testing.T) {
	cases := map[int]int{
		0:                  defaultSearchLimit,
		-1:                 defaultSearchLimit,
		5:                  5,
		maxSearchLimit:     maxSearchLimit,
		maxSearchLimit + 1: maxSearchLimit,
		1000:               maxSearchLimit,
	}
	for in, want := range cases {
		if got := normalizeSearchLimit(in); got != want {
			t.Errorf("normalizeSearchLimit(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestNormalizePrefix(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"AB", "ab", true},
		{"  Zarco  ", "zarco", true},
		{"a", "", false},
		{"", "", false},
		{"   ", "", false},
		{" x ", "", false},
	}
	for _, tc := range cases {
		got, ok := normalizePrefix(tc.in)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("normalizePrefix(%q) = (%q,%v), want (%q,%v)", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}
