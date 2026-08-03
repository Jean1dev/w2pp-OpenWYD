package content

import "testing"

func TestLoadCastleQuestsReal(t *testing.T) {
	quests, err := LoadCastleQuests(release(t, "Common", "Settings", "CastleQuest.txt"))
	if err != nil {
		t.Skipf("CastleQuest.txt unavailable: %v", err)
	}
	if len(quests) != 1 {
		t.Fatalf("quests = %d, want 1", len(quests))
	}
	q := quests[0]
	if q.MobInitial != 6058 || q.MobEnd != 6105 || q.Boss != [2]int{6105, 6105} {
		t.Fatalf("mob range/boss = %d..%d %v", q.MobInitial, q.MobEnd, q.Boss)
	}
	if len(q.Prize) != 1 || q.Prize[0].Index != 5338 || !q.PartyPrize || q.QuestTime != 300 {
		t.Fatalf("prize/party/time = %+v/%v/%d", q.Prize, q.PartyPrize, q.QuestTime)
	}
}
