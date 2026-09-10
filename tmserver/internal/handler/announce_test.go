package handler

import (
	"io"
	"log/slog"
	"testing"
)

func announceDispatcher() *Dispatcher {
	return New(Config{
		Log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		ItemNames: map[int]string{4038: "Vela_do_Coveiro"},
	})
}

// A refine below +10 is routine and must NOT reach the whole server; at +10 and
// above it must. The threshold is the whole point of the feature — announcing
// every +1 would make the channel unreadable.
func TestAnnounceRefineThreshold(_ *testing.T) {
	d := announceDispatcher()
	// A nil world would panic if the guard let the call through, so "does not
	// panic" is the assertion for the levels that must stay quiet.
	for _, level := range []int{0, 1, 9} {
		d.announceRefine(nil, "Hero", 4038, level)
	}
}

// An empty name means the entity was already gone (a docked character, a mob):
// announcing "[EVENTO]  renasceu como Arch" helps nobody.
func TestAnnounceIgnoresEmptyName(_ *testing.T) {
	d := announceDispatcher()
	d.announceArch(nil, "")
	d.announceCelestial(nil, "")
	for _, ok := range []bool{true, false} {
		d.announceMais10(nil, "", 4038, 47, 41, ok)
		d.announceComposicao(nil, "", 4038, 47, 41, ok)
		d.announceAgatha(nil, "", 4038, 47, 41, ok)
	}
	d.announceRefine(nil, "", 4038, 12)
}

func TestItemNameFallsBackToIndex(t *testing.T) {
	d := announceDispatcher()
	if got := d.itemName(4038); got != "Vela_do_Coveiro" {
		t.Errorf("itemName(4038) = %q, want the catalog name", got)
	}
	// An item missing from the catalog still has to say something specific enough
	// to act on, so the index stands in for the name.
	if got := d.itemName(9999); got != "#9999" {
		t.Errorf("itemName(9999) = %q, want #9999", got)
	}
}
