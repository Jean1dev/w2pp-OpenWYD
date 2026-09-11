package grpcsrv

import (
	"context"
	"testing"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
)

// TestMortalNovoNasceSemOuroNoNivelZero: a new character starts at level 0
// (level 1 on screen) with no gold — the team rule of 2026-09-11. It used to
// start at level 1 with 1 000 000 gold.
func TestMortalNovoNasceSemOuroNoNivelZero(t *testing.T) {
	fs := &fakeStore{}
	resp, err := New(fs).CreateCharacter(context.Background(),
		&dbv1.CreateCharacterRequest{AccountId: 1, Slot: 0, Name: "novato", Class: 2})
	if err != nil || !resp.GetOk() {
		t.Fatalf("CreateCharacter: ok=%v err=%v", resp.GetOk(), err)
	}
	ch := fs.createdChar
	if ch.Coin != 0 {
		t.Errorf("nasceu com %d de ouro, want 0", ch.Coin)
	}
	if ch.Level != 0 {
		t.Errorf("nasceu no nível %d, want 0 (nível 1 na tela)", ch.Level)
	}
	if ch.Exp != 0 {
		t.Errorf("nasceu com %d de experiência, want 0", ch.Exp)
	}
}

// TestArchNasceSemOuro: the Arch twin starts with no gold either, as
// _MSG_DBCreateArchCharacter does (CFileDB.cpp:1916).
func TestArchNasceSemOuro(t *testing.T) {
	fs := &fakeStore{}
	resp, err := New(fs).CreateArchCharacter(context.Background(),
		&dbv1.CreateArchCharacterRequest{AccountId: 1, Name: "hero", Class: 1, MortalFace: 21, MortalSlot: 0, MortalLevel: 399})
	if err != nil || !resp.GetOk() {
		t.Fatalf("CreateArchCharacter: ok=%v err=%v", resp.GetOk(), err)
	}
	if fs.archChar.Coin != 0 {
		t.Errorf("o Arch nasceu com %d de ouro, want 0", fs.archChar.Coin)
	}
}
