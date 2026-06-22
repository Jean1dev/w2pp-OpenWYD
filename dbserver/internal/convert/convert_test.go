package convert

import (
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/savefmt"
)

func TestHashSecret(t *testing.T) {
	if h, err := HashSecret(""); err != nil || h != "" {
		t.Errorf("HashSecret(\"\") = %q, %v; want empty", h, err)
	}
	h1, err := HashSecret("secret")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h1, "$argon2id$") {
		t.Errorf("hash not PHC-encoded: %q", h1)
	}
	if strings.Contains(h1, "secret") {
		t.Errorf("hash leaks plaintext")
	}
	h2, _ := HashSecret("secret")
	if h1 == h2 {
		t.Errorf("expected distinct salts to yield distinct hashes")
	}
}

func TestAccountConversion(t *testing.T) {
	var af savefmt.AccountFile
	copy(af.Info.AccountName[:], "Tester") // mixed case → canonical lowercase
	copy(af.Info.AccountPass[:], "secret")
	copy(af.Info.NumericToken[:], "1234")
	copy(af.Info.Email[:], "a@b.com")
	copy(af.BlockPass[:], "blockpw")
	af.Coin = 5000
	af.Donate = 99
	af.IsBlocked = true

	// Slot 0 empty (no name); slot 1 has a character.
	copy(af.Char[1].Name[:], "Hero")
	af.Char[1].Class = 3
	af.Char[1].CurrentScore = savefmt.Score{Level: 100, Str: 50, Hp: 1200}
	af.Char[1].Exp = 9_000_000_000
	af.Char[1].Equip[0] = savefmt.Item{Index: 1100, Effects: [3]savefmt.Effect{{Effect: 1, Value: 6}, {}, {}}}
	af.Char[1].Equip[2] = savefmt.Item{Index: 0} // empty → dropped
	af.Char[1].Carry[10] = savefmt.Item{Index: 2200}
	af.MobExtra[1] = savefmt.MobExtra{} // ClassMaster/Citizen = 0
	af.Affect[1][0] = savefmt.Affect{Type: 5, Value: 1, Level: 2, Time: 60}
	af.Cargo[3] = savefmt.Item{Index: 4444}

	acc, err := Account(af)
	if err != nil {
		t.Fatal(err)
	}

	if acc.Name != "tester" {
		t.Errorf("Name = %q, want tester", acc.Name)
	}
	if acc.PassHash == "" || strings.Contains(acc.PassHash, "secret") {
		t.Errorf("password not hashed: %q", acc.PassHash)
	}
	if acc.PinHash == "" {
		t.Errorf("PIN not hashed")
	}
	if acc.BlockPassHash == "" {
		t.Errorf("block password not hashed")
	}
	if acc.CargoCoin != 5000 || acc.DonateBalance != 99 || !acc.IsBlocked {
		t.Errorf("account scalars wrong: %+v", acc)
	}
	if len(acc.Cargo) != 1 || acc.Cargo[0].Slot != 3 || acc.Cargo[0].Index != 4444 {
		t.Errorf("cargo = %+v", acc.Cargo)
	}

	if len(acc.Characters) != 1 {
		t.Fatalf("characters = %d, want 1 (empty slot 0 skipped)", len(acc.Characters))
	}
	ch := acc.Characters[0]
	if ch.Slot != 1 || ch.Name != "Hero" || ch.Class != 3 || ch.Level != 100 || ch.Exp != 9_000_000_000 {
		t.Errorf("character = %+v", ch)
	}
	if len(ch.Equip) != 1 || ch.Equip[0].Slot != 0 || ch.Equip[0].Index != 1100 || ch.Equip[0].Eff1 != 1 || ch.Equip[0].EffV1 != 6 {
		t.Errorf("equip = %+v", ch.Equip)
	}
	if len(ch.Carry) != 1 || ch.Carry[0].Slot != 10 || ch.Carry[0].Index != 2200 {
		t.Errorf("carry = %+v", ch.Carry)
	}
	if len(ch.Affects) != 1 || ch.Affects[0].Type != 5 {
		t.Errorf("affects = %+v", ch.Affects)
	}
}

func TestAccountEmptyNameRejected(t *testing.T) {
	var af savefmt.AccountFile // all zero → empty name
	if _, err := Account(af); err == nil {
		t.Errorf("expected error for empty account name")
	}
}
