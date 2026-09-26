package handler

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// archRelogDB rehydrates successful snapshots for socket-level login tests.
// Separate dbclient/DBServer/store tests exercise the actual persistence mappings.
type archRelogDB struct{ *fakeDB }

func (db *archRelogDB) LoadCharacter(ctx context.Context, accountID int64, slot int) (world.CharacterState, error) {
	st, err := db.fakeDB.LoadCharacter(ctx, accountID, slot)
	db.mu.Lock()
	defer db.mu.Unlock()
	for i := len(db.savedChars) - 1; i >= 0; i-- {
		save := db.savedChars[i]
		if save.AccountID != accountID || save.Slot != slot {
			continue
		}
		st.Level, st.Exp, st.ClassMaster = int(save.Level), save.Exp, save.ClassMaster
		st.ArchCrystalStage, st.ArchLv355, st.ArchLv370 = save.ArchCrystalStage, save.ArchLv355, save.ArchLv370
		st.MortalLevel, st.CelestialArchLevel, st.Fame = save.MortalLevel, save.CelestialArchLevel, save.Fame
		st.Str, st.Int, st.Dex, st.Con = save.Str, save.Int, save.Dex, save.Con
		st.HP, st.MaxHP, st.MP, st.MaxMP = save.HP, save.MaxHP, save.MP, save.MaxMP
		st.LearnedSkill, st.ScoreBonus, st.SpecialBonus = save.LearnedSkill, save.ScoreBonus, save.SpecialBonus
		st.BaseSpecial, st.SkillBar = save.BaseSpecial, save.SkillBar
		st.Equip, st.Carry = [world.MaxEquip]world.Item{}, [world.MaxCarry]world.Item{}
		for _, group := range []struct {
			items []world.SavedItem
			dst   []world.Item
		}{{save.Equip, st.Equip[:]}, {save.Carry, st.Carry[:]}} {
			for _, it := range group.items {
				group.dst[it.Slot] = world.Item{Index: it.Index, Effects: [3]world.Effect{{Effect: it.Eff1, Value: it.EffV1}, {Effect: it.Eff2, Value: it.EffV2}, {Effect: it.Eff3, Value: it.EffV3}}}
			}
		}
		break
	}
	return st, err
}

func archState(lvl int) world.CharacterState {
	return world.CharacterState{Slot: 0, Name: "Hero", Class: 0, ClassMaster: classMasterArch, Level: lvl,
		X: 5, Y: 5, HP: 1000, MaxHP: 1000, MP: 500, MaxMP: 500,
		Str: 8, Int: 4, Dex: 7, Con: 6, MortalLevel: 399, Exp: 2_000_000_000}
}

func useArchItem(t *testing.T, c net.Conn, slot int) {
	t.Helper()
	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: int32(slot)}
	send(t, c, protocol.MsgUseItem, body.Encode())
}

func relogArch(t *testing.T, c net.Conn) []byte {
	t.Helper()
	send(t, c, protocol.MsgCharacterLogout, nil)
	expect(t, c, protocol.MsgCNFCharacterLogout)
	send(t, c, protocol.MsgCharacterLogin, (&protocol.MsgCharacterLoginBody{}).Encode())
	expect(t, c, protocol.MsgCNFCharacterLogin)
	return expect(t, c, protocol.MsgUpdateScore)
}

func TestArchCrystalSequenceSurvivesRelog(t *testing.T) {
	db := &archRelogDB{newDB()}
	db.loadResult = archState(399)
	db.loadResult.Exp = level.MaxExp
	for i := range 4 {
		db.loadResult.Carry[i] = world.Item{Index: int16(4106 + i), Effects: [3]world.Effect{{Effect: efAmount, Value: 2}}}
	}
	addr, stop, _ := startServerClock(t, db)
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()
	bonuses := [4][3]uint32{{0, 0, 80}, {30, 0, 80}, {30, 80, 80}, {50, 140, 140}}
	for i, bonus := range bonuses {
		wantExp := level.MaxExp - int64(i+1)*100_000_000
		wantLevel := level.ForExpTier(wantExp, classMasterArch)
		lost := uint32(399 - wantLevel)
		wantAC := uint32(628) - lost + bonus[0]
		wantHP, wantMP := uint32(1000)-3*lost+bonus[1], uint32(500)-lost+bonus[2]
		useArchItem(t, c, i)
		slot := expect(t, c, protocol.MsgSendItem)
		if le16(slot[4:6]) != uint16(4106+i) || slot[7] != 1 {
			t.Fatalf("stage %d did not consume exactly one crystal: %v", i+1, slot)
		}
		score := expect(t, c, protocol.MsgUpdateScore)
		if le(score[:4]) != uint32(wantLevel) || le(score[4:8]) != wantAC || le(score[16:20]) != wantHP || le(score[20:24]) != wantMP {
			t.Fatalf("stage %d score AC/HP/MP=%d/%d/%d", i+1, le(score[4:8]), le(score[16:20]), le(score[20:24]))
		}
		expect(t, c, protocol.MsgMessageChat)
		save, _ := db.lastSavedChar()
		if save.ArchCrystalStage != uint8(i+1) || save.Exp != wantExp || save.Level != wantLevel {
			t.Fatalf("stage %d save stage/EXP/level=%d/%d/%d", i+1, save.ArchCrystalStage, save.Exp, save.Level)
		}
		score = relogArch(t, c)
		if le(score[:4]) != uint32(wantLevel) || le(score[4:8]) != wantAC || le(score[16:20]) != wantHP || le(score[20:24]) != wantMP {
			t.Fatalf("stage %d bonuses lost or duplicated after relog", i+1)
		}
		// The remaining stacked crystal is now a repeat, including after relog.
		useArchItem(t, c, i)
		expect(t, c, protocol.MsgMessageChat)
		if slot := expect(t, c, protocol.MsgSendItem); slot[7] != 1 {
			t.Fatalf("stage %d repeat consumed the last crystal", i+1)
		}
	}
}

func TestArchCrystalRejectionAndSaveFailure(t *testing.T) {
	for _, tc := range []struct {
		name  string
		tier  uint8
		lvl   int
		stage uint8
		item  int16
		fail  bool
	}{
		{"mortal", classMasterMortal, 399, 0, 4106, false},
		{"celestial", classMasterCelestial, 399, 0, 4106, false},
		{"below level", classMasterArch, 354, 0, 4106, false},
		{"out of order", classMasterArch, 355, 0, 4107, false},
		{"already done", classMasterArch, 355, 4, 4109, false},
		{"save failed", classMasterArch, 355, 0, 4106, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newDB()
			db.loadResult = archState(tc.lvl)
			db.loadResult.ClassMaster, db.loadResult.ArchCrystalStage = tc.tier, tc.stage
			db.loadResult.Carry[0] = world.Item{Index: tc.item}
			if tc.fail {
				db.loadResult.Exp = level.NextLevelExp(354)
				db.saveErr = errors.New("injected failure")
			}
			addr, stop, _ := startServerClock(t, db)
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()
			useArchItem(t, c, 0)
			expect(t, c, protocol.MsgMessageChat)
			if slot := expect(t, c, protocol.MsgSendItem); int16(le16(slot[4:6])) != tc.item {
				t.Fatal("rejection lost item")
			}
			send(t, c, protocol.MsgCharacterLogout, nil)
			expect(t, c, protocol.MsgCNFCharacterLogout)
			save, _ := db.lastSavedChar()
			if save.Level != int32(tc.lvl) || save.ArchCrystalStage != tc.stage || save.Exp != db.loadResult.Exp || save.MaxHP != 1000 || save.MaxMP != 500 || !hasItem(save.Carry, tc.item) {
				t.Fatalf("rejection mutated state: %+v", save)
			}
		})
	}
}

func TestArchCrystalLevelLossSurvivesRelog(t *testing.T) {
	for cls := uint8(0); cls < 4; cls++ {
		for stage := uint8(1); stage <= 4; stage++ {
			t.Run(fmt.Sprintf("class%d/stage%d", cls, stage), func(t *testing.T) {
				db := &archRelogDB{newDB()}
				st := archState(355)
				st.Class, st.ArchCrystalStage = int(cls), stage-1
				st.ArchLv355, st.ArchLv370 = 1, 1
				st.Exp = level.NextLevelExp(354)
				st.ScoreBonus, st.SpecialBonus = 100, 100
				st.LearnedSkill = 1
				st.Carry[0] = world.Item{Index: 4105 + int16(stage)}
				st.Carry[1] = world.Item{Index: 4106 + int16(stage)}
				db.loadResult = st
				spells := content.NewSkillData([]content.Spell{{Index: int(cls) * content.MaxSkill, SkillPoint: 3}})
				addr, stop := startServerSkillsWithConfig(t, db, Config{Spells: spells})
				defer stop()
				c := enterWorld(t, addr)
				defer c.Close()
				useArchItem(t, c, 0)
				expect(t, c, protocol.MsgSendItem)
				score := expect(t, c, protocol.MsgUpdateScore)
				etc := expect(t, c, protocol.MsgUpdateEtc)
				expect(t, c, protocol.MsgMessageChat)
				wantExp := st.Exp - 100_000_000
				wantLevel := level.ForExpTier(wantExp, classMasterArch)
				lost := int32(st.Level) - wantLevel
				hpReward, mpReward := int32(0), int32(0)
				switch stage {
				case 1:
					mpReward = 80
				case 3:
					hpReward = 80
				case 4:
					hpReward, mpReward = 60, 60
				}
				wantHP := st.MaxHP - lost*level.IncHP(cls) + hpReward
				wantMP := st.MaxMP - lost*level.IncMP(cls) + mpReward
				wantAC := playerBaseAC(&world.Entity{ClassMaster: classMasterArch, Level: wantLevel, ArchCrystalStage: stage})
				wantPoints := uint16(level.ScoreBonus(cls, wantLevel, st.Str, st.Int, st.Dex, st.Con))
				wantSpecial := uint16(100 - 2*lost)
				wantSkill := uint16(wantLevel*3 + max(wantLevel-199, 0) - 3)
				checkScore := func(score []byte) {
					t.Helper()
					if le(score[:4]) != uint32(wantLevel) || le(score[4:8]) != uint32(wantAC) || le(score[16:20]) != uint32(wantHP) || le(score[20:24]) != uint32(wantMP) {
						t.Fatalf("unexpected level/AC/HP/MP: %v", score[:24])
					}
				}
				checkScore(score)
				if int64(binary.LittleEndian.Uint64(etc[4:12])) != wantExp || le16(etc[20:22]) != wantPoints || le16(etc[22:24]) != wantSpecial || le16(etc[24:26]) != wantSkill {
					t.Fatalf("unexpected EXP/points packet: %v", etc)
				}
				save, _ := db.lastSavedChar()
				if save.Level != wantLevel || save.Exp != wantExp || save.MaxHP != wantHP || save.MaxMP != wantMP || save.ScoreBonus != wantPoints || save.SpecialBonus != wantSpecial || save.ArchCrystalStage != stage || save.ArchLv355 != 1 || save.ArchLv370 != 1 || hasItem(save.Carry, st.Carry[0].Index) {
					t.Fatalf("incorrect crystal snapshot: %+v", save)
				}
				if save.HP != min(st.HP, wantHP) || save.MP != min(st.MP, wantMP) {
					t.Fatalf("crystal healed or failed to cap resources: %d/%d", save.HP, save.MP)
				}
				send(t, c, protocol.MsgCharacterLogout, nil)
				expect(t, c, protocol.MsgCNFCharacterLogout)
				send(t, c, protocol.MsgCharacterLogin, (&protocol.MsgCharacterLoginBody{}).Encode())
				login := expect(t, c, protocol.MsgCNFCharacterLogin)
				// STRUCT_MOB begins at body offset 4; free points are at 788..793.
				if le16(login[792:794]) != wantPoints || le16(login[794:796]) != wantSpecial || le16(login[796:798]) != wantSkill {
					t.Fatal("free points changed after relog")
				}
				checkScore(expect(t, c, protocol.MsgUpdateScore))
				if stage < 4 {
					useArchItem(t, c, 1)
					expect(t, c, protocol.MsgMessageChat)
					if slot := expect(t, c, protocol.MsgSendItem); int16(le16(slot[4:6])) != st.Carry[1].Index {
						t.Fatal("below-level rejection consumed the next crystal")
					}
				}
			})
		}
	}
}

func TestArchCrystalExperienceBoundaries(t *testing.T) {
	threshold := level.NextLevelExp(354)
	for _, tc := range []struct {
		name      string
		exp       int64
		wantLevel int32
	}{
		{"above", threshold + 100_000_001, 355},
		{"exact", threshold + 100_000_000, 355},
		{"below", threshold + 99_999_999, 354},
		{"no level up", level.MaxExp, 355},
		{"zero", 100_000_000, 0},
		{"clamp", 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &archRelogDB{newDB()}
			db.loadResult = archState(355)
			db.loadResult.Exp = tc.exp
			// Already allocated points must survive even when no free points remain.
			db.loadResult.Str = 5000
			db.loadResult.BaseSpecial[0] = 100
			db.loadResult.LearnedSkill = 1
			db.loadResult.Carry[0] = world.Item{Index: 4106}
			addr, stop, _ := startServerClock(t, db)
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()
			useArchItem(t, c, 0)
			expect(t, c, protocol.MsgSendItem)
			score := expect(t, c, protocol.MsgUpdateScore)
			expect(t, c, protocol.MsgMessageChat)
			save, _ := db.lastSavedChar()
			if save.Level != tc.wantLevel || le(score[:4]) != uint32(tc.wantLevel) || save.Exp != max(tc.exp-100_000_000, 0) || save.ScoreBonus != 0 || save.SpecialBonus != 0 || save.Str != 5000 || save.BaseSpecial[0] != 100 || save.LearnedSkill != 1 {
				t.Fatalf("incorrect boundary snapshot: %+v", save)
			}
			if got := relogArch(t, c); le(got[:4]) != uint32(tc.wantLevel) {
				t.Fatal("level changed after relog")
			}
		})
	}
}

func TestArchLevelLocksAcrossLargeRewards(t *testing.T) {
	d, w, e := mobKilledWorld(t)
	e.ClassMaster, e.Level, e.Exp = classMasterArch, 353, level.MaxExp
	d.applyLevelUps(w, nil, e)
	if e.Level != 354 || e.SkillBonus != 4 || e.SpecialBonus != 2 {
		t.Fatalf("first gate: level/skill/special=%d/%d/%d", e.Level, e.SkillBonus, e.SpecialBonus)
	}
	e.ArchLv355 = 1
	d.applyLevelUps(w, nil, e)
	if e.Level != 369 {
		t.Fatalf("second gate: level=%d", e.Level)
	}
	e.ArchLv370 = 1
	d.applyLevelUps(w, nil, e)
	if e.Level != level.MaxLevel {
		t.Fatalf("unlocked level=%d", e.Level)
	}
}

func TestArchLockedExpSources(t *testing.T) {
	for _, lvl := range []int32{354, 369, 399} {
		for _, source := range []string{"combat", "direct", "castle"} {
			t.Run(fmt.Sprintf("%s/%d", source, lvl), func(t *testing.T) {
				d, w, e := mobKilledWorld(t)
				e.ClassMaster, e.Level, e.Exp = classMasterArch, lvl, 1234
				if lvl == 369 {
					e.ArchLv355 = 1
				}
				switch source {
				case "castle":
					d.rewardCastlePlayer(w, e, content.CastleQuest{ExpPrize: [6]int64{0, 1000000}})
				case "combat":
					d.grantExp(w, nil, e, &world.Entity{Level: lvl, Exp: 1000000})
				default:
					d.grantDirectExp(w, nil, e, 1000000)
				}
				if e.Exp != 1234 || e.Level != lvl {
					t.Fatalf("locked source changed EXP/level: %d/%d", e.Exp, e.Level)
				}
			})
		}
	}
}

func TestArchFairyDustBoundaries(t *testing.T) {
	for _, tc := range []struct {
		lvl           int
		first, second uint8
		want          int
		consumed      bool
	}{
		{353, 0, 0, 354, true}, {354, 0, 0, 354, false}, {354, 1, 0, 355, true},
		{368, 1, 0, 369, true}, {369, 1, 0, 369, false}, {369, 1, 1, 370, true},
		{380, 0, 0, 380, false}, {380, 1, 0, 380, false},
	} {
		t.Run(fmt.Sprintf("%d/%d/%d", tc.lvl, tc.first, tc.second), func(t *testing.T) {
			const dust = 5001
			db := newDB()
			db.loadResult = archState(tc.lvl)
			db.loadResult.ArchLv355, db.loadResult.ArchLv370 = tc.first, tc.second
			db.loadResult.Carry[0] = world.Item{Index: dust, Effects: [3]world.Effect{{Effect: efAmount, Value: 2}}}
			addr, stop := startServerClockVol(t, db, map[int]int{dust: volFairyDust})
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()
			useArchItem(t, c, 0)
			if !tc.consumed {
				expect(t, c, protocol.MsgMessageChat)
			}
			slot := expect(t, c, protocol.MsgSendItem)
			amount := byte(2)
			if tc.consumed {
				amount = 1
				expect(t, c, protocol.MsgUpdateEtc)
			}
			if slot[7] != amount {
				t.Fatalf("dust count=%d want=%d", slot[7], amount)
			}
			send(t, c, protocol.MsgCharacterLogout, nil)
			expect(t, c, protocol.MsgCNFCharacterLogout)
			save, _ := db.lastSavedChar()
			if save.Level != int32(tc.want) {
				t.Fatalf("level=%d want=%d", save.Level, tc.want)
			}
			if !tc.consumed && save.Exp != db.loadResult.Exp {
				t.Fatal("locked dust changed EXP")
			}
		})
	}
}

func lindyRecipe(st *world.CharacterState, offset int) protocol.MsgCombineItemBody {
	var body protocol.MsgCombineItemBody
	for i := range 7 {
		it := world.Item{Index: 413}
		if i < 2 {
			it.Effects[0] = world.Effect{Effect: efAmount, Value: 10}
		}
		if i == 2 {
			it.Index = 4127
		}
		st.Carry[offset+i] = it
		body.InvenPos[i] = uint8(offset + i)
		body.Item[i] = protocol.WireItem{Index: it.Index}
		for j, ef := range it.Effects {
			body.Item[i].Effects[j] = protocol.WireEffect{Effect: ef.Effect, Value: ef.Value}
		}
	}
	return body
}

func TestArchLindyLateUnlockAndCapeBroadcast(t *testing.T) {
	for _, tc := range []struct {
		clan uint8
		cape uint16
	}{{0, 3193}, {7, 3191}, {8, 3192}} {
		t.Run(fmt.Sprint(tc.clan), func(t *testing.T) {
			db := &archRelogDB{newDB()}
			db.loadResult = archState(399)
			db.loadResult.Clan, db.loadResult.Fame = tc.clan, 1
			first := lindyRecipe(&db.loadResult, 0)
			second := lindyRecipe(&db.loadResult, 7)
			third := lindyRecipe(&db.loadResult, 14)
			addr, stop, _ := startServerClock(t, db)
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()
			watcher := enterWorldAs(t, addr, "tradeb")
			defer watcher.Close()
			send(t, c, protocol.MsgCombineItemLindy, first.Encode())
			selfEquip := expect(t, c, protocol.MsgUpdateEquip)
			watchEquip := expect(t, watcher, protocol.MsgUpdateEquip)
			if le16(selfEquip[30:32]) != tc.cape || le16(watchEquip[30:32]) != tc.cape {
				t.Fatalf("cape broadcast mismatch: own=%v observer=%v", selfEquip, watchEquip)
			}
			if parmOf(t, expect(t, c, protocol.MsgCombineComplete)) != combineSuccess {
				t.Fatal("first unlock failed")
			}
			save, _ := db.lastSavedChar()
			if save.Level != 399 || save.ArchLv355 != 1 || save.ArchLv370 != 0 || save.Fame != 1 {
				t.Fatalf("first unlock state: %+v", save)
			}
			relogArch(t, c)
			send(t, c, protocol.MsgCombineItemLindy, second.Encode())
			if parmOf(t, expect(t, c, protocol.MsgCombineComplete)) != combineSuccess {
				t.Fatal("second unlock failed")
			}
			save, _ = db.lastSavedChar()
			if save.ArchLv355 != 1 || save.ArchLv370 != 1 || save.Fame != 0 || !hasItem(save.Equip, int16(tc.cape)) {
				t.Fatalf("second unlock state: %+v", save)
			}
			send(t, c, protocol.MsgCombineItemLindy, third.Encode())
			if parmOf(t, expect(t, c, protocol.MsgCombineComplete)) != combineInvalid {
				t.Fatal("repeated unlock accepted")
			}
			send(t, c, protocol.MsgCharacterLogout, nil)
			expect(t, c, protocol.MsgCNFCharacterLogout)
			save, _ = db.lastSavedChar()
			if len(save.Carry) != 7 {
				t.Fatalf("repeat consumed recipe: %+v", save.Carry)
			}
		})
	}
}

func TestArchLindyRejectsAndPreservesIngredients(t *testing.T) {
	for _, kind := range []string{"below level", "missing fame", "duplicate slots", "invalid recipe", "save failure"} {
		t.Run(kind, func(t *testing.T) {
			db := newDB()
			db.loadResult = archState(354)
			body := lindyRecipe(&db.loadResult, 0)
			want := int32(combineInvalid)
			switch kind {
			case "below level":
				db.loadResult.Level = 353
			case "missing fame":
				db.loadResult.Level, db.loadResult.ArchLv355 = 369, 1
			case "duplicate slots":
				body.InvenPos[1] = 0
			case "invalid recipe":
				body.Item[2].Index, db.loadResult.Carry[2].Index = 4126, 4126
			case "save failure":
				db.saveErr = errors.New("injected failure")
				want = combineFailed
			}
			addr, stop, _ := startServerClock(t, db)
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()
			send(t, c, protocol.MsgCombineItemLindy, body.Encode())
			if got := parmOf(t, expect(t, c, protocol.MsgCombineComplete)); got != want {
				t.Fatalf("result=%d want=%d", got, want)
			}
			send(t, c, protocol.MsgCharacterLogout, nil)
			expect(t, c, protocol.MsgCNFCharacterLogout)
			save, _ := db.lastSavedChar()
			if len(save.Carry) != 7 || save.ArchLv355 != db.loadResult.ArchLv355 || save.ArchLv370 != 0 {
				t.Fatalf("failed unlock changed state: %+v", save)
			}
		})
	}
}

func TestArchIdealStoneCrystalPenalty(t *testing.T) {
	for stage := uint8(0); stage <= 4; stage++ {
		for _, lvl := range []int{355, 380, 399} {
			t.Run(fmt.Sprintf("%d/%d", stage, lvl), func(t *testing.T) {
				db := &archRelogDB{idealStoneDB(classMasterArch, lvl)}
				db.loadResult.ArchCrystalStage = stage
				db.loadResult.Carry[0].Effects[0] = world.Effect{Effect: efAmount, Value: 2}
				addr, stop, _ := startServerClock(t, db)
				defer stop()
				c := enterWorld(t, addr)
				defer c.Close()
				useArchItem(t, c, 0)
				expect(t, c, protocol.MsgCNFCharacterLogout)
				save, _ := db.lastSavedChar()
				want := int16(3500)
				if stage == 4 && lvl == 380 {
					want = 3501
				}
				if stage == 4 && lvl == 399 {
					want = 3502
				}
				if !hasItem(save.Equip, want) || save.ClassMaster != classMasterCelestial || save.ArchCrystalStage != stage || len(save.Carry) != 1 || save.Carry[0].EffV1 != 1 {
					t.Fatalf("wrong evolution outcome: %+v", save)
				}
				for _, it := range save.Equip {
					if it.Slot == 1 && (it.Eff1 != 0 || it.Eff2 != 0 || it.Eff3 != 0) {
						t.Fatal("Cythera is not +0")
					}
				}
				send(t, c, protocol.MsgCharacterLogin, (&protocol.MsgCharacterLoginBody{}).Encode())
				expect(t, c, protocol.MsgCNFCharacterLogin)
				score := expect(t, c, protocol.MsgUpdateScore)
				if le(score[0:4]) != 0 || le(score[4:8]) != 230 || le(score[16:20]) != 80 || le(score[20:24]) != 45 {
					t.Fatalf("Celestial reset/relog retained Arch bonuses: %v", score[:32])
				}
			})
		}
	}
}

func TestArchCompleteFairyDustJourney(t *testing.T) {
	const dust = 5001
	db := &archRelogDB{newDB()}
	db.loadResult = archState(353)
	db.loadResult.Fame = 1
	db.loadResult.Carry[0] = world.Item{Index: dust, Effects: [3]world.Effect{{Effect: efAmount, Value: 200}}}
	first := lindyRecipe(&db.loadResult, 1)
	second := lindyRecipe(&db.loadResult, 8)
	for i := range 4 {
		db.loadResult.Carry[15+i] = world.Item{Index: int16(4106 + i)}
	}
	db.loadResult.Carry[19] = world.Item{Index: idealStoneItem}
	addr, stop := startServerClockVol(t, db, map[int]int{dust: volFairyDust})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()
	advance := func(want uint32) {
		t.Helper()
		useArchItem(t, c, 0)
		if score := expect(t, c, protocol.MsgUpdateScore); le(score[:4]) != want {
			t.Fatalf("dust level=%d want=%d", le(score[:4]), want)
		}
		expect(t, c, protocol.MsgMotion)
	}
	unlock := func(recipe protocol.MsgCombineItemBody) {
		t.Helper()
		useArchItem(t, c, 0)
		expect(t, c, protocol.MsgMessageChat)
		if slot := expect(t, c, protocol.MsgSendItem); le16(slot[4:6]) != dust {
			t.Fatal("lock lost dust")
		}
		send(t, c, protocol.MsgCombineItemLindy, recipe.Encode())
		if parmOf(t, expect(t, c, protocol.MsgCombineComplete)) != combineSuccess {
			t.Fatal("unlock failed")
		}
		expect(t, c, protocol.MsgMessageChat)
	}
	advance(354)
	unlock(first)
	advance(355)
	recoveryDust := 0
	for i := range 4 {
		useArchItem(t, c, 15+i)
		score := expect(t, c, protocol.MsgUpdateScore)
		expect(t, c, protocol.MsgMessageChat)
		for lvl := le(score[:4]) + 1; lvl <= 355; lvl++ {
			advance(lvl)
			recoveryDust++
		}
	}
	for lvl := uint32(356); lvl <= 369; lvl++ {
		advance(lvl)
	}
	unlock(second)
	for lvl := uint32(370); lvl <= 399; lvl++ {
		advance(lvl)
	}
	useArchItem(t, c, 19)
	expect(t, c, protocol.MsgCNFCharacterLogout)
	save, _ := db.lastSavedChar()
	if save.ClassMaster != classMasterCelestial || save.Level != 0 || save.ArchCrystalStage != 4 || save.ArchLv355 != 1 || save.ArchLv370 != 1 || !hasItem(save.Equip, 3502) {
		t.Fatalf("full journey outcome: %+v", save)
	}
	if len(save.Carry) != 1 || save.Carry[0].Index != dust || int(save.Carry[0].EffV1) != 154-recoveryDust {
		t.Fatalf("incorrect journey consumption: %+v", save.Carry)
	}
}

func TestArchCombatExperienceReachesAndRespectsBothLocks(t *testing.T) {
	d, w, e := mobKilledWorld(t)
	e.ClassMaster = classMasterArch
	for _, boundary := range []int32{354, 369} {
		e.Level, e.Exp = boundary-1, level.NextLevelExp(boundary-1)-1
		mob := &world.Entity{Level: boundary, Exp: 1_000_000}
		d.grantExp(w, nil, e, mob)
		if e.Level != boundary {
			t.Fatalf("combat did not reach boundary %d: %d", boundary, e.Level)
		}
		exp := e.Exp
		d.grantExp(w, nil, e, mob)
		if e.Level != boundary || e.Exp != exp {
			t.Fatalf("combat bypassed boundary %d", boundary)
		}
		if boundary == 354 {
			e.ArchLv355 = 1
		} else {
			e.ArchLv370 = 1
		}
		e.Exp = level.NextLevelExp(boundary) - 1
		d.grantExp(w, nil, e, mob)
		if e.Level != boundary+1 {
			t.Fatalf("combat did not resume after boundary %d", boundary)
		}
	}
}

func TestArchIdealStoneRejectsMissingPrerequisites(t *testing.T) {
	for _, kind := range []string{"body armor", "mortal level"} {
		t.Run(kind, func(t *testing.T) {
			db := idealStoneDB(classMasterArch, 399)
			if kind == "body armor" {
				db.loadResult.Equip[1] = world.Item{Index: 1100}
			} else {
				db.loadResult.MortalLevel = 98
			}
			addr, stop, _ := startServerClock(t, db)
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()
			useArchItem(t, c, 0)
			expect(t, c, protocol.MsgMessageChat)
			if slot := expect(t, c, protocol.MsgSendItem); le16(slot[4:6]) != idealStoneItem {
				t.Fatal("rejected stone consumed")
			}
			send(t, c, protocol.MsgCharacterLogout, nil)
			expect(t, c, protocol.MsgCNFCharacterLogout)
			save, _ := db.lastSavedChar()
			if save.ClassMaster != classMasterArch || !hasItem(save.Carry, idealStoneItem) {
				t.Fatal("rejected evolution changed character")
			}
		})
	}
}
