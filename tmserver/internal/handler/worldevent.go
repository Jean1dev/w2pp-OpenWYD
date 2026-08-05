package handler

import (
	"context"
	"fmt"
	"time"

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
	d.applyWorldEventConfig(w, snap)
	d.log.Info("world event config applied at boot", "version", snap.Version)
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
	w.SetWorldEventConfig(world.EventConfig{
		Version: snap.Version, Enabled: ev.Enabled, ItemIndex: ev.ItemIndex, Rate: ev.Rate,
		StartIndex: ev.StartIndex, CurrentIndex: ev.CurrentIndex, EndIndex: ev.EndIndex,
		Indexed: ev.Indexed, NoticeEnabled: ev.NoticeEnabled,
	})
	d.worldEventVersion = snap.Version
}

func (d *Dispatcher) tryWorldEventDrop(w *world.World, reward *world.Entity) {
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
	slot := firstEmptyAccessibleCarry(reward)
	if slot < 0 {
		conn := -1
		if reward != nil {
			conn = reward.ID
		}
		d.log.Warn("world event drop lost: carry full", "conn", conn, "item", cfg.ItemIndex, "serial", serial)
		return
	}
	reward.Carry[slot] = item
	if s := w.Session(reward.ID); s != nil {
		d.sendSlot(w, s, world.ItemPlaceCarry, slot, item)
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
