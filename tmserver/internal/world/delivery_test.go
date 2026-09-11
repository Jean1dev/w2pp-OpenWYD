package world

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// drainCapture records the cargo writes the drain paths make: the plain
// SaveCargo and the SaveCargoWithDeliveries that also acks mailbox rows. With
// failDrains set, every SaveCargoWithDeliveries fails, as a dbServer outage would.
type drainCapture struct {
	NopPersistence
	mu         sync.Mutex
	failDrains bool
	plain      []CargoSave
	acks       [][]int64
}

func (d *drainCapture) SaveCargo(_ context.Context, cs CargoSave) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.plain = append(d.plain, cs)
	return nil
}

func (d *drainCapture) SaveCargoWithDeliveries(_ context.Context, _ CargoSave, delivered, _ []int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.acks = append(d.acks, append([]int64(nil), delivered...))
	if d.failDrains {
		return errors.New("dbserver down")
	}
	return nil
}

// waitAcks waits for the async drain save to reach the persistence port.
func (d *drainCapture) waitAcks(t *testing.T, n int) [][]int64 {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		d.mu.Lock()
		if len(d.acks) >= n {
			out := append([][]int64(nil), d.acks...)
			d.mu.Unlock()
			return out
		}
		d.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("waited for %d drain saves", n)
	return nil
}

func countIndex(c *CargoState, index int16) int {
	n := 0
	for _, it := range c.Items {
		if it.Index == index {
			n++
		}
	}
	return n
}

// TestDeliveryDrainTwicePlacesOnce: the login and a deliver-now (or two
// deliver-nows from the site) each fetch the pending list before the other's ack
// commits, so both hand the same row to ApplyDeliveries. The second call must
// place nothing: the item is already in the cargo. Before, it was placed twice —
// a paid item duplicated by a player clicking "entregar" with the game open.
func TestDeliveryDrainTwicePlacesOnce(t *testing.T) {
	pc := &drainCapture{}
	w := New(Config{GridDim: 16}, slogDiscard(), pc, nil)
	s := &Session{Conn: 1, AccountID: 42}
	w.SetCargo(42, &CargoState{})
	lista := []Delivery{{ID: 21, Item: Item{Index: 4321}}}

	if d, h := w.ApplyDeliveries(s, lista); d != 1 || h != 0 {
		t.Fatalf("first drain = %d delivered %d held, want 1/0", d, h)
	}
	if d, h := w.ApplyDeliveries(s, lista); d != 0 || h != 0 {
		t.Fatalf("second drain of the same row = %d delivered %d held, want 0/0", d, h)
	}
	if n := countIndex(w.Cargo(42), 4321); n != 1 {
		t.Fatalf("cargo holds %d copies of the grant, want 1", n)
	}
	pc.waitAcks(t, 1)
}

// TestDeliveryAckRidesTheNextCargoSave: when the drain's save fails, the item is
// in the in-memory cargo but its row is still 'pending'. The next cargo write of
// the account — here the logout — must carry the ack in the same transaction:
// a plain SaveCargo would store the item and leave the row to be delivered again
// at the next login.
func TestDeliveryAckRidesTheNextCargoSave(t *testing.T) {
	pc := &drainCapture{failDrains: true}
	w := New(Config{GridDim: 16}, slogDiscard(), pc, nil)
	s := &Session{Conn: 1, AccountID: 42}
	w.SetCargo(42, &CargoState{})

	w.ApplyDeliveries(s, []Delivery{{ID: 31, Item: Item{Index: 4321}}})
	pc.waitAcks(t, 1) // the drain save that fails

	w.ReleaseCargo(42)
	acks := pc.waitAcks(t, 2)
	if got := acks[1]; len(got) != 1 || got[0] != 31 {
		t.Fatalf("logout cargo save acked %v, want [31]", got)
	}
	pc.mu.Lock()
	defer pc.mu.Unlock()
	if len(pc.plain) != 0 {
		t.Fatalf("logout used a plain SaveCargo (%d) with an unacked delivery in the cargo", len(pc.plain))
	}
}

// TestDeliveryReleaseForgetsTheMailbox: a released cargo takes its drain
// bookkeeping with it, so the next login (a fresh cargo) delivers a row that is
// still pending instead of skipping it as already placed.
func TestDeliveryReleaseForgetsTheMailbox(t *testing.T) {
	pc := &drainCapture{failDrains: true}
	w := New(Config{GridDim: 16}, slogDiscard(), pc, nil)
	s := &Session{Conn: 1, AccountID: 42}
	w.SetCargo(42, &CargoState{})
	w.ApplyDeliveries(s, []Delivery{{ID: 41, Item: Item{Index: 4321}}})
	w.ReleaseCargo(42)

	w.SetCargo(42, &CargoState{})
	if d, _ := w.ApplyDeliveries(s, []Delivery{{ID: 41, Item: Item{Index: 4321}}}); d != 1 {
		t.Fatalf("re-drain after release delivered %d, want 1 (the row never left pending)", d)
	}
}
