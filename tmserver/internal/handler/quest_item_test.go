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

// TestQuestItemRewardAllTiersAndStackConsumption covers a solo consumer: with no
// party the legacy leader is the consumer itself (_MSG_UseItem.cpp:2403-2414),
// so the EXP is the table value plus its 10% leader share (issue #396).
func TestQuestItemRewardAllTiersAndStackConsumption(t *testing.T) {
	tiers := []struct {
		item int16
		lvl  int
		exp  int64
		coin int32
	}{{4117, 39, 1100, 2000}, {4118, 115, 2200, 4000}, {4119, 190, 3300, 6000}, {4120, 265, 4400, 8000}, {4121, 320, 5500, 10000}}
	for _, tc := range tiers {
		t.Run(fmt.Sprintf("item-%d", tc.item), func(t *testing.T) {
			addr, stop := startServerClockVol(t, questRewardDB(tc.item, tc.lvl, 3), map[int]int{int(tc.item): volQuestReward})
			defer stop()
			c := enterWorld(t, addr)
			defer c.Close()
			useQuestItem(t, c)
			exp, coin, item, panel := collectQuestResult(t, c, 20)
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
	exp, coin, item, _ := collectQuestResult(t, c, 20)
	if exp != level.MaxExp || coin != maxCoin {
		t.Errorf("capped reward = exp %d coin %d, want %d %d", exp, coin, level.MaxExp, maxCoin)
	}
	if len(item) < 6 || le16(item[4:6]) != 0 {
		t.Errorf("final unit not consumed: %v", item)
	}
}

// TestQuestItemRewardPartyMemberGetsTenPercent ports the party split of
// _MSG_UseItem.cpp:2403-2480: the leader always gets QuestExp/10 (also when it
// is the consumer, issue #396) and the leader's members get it except the
// consumer.
func TestQuestItemRewardPartyMemberGetsTenPercent(t *testing.T) {
	cases := []struct {
		name                 string
		leaderUses           bool
		leaderExp, memberExp int64
	}{
		{"leader-consumes", true, 1100, 100},
		{"member-consumes", false, 100, 1000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testQuestItemRewardParty(t, tc.leaderUses, tc.leaderExp, tc.memberExp)
		})
	}
}

func testQuestItemRewardParty(t *testing.T, leaderUses bool, wantLeader, wantMember int64) {
	t.Helper()
	db := partyDB()
	consumerKey := int64(11)
	if leaderUses {
		consumerKey = 7
	}
	consumer := db.loads[consumerKey]
	consumer.Carry[0] = world.Item{Index: 4117}
	db.loads[consumerKey] = consumer
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

	if leaderUses {
		useQuestItem(t, a)
	} else {
		useQuestItem(t, b)
	}
	leaderExp, _, _, leaderPanel := collectQuestResult(t, a, 20)
	memberExp, _, _, memberPanel := collectQuestResult(t, b, 20)
	if leaderExp != wantLeader || !leaderPanel {
		t.Errorf("leader exp = %d panel %v, want %d true", leaderExp, leaderPanel, wantLeader)
	}
	if memberExp != wantMember || !memberPanel {
		t.Errorf("member exp = %d panel %v, want %d true", memberExp, memberPanel, wantMember)
	}
}
