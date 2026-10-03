package handler

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

type kingDialogueDB struct {
	*fakeDB
	quote     world.KingdomCapeQuote
	purchases atomic.Int32
}

func (db *kingDialogueDB) QuoteKingdomCape(context.Context) (world.KingdomCapeQuote, error) {
	return db.quote, nil
}

func (db *kingDialogueDB) PurchaseKingdomCape(context.Context, int64, uint8, world.CharacterSave) (world.KingdomCapeQuote, bool, error) {
	db.purchases.Add(1)
	return db.quote, false, nil
}

func TestKingCapeDialogue(t *testing.T) {
	kings := []struct {
		name     string
		merchant uint8
		clan     uint8
		cape     int16
		cost     int
	}{
		{"Harabard legacy", 14, clanHekalotia, 543, 8},
		{"Glantuar legacy", 15, clanAkelonia, 544, 6},
		{"Harabard canonical", 111, clanHekalotia, 543, 8},
		{"Glantuar canonical", 111, clanAkelonia, 544, 6},
	}
	for _, king := range kings {
		for _, branch := range []string{"quote", "insufficient", "inexact", "complete"} {
			t.Run(king.name+"/"+branch, func(t *testing.T) {
				st := baseMortalState(255)
				st.Carry[0] = world.Item{Index: 697}
				wantCarry := []world.SavedItem{{Slot: 0, Index: 697}}
				confirm := int32(0)
				wantText := fmt.Sprintf("Sao necessarias %d safiras.", king.cost)
				switch branch {
				case "insufficient", "inexact":
					confirm = 1
					wantText = fmt.Sprintf("Sao necessarias %d safiras em pagamento exato.", king.cost)
					if branch == "inexact" {
						st.Carry[1] = world.Item{Index: 4131}
						wantCarry = append(wantCarry, world.SavedItem{Slot: 1, Index: 4131})
					}
				case "complete":
					st.Clan = king.clan
					st.Equip[capeEquipSlot] = world.Item{Index: king.cape}
					wantText = "A capa deste reino ja esta completa."
				}
				db := &kingDialogueDB{
					fakeDB: newDB(),
					quote:  world.KingdomCapeQuote{Revision: 7, HekalotiaCost: 8, AkeloniaCost: 6},
				}
				watcherState := baseMortalState(255)
				watcherState.Name = "Watcher"
				db.loads = map[int64]world.CharacterState{7: st, 11: watcherState}
				// STRUCT_MOB.Clan is at byte 16; the generic quest fixture's
				// clan argument writes CurrentScore.Damage instead.
				tmpl := questNPCTemplate(king.name, king.merchant, 0, 0)
				tmpl[16] = king.clan
				if got := protocol.ParseMobBasics(tmpl); got.Clan != king.clan || got.Merchant != king.merchant {
					t.Fatalf("King fixture decoded clan=%d merchant=%d, want %d/%d", got.Clan, got.Merchant, king.clan, king.merchant)
				}
				addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
				defer stop()
				player := enterWorldAs(t, addr, "tester") // First connection is player slot 1.
				defer player.Close()
				watcher := enterWorldAs(t, addr, "tradeb")
				defer watcher.Close()
				drainRaw(t, player)
				drainRaw(t, watcher)
				if npcID == 1 {
					t.Fatal("fixture must distinguish NPC and player identifiers")
				}

				send(t, player, protocol.MsgQuest, protocol.EncodeStandardParm2(int32(npcID), confirm))
				h, body, ok := readMaybeHeader(t, player)
				if !ok || h.Type != protocol.MsgMessageChat || h.ID != uint16(npcID) {
					t.Fatalf("dialogue header = %+v, received=%v; want chat from NPC %d", h, ok, npcID)
				}
				// The legacy SendSay always ships the full MSG_MessageChat struct
				// (issue #344): a shorter frame from an NPC speaker disconnects the
				// unmodified client.
				if string(body) != string(protocol.EncodeMessageChatBody(wantText)) {
					t.Fatalf("dialogue body = %q (len %d), want %q padded to %d bytes", body, len(body), wantText, protocol.MessageLength)
				}
				if h, _, ok := readMaybeHeader(t, watcher); ok {
					t.Fatalf("private dialogue leaked to watcher: %+v", h)
				}

				// Exercise the original player-attributed helper after NPC dialogue.
				whisperFrame(t, player, "cp", "")
				h, body, ok = readMaybeHeader(t, player)
				if !ok || h.Type != protocol.MsgMessageChat || h.ID != 1 || string(body) != string(protocol.EncodeMessageChatBody("Pontos Caos atual: 0")) {
					t.Fatalf("player reply = %+v %q received=%v", h, body, ok)
				}

				// Logout acknowledgement is a barrier after the authoritative save.
				send(t, player, protocol.MsgCharacterLogout, nil)
				expect(t, player, protocol.MsgCNFCharacterLogout)
				save, n := db.lastSavedChar()
				if n == 0 || !reflect.DeepEqual(save.Carry, wantCarry) || save.Clan != st.Clan {
					t.Fatalf("dialogue changed inventory/clan: saves=%d carry=%+v clan=%d", n, save.Carry, save.Clan)
				}
				cape, present := savedSlot(save.Equip, capeEquipSlot)
				if branch == "complete" {
					if !present || cape != (world.SavedItem{Slot: capeEquipSlot, Index: king.cape}) {
						t.Fatalf("completed cape changed: %+v present=%v", cape, present)
					}
				} else if present {
					t.Fatalf("dialogue granted a cape: %+v", cape)
				}
				if got := db.purchases.Load(); got != 0 {
					t.Fatalf("dialogue attempted %d purchases/balance changes", got)
				}
			})
		}
	}
}

// TestKingArchIdealStoneDialogue reproduces issue #344: an Arch ready to become
// Celestial (level 356+, Pedra Ideal carried or equipped) talks to the King and
// loses the connection. The King never creates a Celestial (the legacy entry is
// right-clicking the stone, _MSG_UseItem.cpp), so every reply must be a
// well-formed frame and the session must stay alive afterwards.
func TestKingArchIdealStoneDialogue(t *testing.T) {
	kings := []struct {
		name     string
		merchant uint8
		clan     uint8
		cape     int16
	}{
		{"Harabard legacy", 14, clanHekalotia, 543},
		{"Glantuar legacy", 15, clanAkelonia, 544},
		{"Harabard canonical", 111, clanHekalotia, 543},
	}
	for _, king := range kings {
		for _, level := range []int{356, 399} {
			for _, stone := range []string{"carry", "equip"} {
				for _, capeDone := range []bool{false, true} {
					for _, confirm := range []int32{0, 1} {
						name := fmt.Sprintf("%s/lv%d/%s/cape=%v/confirm=%d", king.name, level, stone, capeDone, confirm)
						t.Run(name, func(t *testing.T) {
							st := baseMortalState(level)
							st.ClassMaster = classMasterArch
							st.MortalLevel = 99
							st.ArchCrystalStage = 4
							switch stone {
							case "carry":
								st.Carry[0] = world.Item{Index: idealStoneItem}
							case "equip":
								st.Equip[idealStoneEquipSlot] = world.Item{Index: idealStoneItem}
								st.Equip[sephirotEquipSlot] = world.Item{Index: archSephirotMin}
							}
							if capeDone {
								st.Clan = king.clan
								st.Equip[capeEquipSlot] = world.Item{Index: king.cape}
							}
							db := &kingDialogueDB{
								fakeDB: newDB(),
								quote:  world.KingdomCapeQuote{Revision: 7, HekalotiaCost: 8, AkeloniaCost: 6},
							}
							db.loadResult = st
							tmpl := questNPCTemplate(king.name, king.merchant, 0, 0)
							tmpl[16] = king.clan
							addr, stop, npcID := startServerQuestNPC(t, db, tmpl)
							defer stop()
							player := enterWorld(t, addr)
							defer player.Close()
							drainRaw(t, player)

							send(t, player, protocol.MsgQuest, protocol.EncodeStandardParm2(int32(npcID), confirm))
							for {
								h, body, ok := readMaybeHeader(t, player)
								if !ok {
									break
								}
								if h.Type == protocol.MsgMessageChat && len(body) != protocol.MessageLength {
									t.Fatalf("King reply %+v has a %d-byte body, want sizeof(MSG_MessageChat.String)=%d", h, len(body), protocol.MessageLength)
								}
								if h.Type == protocol.MsgCNFCharacterLogout {
									t.Fatalf("King sent an Arch back to character selection: %+v", h)
								}
							}

							// The session is still in play: a slash command round-trips.
							whisperFrame(t, player, "cp", "")
							h, _, ok := readMaybeHeader(t, player)
							if !ok || h.Type != protocol.MsgMessageChat {
								t.Fatalf("session dead after King dialogue: %+v received=%v", h, ok)
							}
						})
					}
				}
			}
		}
	}
}
