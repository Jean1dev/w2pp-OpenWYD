package dbclient

import (
	"testing"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// The equipment-free base (migration 0027) crosses the wire both ways; a save
// always carries it, and a row without it loads with HasBase false.
func TestBaseScoreMapping(t *testing.T) {
	c := characterSaveToProto(world.CharacterSave{Str: 212, BaseStr: 12, BaseInt: 13, BaseDex: 2512, BaseCon: 860,
		BaseMaxHP: 2000, BaseMaxMP: 500})
	if !c.GetHasBaseScore() || c.GetBaseStr() != 12 || c.GetBaseInt() != 13 || c.GetBaseDex() != 2512 ||
		c.GetBaseCon() != 860 || c.GetBaseMaxHp() != 2000 || c.GetBaseMaxMp() != 500 || c.GetStr() != 212 {
		t.Fatalf("save mapping %+v", c)
	}
	st := characterStateFromProto(c)
	if !st.HasBase || st.BaseStr != 12 || st.BaseInt != 13 || st.BaseDex != 2512 || st.BaseCon != 860 ||
		st.BaseMaxHP != 2000 || st.BaseMaxMP != 500 || st.Str != 212 {
		t.Fatalf("load mapping %+v", st)
	}
	if old := characterStateFromProto(&dbv1.Character{Str: 112}); old.HasBase {
		t.Fatal("a row without the base must load with HasBase false")
	}
}
