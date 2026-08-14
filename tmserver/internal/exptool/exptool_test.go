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

// legacyTemplate builds a raw 756-byte legacy STRUCT_MOB (data-formats.md
// §1.4.1) with the same nonzero-pattern trick as template: Exp is a 32-bit
// field at offset 28 and the compact score sits at 64 (Level u16 @+0,
// Merchant @+6). size selects the plain 756 layout or the 920-byte padded one,
// whose trailing junk must survive the stamp untouched.
func legacyTemplate(name string, lvl uint16, exp int32, merchant byte, size int) []byte {
	b := make([]byte, size)
	for i := range b {
		b[i] = byte(i % 251)
	}
	for i := 0; i < 16; i++ {
		b[i] = 0
	}
	copy(b, name)
	binary.LittleEndian.PutUint32(b[28:], uint32(exp))
	const cs = 64
	binary.LittleEndian.PutUint16(b[cs+0:], lvl)
	b[cs+6] = merchant
	return b
}

// TestStampCoversLegacyTemplates is the fix for the residue issue #244 left
// behind: the legacy layouts used to be skipped, so 222 shipped monsters kept
// their authored Exp while the tmServer boot log kept telling operators to run
// this tool. They are restamped in place now — 4 bytes at offset 28 — and the
// file keeps its size, which was the original reason for skipping them.
func TestStampCoversLegacyTemplates(t *testing.T) {
	dir := t.TempDir()
	legacy := legacyTemplate("Antigo", 20, 999, 0, savefmt.MobSizeLegacy756)
	padded := legacyTemplate("Preenchido", 30, 0, 0, savefmt.MobSizeLegacy756Padded)
	shop := legacyTemplate("Loja", 2, 1, 1, savefmt.MobSizeLegacy756)
	monster := template("Cobaia", 10, 0, 0)
	write(t, dir, "Antigo", legacy)
	write(t, dir, "Preenchido", padded)
	write(t, dir, "Loja", shop)
	write(t, dir, "Cobaia", monster)

	res, err := Stamp(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Stamped) != 3 || res.StampedVariant != 2 || res.Skipped != 1 || res.SkippedNonTemplate != 0 {
		t.Fatalf("stamped %d (variant %d) / skipped %d / non-template %d, want 3 (2) / 1 / 0",
			len(res.Stamped), res.StampedVariant, res.Skipped, res.SkippedNonTemplate)
	}

	// Each legacy file changed in exactly the 4 Exp bytes — trailing junk of the
	// padded form included.
	for _, tc := range []struct {
		file   string
		before []byte
		lvl    int32
	}{
		{"Antigo", legacy, 20},
		{"Preenchido", padded, 30},
	} {
		got := read(t, dir, tc.file)
		if len(got) != len(tc.before) {
			t.Errorf("%s: size = %d, want %d", tc.file, len(got), len(tc.before))
			continue
		}
		patched := append([]byte(nil), tc.before...)
		binary.LittleEndian.PutUint32(patched[28:], uint32(level.MobExpForLevel(tc.lvl)))
		if !bytes.Equal(got, patched) {
			t.Errorf("%s: stamp touched bytes outside the legacy Exp field", tc.file)
		}
	}

	// A legacy merchant is still a merchant: its Merchant flag lives in the
	// compact score, so reading it through the canonical offsets would have
	// stamped it by mistake.
	if !bytes.Equal(read(t, dir, "Loja"), shop) {
		t.Error("legacy merchant template was modified")
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
