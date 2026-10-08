package grpcsrv

import (
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/domain"
)

// The base columns of migration 0027 survive the domain <-> proto mapping, and
// HasBase travels with them (false keeps a pre-0027 row on the derive path).
func TestCharacterBaseScoreMapping(t *testing.T) {
	ch := domain.Character{Str: 212, HasBase: true, BaseStr: 12, BaseInt: 13, BaseDex: 2512, BaseCon: 860,
		BaseMaxHp: 2000, BaseMaxMp: 500}
	got := protoToCharacter(characterToProto(ch))
	if got.HasBase != ch.HasBase || got.BaseStr != ch.BaseStr || got.BaseInt != ch.BaseInt ||
		got.BaseDex != ch.BaseDex || got.BaseCon != ch.BaseCon || got.BaseMaxHp != ch.BaseMaxHp ||
		got.BaseMaxMp != ch.BaseMaxMp || got.Str != ch.Str {
		t.Fatalf("round trip %+v, want %+v", got, ch)
	}
	if protoToCharacter(characterToProto(domain.Character{Str: 112})).HasBase {
		t.Fatal("HasBase false must stay false")
	}
}
