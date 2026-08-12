package exptool

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// template builds a raw 816-byte STRUCT_MOB filled with a nonzero pattern, so
// the round-trip check catches any byte the stamp isn't allowed to touch.
func template(name string, lvl uint32, exp uint64, merchant byte) []byte {
	b := make([]byte, content.BaseMobSize)
	for i := range b {
		b[i] = byte(i % 251)
	}
	for i := 0; i < 16; i++ {
		b[i] = 0
	}
	copy(b, name)
	binary.LittleEndian.PutUint64(b[32:], exp)
	const cs = 92
	binary.LittleEndian.PutUint32(b[cs+0:], lvl)
	b[cs+12] = merchant
	return b
}

func write(t *testing.T, dir, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, dir, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestStampRoundTrip(t *testing.T) {
	dir := t.TempDir()
	monster := template("Cobaia", 10, 68571, 0) // garbage authored exp
	merchant := template("Vendedor", 5, 25000, 100)
	levelZero := template("Base", 0, 123, 0)
	write(t, dir, "Cobaia", monster)
	write(t, dir, "Vendedor", merchant)
	write(t, dir, "Base", levelZero)
	write(t, dir, "Nota.txt", []byte("not a template"))

	res, err := Stamp(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Stamped) != 1 || res.Skipped != 2 || res.SkippedNonTemplate != 1 {
		t.Fatalf("stamped %d / skipped %d / non-template %d, want 1 / 2 / 1",
			len(res.Stamped), res.Skipped, res.SkippedNonTemplate)
	}
	e := res.Stamped[0]
	want := level.MobExpForLevel(10)
	if e.Name != "Cobaia" || e.OldExp != 68571 || e.NewExp != want {
		t.Fatalf("entry = %+v, want Cobaia 68571 -> %d", e, want)
	}

	// The monster file changed in exactly the 8 Exp bytes.
	got := read(t, dir, "Cobaia")
	if b := protocol.ParseMobBasics(got); b.Exp != want {
		t.Errorf("stamped Exp = %d, want %d", b.Exp, want)
	}
	patched := append([]byte(nil), monster...)
	binary.LittleEndian.PutUint64(patched[32:], uint64(want))
	if !bytes.Equal(got, patched) {
		t.Error("stamp touched bytes outside the Exp field")
	}

	// Non-monster files stay byte-identical.
	if !bytes.Equal(read(t, dir, "Vendedor"), merchant) {
		t.Error("merchant template was modified")
	}
	if !bytes.Equal(read(t, dir, "Base"), levelZero) {
		t.Error("level-0 template was modified")
	}
}

// TestStampLeavesLegacyTemplatesAlone pins the deliberate exception of issue
// #244: the legacy layouts are decodable everywhere else, but restamping them
// in place is not possible without rewriting the whole record (their Exp is a
// 32-bit field at offset 28), so Stamp reports them instead of touching them.
func TestStampLeavesLegacyTemplatesAlone(t *testing.T) {
	dir := t.TempDir()
	legacy := make([]byte, savefmt.MobSizeLegacy756)
	for i := range legacy {
		legacy[i] = byte(i % 251)
	}
	padded := make([]byte, savefmt.MobSizeLegacy756Padded)
	for i := range padded {
		padded[i] = byte(i % 241)
	}
	monster := template("Cobaia", 10, 0, 0)
	write(t, dir, "Legacy", legacy)
	write(t, dir, "Padded", padded)
	write(t, dir, "Cobaia", monster)

	res, err := Stamp(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Stamped) != 1 || res.SkippedVariant != 2 || res.SkippedNonTemplate != 0 {
		t.Fatalf("stamped %d / variant %d / non-template %d, want 1 / 2 / 0",
			len(res.Stamped), res.SkippedVariant, res.SkippedNonTemplate)
	}
	if !bytes.Equal(read(t, dir, "Legacy"), legacy) {
		t.Error("756-byte template was rewritten")
	}
	if !bytes.Equal(read(t, dir, "Padded"), padded) {
		t.Error("920-byte template was rewritten")
	}
}

func TestStampDryRun(t *testing.T) {
	dir := t.TempDir()
	monster := template("Cobaia", 10, 0, 0)
	write(t, dir, "Cobaia", monster)

	res, err := Stamp(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Stamped) != 1 || res.Stamped[0].NewExp != level.MobExpForLevel(10) {
		t.Fatalf("dry-run report = %+v, want the would-be stamp", res.Stamped)
	}
	if !bytes.Equal(read(t, dir, "Cobaia"), monster) {
		t.Error("dry-run wrote to disk")
	}
}
