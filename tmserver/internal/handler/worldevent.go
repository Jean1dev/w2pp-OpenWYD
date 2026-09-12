package handler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jeanluca/w2pp-openwyd/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/worldcfg"
)

const (
	worldEventPollPeriod   = 15
	worldEventFetchTimeout = 5 * time.Second

	eventSerialHi   = 62
	eventSerialLo   = 63
	eventSerialRand = 59

	maxWorldEventItemIndex = 32767
)

// ApplyWorldEventConfigBoot fetches the portal-managed world event config
// synchronously and applies it before the loop starts accepting players.
func (d *Dispatcher) ApplyWorldEventConfigBoot(w *world.World) {
	if d.worldEventSource == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), worldEventFetchTimeout)
	defer cancel()
	snap, err := d.worldEventSource.Snapshot(ctx)
	if err != nil {
		d.log.Warn("world event config boot load failed (will retry via poll)", "err", err)
		return
	}
	daLinha := d.expEvents
	d.applyWorldEventConfig(w, snap)
	avisaDivergencia(d.log, daLinha, d.expEvents)
	d.log.Info("world event config applied at boot", "version", snap.Version)
}

// avisaDivergencia names every exp switch the database overruled.
//
// The three switches exist twice: as a command-line option and as a column the
// panel writes. The database wins, which is what makes the panel useful — but it
// means a server deliberately started with -kefra-live silently loses it, and
// the symptom is half the experience with nothing in the log to explain it. So
// the log explains it.
func avisaDivergencia(log *slog.Logger, daLinha, doBanco level.ExpEvents) {
	if daLinha == doBanco {
		return
	}
	for _, s := range []struct {
		nome         string
		linha, banco bool
	}{
		{"double-exp", daLinha.DoubleMode, doBanco.DoubleMode},
		{"newbie-event", daLinha.NewbieEvent, doBanco.NewbieEvent},
		{"kefra-live", daLinha.KefraLive, doBanco.KefraLive},
	} {
		if s.linha != s.banco {
			log.Warn("the panel overruled a command-line exp switch",
				"switch", s.nome, "command_line", s.linha, "panel", s.banco)
		}
	}
}

// pollWorldEventConfig reloads portal-managed event settings when the config
// version changes. Called from the world tick; gRPC work runs off-loop.
func (d *Dispatcher) pollWorldEventConfig(w *world.World) {
	if d.worldEventSource == nil || d.worldEventPolling {
		return
	}
	d.worldEventPollTick++
	if d.worldEventPollTick%worldEventPollPeriod != 0 {
		return
	}
	known := d.worldEventVersion
	src := d.worldEventSource
	d.worldEventPolling = true
	w.GoDetached(func() func(*world.World) {
		ctx, cancel := context.WithTimeout(context.Background(), worldEventFetchTimeout)
		defer cancel()
		version, err := src.Version(ctx)
		if err != nil {
			return func(*world.World) {
				d.worldEventPolling = false
				d.log.Warn("world event config version poll failed", "err", err)
			}
		}
		if version == known {
			return func(*world.World) { d.worldEventPolling = false }
		}
		snap, err := src.Snapshot(ctx)
		if err != nil {
			return func(*world.World) {
				d.worldEventPolling = false
				d.log.Warn("world event config reload failed", "err", err)
			}
		}
		return func(w *world.World) {
			d.applyWorldEventConfig(w, snap)
			d.worldEventPolling = false
			d.log.Info("world event config reloaded", "version", snap.Version)
		}
	})
}

func (d *Dispatcher) applyWorldEventConfig(w *world.World, snap worldcfg.Snapshot) {
	ev := snap.Event
	d.expEvents.DoubleMode = ev.DoubleExpEnabled
	d.setNewbieEvent(w, ev.NewbieEventEnabled)
	// The daily Tower War has its own switch and hour (migration 0051). It used
	// to ride on the newbie event above, which also changes EXP and monster HP
	// for low levels.
	d.setTowerSchedule(ev.TowerWarEnabled, int(ev.TowerWarHour))
	// Lone bosses come back in hours (migration 0056, chefes.go). The wait is
	// taken at each death, so a change reaches the next boss to die, not the
	// ones already waiting.
	d.setChefeHoras(ev.BossRespawnHours)
	// No side effects to run, unlike the newbie event: KefraLive is one branch
	// in the reward pipeline and touches nothing that is already in the world.
	d.expEvents.KefraLive = ev.KefraLiveEnabled
	w.SetWorldEventConfig(world.EventConfig{
		Version: snap.Version, Enabled: ev.Enabled, ItemIndex: ev.ItemIndex, Rate: ev.Rate,
		StartIndex: ev.StartIndex, CurrentIndex: ev.CurrentIndex, EndIndex: ev.EndIndex,
		Indexed: ev.Indexed, NoticeEnabled: ev.NoticeEnabled,
	})
	d.worldEventVersion = snap.Version
}

func (d *Dispatcher) tryWorldEventDrop(w *world.World, reward *world.Entity, nivelMob int) {
	cfg := w.WorldEventConfig()
	if !worldEventDropActive(cfg) {
		return
	}
	if w.Rand().Intn(int(cfg.Rate)) != 0 {
		return
	}
	serial := cfg.CurrentIndex
	item := world.Item{Index: int16(cfg.ItemIndex)}
	if cfg.Indexed {
		item.Effects = [3]world.Effect{
			{Effect: eventSerialHi, Value: uint8(serial / 256)},
			{Effect: eventSerialLo, Value: uint8(serial)},
			{Effect: eventSerialRand, Value: uint8(w.Rand().Rand())},
		}
	}
	// The event item goes through the same drop roll as ordinary loot, in the
	// same place the legacy puts it: after the serial is stamped, before
	// delivery (MobKilled.cpp:2752).
	//
	// An indexed event item arrives with all three slots already written, and
	// the roll is gated on slot 0 being empty — so it skips the whole bonus
	// block and the serial survives. Only a plain (unindexed) event item gets
	// bonuses. The one thing that can still touch an indexed item is the tail
	// override, if its catalog row carries EF_SANC/EF_AMOUNT/EF_INCUBATE; that
	// is the legacy's behaviour too, and choosing such an item as an indexed
	// event prize would break its numbering there as well.
	// Bonus 0, and not the winner's DropBonus: the legacy passes a literal zero
	// for the event item (MobKilled.cpp:2752), so the prize is the same prize for
	// everybody. Handing it the killer's bonus would make a numbered event item
	// worth more for whoever happened to be wearing the right fairy.
	d.rolarBonusDrop(w, &item, nivelMob, 0)

	// putCarryItem, so a merging fairy piles this onto its own kind (carry.go).
	if d.putCarryItem(w, reward, item) < 0 {
		conn := -1
		if reward != nil {
			conn = reward.ID
		}
		d.log.Warn("world event drop lost: carry full", "conn", conn, "item", cfg.ItemIndex, "serial", serial)
		return
	}
	if cfg.NoticeEnabled {
		noticeSerial := int32(0)
		if cfg.Indexed {
			noticeSerial = serial
		}
		d.broadcastWorldEventNotice(w, reward, item, noticeSerial)
	}
	cfg.CurrentIndex++
	w.SetWorldEventConfig(cfg)
	d.persistWorldEventProgress(w, cfg)
}

func worldEventDropActive(cfg world.EventConfig) bool {
	return cfg.Enabled &&
		cfg.ItemIndex > 0 &&
		cfg.ItemIndex <= maxWorldEventItemIndex &&
		cfg.Rate > 0 &&
		cfg.StartIndex > 0 &&
		cfg.CurrentIndex >= cfg.StartIndex &&
		cfg.CurrentIndex < cfg.EndIndex
}

func (d *Dispatcher) persistWorldEventProgress(w *world.World, cfg world.EventConfig) {
	if d.worldEventSource == nil {
		return
	}
	src := d.worldEventSource
	version := cfg.Version
	current := cfg.CurrentIndex
	w.GoDetached(func() func(*world.World) {
		ctx, cancel := context.WithTimeout(context.Background(), worldEventFetchTimeout)
		defer cancel()
		applied, err := src.UpdateProgress(ctx, version, current)
		return func(*world.World) {
			if err != nil {
				d.log.Warn("world event progress update failed", "version", version, "current_index", current, "err", err)
				return
			}
			if !applied {
				d.log.Debug("world event progress update skipped for stale version", "version", version, "current_index", current)
			}
		}
	})
}

func (d *Dispatcher) broadcastWorldEventNotice(w *world.World, reward *world.Entity, item world.Item, serial int32) {
	name := "alguem"
	if reward != nil && reward.Name != "" {
		name = reward.Name
	}
	msg := fmt.Sprintf("[Evento] %s recebeu item %d", name, item.Index)
	if serial > 0 {
		msg = fmt.Sprintf("%s #%d", msg, serial)
	}
	payload := protocol.EncodeMessageChatBody(msg)
	w.ForEachPlaying(-1, func(s *world.Session, _ *world.Entity) {
		w.SendTo(s, protocol.Header{Type: protocol.MsgMessageChat, ID: 0}, payload)
	})
}
