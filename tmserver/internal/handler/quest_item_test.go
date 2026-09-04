package handler

import (
	"encoding/binary"
	"fmt"
	"net"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

func questRewardDB(item int16, lvl int, amount uint8) *fakeDB {
	db := newDB()
	st := world.CharacterState{
		Slot: 0, Name: "Hero", ClassMaster: classMasterMortal, Level: lvl,
		X: 5, Y: 5, HP: 1000, MaxHP: 1000,
	}
	st.Carry[0] = world.Item{Index: item}
	if amount > 1 {
		st.Carry[0].Effects[0] = world.Effect{Effect: efAmount, Value: amount}
	}
	db.loadResult = st
	return db
}

func useQuestItem(t *testing.T, c net.Conn) {
	t.Helper()
	body := protocol.MsgUseItemBody{SourType: world.ItemPlaceCarry, SourPos: 0}
	send(t, c, protocol.MsgUseItem, body.Encode())
}

func collectQuestResult(t *testing.T, c net.Conn, limit int) (exp int64, coin int32, item []byte, sawPanel bool) {
	t.Helper()
	for range limit {
		ty, payload, ok := readMaybe(t, c)
		if !ok {
			break
		}
		switch ty {
		case protocol.MsgExpPanel:
			sawPanel = true
		case protocol.MsgUpdateEtc:
			exp = int64(binary.LittleEndian.Uint64(payload[4:12]))
			coin = int32(binary.LittleEndian.Uint32(payload[28:32]))
		case protocol.MsgSendItem:
			item = payload
		}
	}
	return
}

func TestQuestItemRewardAllTiersAndStackConsumption(t *testing.T) {
	tiers := []struct {
		item int16
		lvl  int
		exp  int64
		coin int32
	}{{4117, 39, 1000, 2000}, {4118, 115, 2000, 4000}, {4119, 190, 3000, 6000}, {4120, 265, 4000, 8000}, {4121, 320, 5000, 10000}}
	for _, tc := range tiers {
		t.Run(fmt.Sprintf("item-%d", tc.item), func(t *testing.T) {
			addr, stop := startServerClockVol(t, questRewardDB(tc.item, tc.lvl, 3), map[int]int{int(tc.item): volQuestReward})
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()
			useQuestItem(t, c)
			exp, coin, item, panel := collectQuestResult(t, c, 10)
			if exp != tc.exp || coin != tc.coin || !panel {
				t.Errorf("reward = exp %d coin %d panel %v, want %d %d true", exp, coin, panel, tc.exp, tc.coin)
			}
			if len(item) < 8 || le16(item[4:6]) != uint16(tc.item) || item[6] != efAmount || item[7] != 2 {
				t.Errorf("remaining stack = %v, want item %d amount 2", item, tc.item)
			}
		})
	}
}

func TestQuestItemRewardLevelBoundariesRejectWithoutConsumption(t *testing.T) {
	for _, lvl := range []int{38, 115} {
		addr, stop := startServerClockVol(t, questRewardDB(4117, lvl, 3), map[int]int{4117: volQuestReward})
		c := enterWorld(t, addr)
		useQuestItem(t, c)
		if notice := expect(t, c, protocol.MsgMessageBoxOk); noticeCode(t, notice) != NoticeReqNotMet {
			t.Errorf("level %d notice = %d", lvl, noticeCode(t, notice))
		}
		item := expect(t, c, protocol.MsgSendItem)
		if le16(item[4:6]) != 4117 || item[7] != 3 {
			t.Errorf("level %d changed rejected stack: %v", lvl, item)
		}
		c.Close()
		stop()
	}
}

func TestQuestItemRewardCaps(t *testing.T) {
	db := questRewardDB(4117, 39, 1)
	db.loadResult.Exp = level.MaxExp - 10
	db.loadResult.Coin = maxCoin - 10
	addr, stop := startServerClockVol(t, db, map[int]int{4117: volQuestReward})
	defer stop()
	c := enterWorld(t, addr)
	defer c.Close()
	useQuestItem(t, c)
	exp, coin, item, _ := collectQuestResult(t, c, 10)
	if exp != level.MaxExp || coin != maxCoin {
		t.Errorf("capped reward = exp %d coin %d, want %d %d", exp, coin, level.MaxExp, maxCoin)
	}
	if len(item) < 6 || le16(item[4:6]) != 0 {
		t.Errorf("final unit not consumed: %v", item)
	}
}

func TestQuestItemRewardPartyMemberGetsTenPercent(t *testing.T) {
	db := partyDB()
	leader := db.loads[7]
	leader.Carry[0] = world.Item{Index: 4117}
	db.loads[7] = leader
	addr, stop := startServerClockVol(t, db, map[int]int{4117: volQuestReward})
	defer stop()
	a := enterWorldAs(t, addr, "tester")
	defer a.Close()
	b := enterWorldAs(t, addr, "tradeb")
	defer b.Close()

	reqPartyFrame(t, a, 1, 2)
	expectPartyFrame(t, b, protocol.MsgSendReqParty)
	acceptPartyFrame(t, b, 1, "Hero")
	for {
		if _, _, ok := readMaybe(t, a); !ok {
			break
		}
	}
	for {
		if _, _, ok := readMaybe(t, b); !ok {
			break
		}
	}

	useQuestItem(t, a)
	leaderExp, _, _, _ := collectQuestResult(t, a, 10)
	memberExp, memberCoin, _, panel := collectQuestResult(t, b, 8)
	if leaderExp != 1000 {
		t.Errorf("consumer exp = %d, want 1000 (no self share)", leaderExp)
	}
	if memberExp != 100 || memberCoin != 0 || !panel {
		t.Errorf("member reward = exp %d coin %d panel %v, want 100 0 true", memberExp, memberCoin, panel)
	}
}
