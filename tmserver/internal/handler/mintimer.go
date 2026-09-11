package handler

import (
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/spawnrate"
)

// minTimerTicks is one pass of the legacy "minute" timer, in world ticks (our
// tick is 1 s, world/tick.go).
//
// FIDELIDADE AO LEGADO (restaurada): despite its name, TIMER_MIN fires every
// 12000 ms (Server.cpp:4087, next to TIMER_SEC's 500), and ProcessMinTimer
// counts ITS OWN passes (ProcessSecMinTimer.cpp:2523). So NPCGener.txt's
// MinuteGenerate was never minutes: a block with MinuteGenerate 10 refills every
// 10 passes, 120 s (ProcessSecMinTimer.cpp:2723-2733). The rewrite had read the
// name literally and run the generators and the weather on a 60-tick minute,
// which made every timed spawn block refill five times slower than the game it
// ports.
const minTimerTicks = int(spawnrate.MinTimerPass / time.Second)

// minutoTicks is a wall-clock minute in world ticks. The castle, the kingdom
// throne rooms and the tower war still step on it. Whether each of them ports a
// count of ProcessMinTimer passes (and so has the same five-fold defect) or only
// polls the time of day is an open audit, answered one by one against the
// legacy before anything moves: castle.go tickCastle, kingdom.go tickKingdomRvR,
// towerwar.go tickTowerWar.
const minutoTicks = 60
