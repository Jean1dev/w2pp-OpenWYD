package handler

import (
	"context"
	"strings"
	"time"

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/world"
)

// Guild costs and ranks (lote2-party-guilda-guerra.md and
// _MSG_MessageWhisper.cpp guild command blocks).
const (
	guildInviteCost  = 4_000_000
	guildSpecialCost = 100_000_000
	guildCreateCost  = 100_000_000
	guildSubCost     = 100_000_000
	guildLeaderLevel = 9
	guildNameMaxLen  = 16
)

// inviteGuild handles _MSG_InviteGuild (0x03D5, MSG_STANDARDPARM2:
// Parm1=TargetID, Parm2=InviteType): add a same-clan, guildless player to the
// inviter's guild for a gold cost.
func (d *Dispatcher) inviteGuild(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	e := w.Entity(s.Conn)
	if e == nil || s.Mode != world.UserPlay {
		return
	}
	p1, p2, ok := protocol.StandardParm2(payload)
	if !ok {
		return
	}
	target, inviteType := int(p1), int(p2)
	if target <= 0 || target >= world.MaxUser || inviteType < 0 || inviteType >= 4 {
		return
	}
	if e.Guild == 0 || e.GuildLevel == 0 {
		return
	}
	if inviteType != 0 && e.GuildLevel != guildLeaderLevel {
		return
	}
	if time.Now().Weekday() == time.Sunday {
		return
	}
	other, te := w.Session(target), w.Entity(target)
	if other == nil || other.Mode != world.UserPlay || te == nil {
		return
	}
	if te.Guild != 0 || te.Clan != e.Clan {
		return
	}
	cost := int32(guildInviteCost)
	if inviteType != 0 {
		cost = guildSpecialCost
	}
	if e.Coin < cost {
		d.notify(w, s, NoticeNotEnoughCoin)
		return
	}

	e.Coin -= cost
	te.Guild = e.Guild
	te.GuildLevel = 0
	d.refreshGuildTag(w, target)
	d.sendEtc(w, s, e)
	w.Send(other, protocol.MsgMessagePanel, nil) // welcome (payload UNVERIFIED)
	w.SaveCharacterAsync(s)
	w.SaveCharacterAsync(other)
	d.persistGuildMember(w, s, other, te)
}

func (d *Dispatcher) createGuild(w *world.World, s *world.Session, args []byte) {
	e := w.Entity(s.Conn)
	if e == nil || s.Mode != world.UserPlay {
		return
	}
	name := strings.TrimSpace(cstr(args))
	if !validGuildName(name) || e.Guild != 0 || e.Coin < guildCreateCost || (e.Clan != 7 && e.Clan != 8) || e.Citizen == 0 {
		if e.Coin < guildCreateCost {
			d.notify(w, s, NoticeNotEnoughCoin)
		}
		return
	}
	if time.Now().Weekday() == time.Sunday {
		return
	}
	accountID, slot, charName, clan, citizen, serverIndex := s.AccountID, s.Slot, e.Name, e.Clan, e.Citizen, d.serverIndex
	p := w.Persistence()
	s.Mode = world.UserWaitDB
	w.Go(s, func() func(*world.World, *world.Session) {
		guild, ok, err := p.CreateGuild(context.Background(), accountID, slot, charName, name, clan, citizen, serverIndex, guildCreateCost)
		return func(w *world.World, s *world.Session) {
			if s.Mode == world.UserWaitDB {
				s.Mode = world.UserPlay
			}
			e := w.Entity(s.Conn)
			if e == nil {
				return
			}
			if err != nil {
				d.log.Warn("create guild failed", "conn", s.Conn, "guild", name, "err", err)
				d.notify(w, s, NoticeDBError)
				return
			}
			if !ok || guild.ID == 0 || e.Guild != 0 {
				return
			}
			e.Coin -= guildCreateCost
			e.Guild = guild.ID
			e.GuildLevel = guildLeaderLevel
			d.sendEtc(w, s, e)
			d.refreshGuildTag(w, s.Conn)
			w.SaveCharacterAsync(s)
			w.Send(s, protocol.MsgMessagePanel, nil)
		}
	})
}

func validGuildName(name string) bool {
	if name == "" || len(name) > guildNameMaxLen {
		return false
	}
	return !strings.ContainsRune(name, 0)
}

func (d *Dispatcher) subcreate(w *world.World, s *world.Session, args []byte) {
	e := w.Entity(s.Conn)
	if e == nil || s.Mode != world.UserPlay || e.Guild == 0 || e.GuildLevel != guildLeaderLevel || e.Coin < guildSubCost {
		if e != nil && e.Coin < guildSubCost {
			d.notify(w, s, NoticeNotEnoughCoin)
		}
		return
	}
	fields := strings.Fields(cstr(args))
	if len(fields) < 1 {
		return
	}
	targetSession, target := w.SessionByName(fields[0])
	if targetSession == nil || target == nil || target.ID == s.Conn {
		d.notify(w, s, NoticeNotConnected)
		return
	}
	if targetSession.Mode != world.UserPlay {
		return
	}
	if target.Guild != e.Guild || target.GuildLevel != 0 {
		return
	}
	p := w.Persistence()
	guildID := e.Guild
	leaderConn, targetConn := s.Conn, target.ID
	leaderSession, memberSession := s, targetSession
	leaderAccountID, leaderSlot := s.AccountID, s.Slot
	accountID, slot, targetName := targetSession.AccountID, targetSession.Slot, target.Name
	s.Mode = world.UserWaitDB
	targetSession.Mode = world.UserWaitDB
	w.GoDetached(func() func(*world.World) {
		level, ok, err := p.PromoteGuildMember(context.Background(), guildID, leaderAccountID, leaderSlot, accountID, slot, guildSubCost)
		return func(w *world.World) {
			ls := w.Session(leaderConn)
			if ls != leaderSession {
				ls = nil
			}
			ts := w.Session(targetConn)
			if ts != memberSession {
				ts = nil
			}
			if ls != nil && ls.Mode == world.UserWaitDB {
				ls.Mode = world.UserPlay
			}
			if ts != nil && ts.Mode == world.UserWaitDB {
				ts.Mode = world.UserPlay
			}
			if err != nil {
				d.log.Warn("subcreate failed", "conn", leaderConn, "target", targetName, "err", err)
				if ls != nil {
					d.notify(w, ls, NoticeDBError)
				}
				return
			}
			if !ok || level < 6 || level > 8 {
				return
			}
			if ls != nil {
				if le := w.Entity(leaderConn); le != nil {
					le.Coin -= guildSubCost
					d.sendEtc(w, ls, le)
					w.SaveCharacterAsync(ls)
				}
			}
			if ts != nil {
				if te := w.Entity(targetConn); te != nil && te.Guild == guildID {
					te.GuildLevel = level
					d.refreshGuildTag(w, te.ID)
					w.SaveCharacterAsync(ts)
				}
			}
		}
	})
}

func (d *Dispatcher) handoverGuild(w *world.World, s *world.Session, args []byte) {
	e := w.Entity(s.Conn)
	if e == nil || s.Mode != world.UserPlay || e.Guild == 0 || e.GuildLevel != guildLeaderLevel {
		return
	}
	name := strings.TrimSpace(cstr(args))
	targetSession, target := w.SessionByName(name)
	if targetSession == nil || target == nil || target.ID == s.Conn {
		d.notify(w, s, NoticeNotConnected)
		return
	}
	if targetSession.Mode != world.UserPlay {
		return
	}
	if target.Guild != e.Guild {
		return
	}
	guildID := e.Guild
	oldAccountID, oldSlot := s.AccountID, s.Slot
	newAccountID, newSlot := targetSession.AccountID, targetSession.Slot
	leaderConn, targetConn := s.Conn, target.ID
	leaderSession, memberSession := s, targetSession
	p := w.Persistence()
	s.Mode = world.UserWaitDB
	targetSession.Mode = world.UserWaitDB
	w.GoDetached(func() func(*world.World) {
		err := p.TransferGuildLeader(context.Background(), guildID, oldAccountID, oldSlot, newAccountID, newSlot)
		return func(w *world.World) {
			ls := w.Session(leaderConn)
			if ls != leaderSession {
				ls = nil
			}
			ts := w.Session(targetConn)
			if ts != memberSession {
				ts = nil
			}
			if ls != nil && ls.Mode == world.UserWaitDB {
				ls.Mode = world.UserPlay
			}
			if ts != nil && ts.Mode == world.UserWaitDB {
				ts.Mode = world.UserPlay
			}
			if err != nil {
				d.log.Warn("handover guild failed", "conn", leaderConn, "guild", guildID, "err", err)
				if ls != nil {
					d.notify(w, ls, NoticeDBError)
				}
				return
			}
			if ls != nil {
				if le := w.Entity(leaderConn); le != nil && le.Guild == guildID {
					le.GuildLevel = 0
					d.refreshGuildTag(w, leaderConn)
					w.SaveCharacterAsync(ls)
				}
			}
			if ts != nil {
				if te := w.Entity(targetConn); te != nil && te.Guild == guildID {
					te.GuildLevel = guildLeaderLevel
					d.refreshGuildTag(w, targetConn)
					w.SaveCharacterAsync(ts)
				}
			}
		}
	})
}

// leaveGuild handles /sair and /abandonar. /expulsar <char> is handled by
// kickGuild; bare /expulsar keeps the legacy self-leave behavior.
func (d *Dispatcher) leaveGuild(w *world.World, s *world.Session) {
	e := w.Entity(s.Conn)
	if e == nil || e.Guild == 0 {
		return
	}
	e.Guild = 0
	e.GuildLevel = 0
	d.refreshGuildTag(w, s.Conn)
	w.SaveCharacterAsync(s)
	d.persistLeaveGuild(w, s)
}

func (d *Dispatcher) kickGuild(w *world.World, s *world.Session, args []byte) {
	e := w.Entity(s.Conn)
	if e == nil || e.Guild == 0 || e.GuildLevel == 0 {
		return
	}
	name := strings.TrimSpace(cstr(args))
	if name == "" {
		d.leaveGuild(w, s)
		return
	}
	targetSession, target := w.SessionByName(name)
	if targetSession == nil || target == nil {
		d.notify(w, s, NoticeNotConnected)
		return
	}
	if target.Guild != e.Guild || target.ID == s.Conn || e.GuildLevel <= target.GuildLevel {
		return
	}
	target.Guild = 0
	target.GuildLevel = 0
	d.refreshGuildTag(w, target.ID)
	w.SaveCharacterAsync(targetSession)
	d.persistLeaveGuild(w, targetSession)
	w.Send(targetSession, protocol.MsgMessagePanel, nil)
}

func (d *Dispatcher) summonGuild(w *world.World, s *world.Session) {
	e := w.Entity(s.Conn)
	if e == nil || e.Guild == 0 || e.GuildLevel == 0 || world.Village(e.X, e.Y) < 0 {
		return
	}
	count := 0
	w.ForEachPlaying(s.Conn, func(ts *world.Session, te *world.Entity) {
		if count >= 350 || te.Guild != e.Guild {
			return
		}
		x, y := e.X+int16(w.Rand().Intn(3)), e.Y+int16(w.Rand().Intn(3))
		if ex, ey, ok := w.EmptyCellNear(x, y); ok {
			x, y = ex, ey
		}
		d.doTeleport(w, ts, x, y)
		count++
	})
}

func (d *Dispatcher) guildTax(w *world.World, s *world.Session, text string) bool {
	e := w.Entity(s.Conn)
	if e == nil || e.Guild == 0 || e.GuildLevel != guildLeaderLevel {
		return true
	}
	fields := strings.Fields(text)
	if len(fields) != 2 {
		return true
	}
	tax, ok := parseSmallInt(fields[1])
	if !ok || tax < 0 || tax > 30 {
		return true
	}
	now := time.Now()
	for i := range d.guildZones {
		z := &d.guildZones[i]
		if z.ChargeGuild != e.Guild || sameDate(d.taxChangedAt[i], now) {
			continue
		}
		z.CityTax = uint8(tax)
		d.taxChangedAt[i] = now
		d.persistGuildZone(w, s, *z)
		break
	}
	return true
}

// sameDate reports whether a and b fall on the same calendar day, used to
// gate guildtax to one change per day (lote2-chat.md: TaxChanged[i]).
func sameDate(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func parseSmallInt(s string) (int, bool) {
	n := 0
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

// guildAlly handles _MSG_GuildAlly (0x0E12).
func (d *Dispatcher) guildAlly(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	d.guildRelay(w, s, payload, world.GuildRelationAlly)
}

// war handles _MSG_War (0x0E0E).
func (d *Dispatcher) war(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	d.guildRelay(w, s, payload, world.GuildRelationWar)
}

func (d *Dispatcher) guildRelay(w *world.World, s *world.Session, payload []byte, kind world.GuildRelationKind) {
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	guild, target, ok := protocol.StandardParm2(payload)
	if !ok || guild <= 0 || guild >= 65536 || target < 0 || target >= 65536 {
		return
	}
	if e.Guild != uint16(guild) || e.GuildLevel != guildLeaderLevel {
		return
	}
	p := w.Persistence()
	guildID, targetID := uint16(guild), uint16(target)
	w.Go(s, func() func(*world.World, *world.Session) {
		err := p.SetGuildRelation(context.Background(), guildID, targetID, kind)
		return func(w *world.World, s *world.Session) {
			if err != nil {
				d.log.Warn("guild relation failed", "conn", s.Conn, "guild", guildID, "target", targetID, "kind", kind, "err", err)
				return
			}
			d.applyGuildRelation(guildID, targetID, kind)
			d.sendWarInfoToGuild(w, guildID)
		}
	})
}

// challange handles _MSG_Challange (0x028E): status/collection for a guild zone.
func (d *Dispatcher) challange(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	zoneParm, ok := protocol.StandardParm(payload)
	if !ok || zoneParm < 0 || int(zoneParm) >= len(d.guildZones) {
		return
	}
	e := w.Entity(s.Conn)
	if e == nil {
		return
	}
	z := &d.guildZones[zoneParm]
	if z.ChargeGuild == e.Guild && e.GuildLevel == guildLeaderLevel && z.TaxVault > 0 {
		coin := z.TaxVault
		if coin > 2_000_000_000-int64(e.Coin) {
			coin = 2_000_000_000 - int64(e.Coin)
		}
		if coin > 0 {
			e.Coin += int32(coin)
			z.TaxVault -= coin
			d.sendEtc(w, s, e)
			w.SaveCharacterAsync(s)
			d.persistGuildZone(w, s, *z)
		}
	}
}

// challangeConfirm handles _MSG_ChallangeConfirm (0x028F). We model the bid as
// Parm1=zone, Parm2=coin; the exact NPC target variant remains capture-pending.
func (d *Dispatcher) challangeConfirm(w *world.World, s *world.Session, _ protocol.Header, payload []byte) {
	zoneParm, coinParm, ok := protocol.StandardParm2(payload)
	if !ok || zoneParm < 0 || int(zoneParm) >= len(d.guildZones) || coinParm <= 0 {
		return
	}
	e := w.Entity(s.Conn)
	if e == nil || e.Guild == 0 || e.GuildLevel != guildLeaderLevel || e.Coin < coinParm {
		return
	}
	z := &d.guildZones[zoneParm]
	if z.ChargeGuild == e.Guild || int64(coinParm) <= z.ChallengeMoney {
		return
	}
	e.Coin -= coinParm
	z.ChallengeGuild = e.Guild
	z.ChallengeMoney = int64(coinParm)
	d.sendEtc(w, s, e)
	w.SaveCharacterAsync(s)
	d.persistGuildZone(w, s, *z)
}

func (d *Dispatcher) refreshGuildTag(w *world.World, id int) {
	e := w.Entity(id)
	if e == nil {
		return
	}
	body := protocol.EncodeCreateMobBody(createMobFrom(e, 0))
	w.ForEachInView(id, func(vs *world.Session, _ *world.Entity) {
		w.SendTo(vs, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene}, body)
	})
	if s := w.Session(id); s != nil && s.Mode == world.UserPlay {
		w.SendTo(s, protocol.Header{Type: protocol.MsgCreateMob, ID: protocol.IDScene}, body)
	}
}

func (d *Dispatcher) persistGuildMember(w *world.World, actor, member *world.Session, e *world.Entity) {
	if member == nil || e == nil {
		return
	}
	accountID, slot, name, guildID, level := member.AccountID, member.Slot, e.Name, e.Guild, e.GuildLevel
	p := w.Persistence()
	w.Go(actor, func() func(*world.World, *world.Session) {
		err := p.SetGuildMember(context.Background(), accountID, slot, name, guildID, level)
		return func(_ *world.World, _ *world.Session) {
			if err != nil {
				d.log.Warn("persist guild member failed", "account", accountID, "slot", slot, "guild", guildID, "err", err)
			}
		}
	})
}

func (d *Dispatcher) persistLeaveGuild(w *world.World, s *world.Session) {
	accountID, slot := s.AccountID, s.Slot
	p := w.Persistence()
	w.Go(s, func() func(*world.World, *world.Session) {
		err := p.LeaveGuild(context.Background(), accountID, slot)
		return func(_ *world.World, _ *world.Session) {
			if err != nil {
				d.log.Warn("persist leave guild failed", "account", accountID, "slot", slot, "err", err)
			}
		}
	})
}

func (d *Dispatcher) persistGuildZone(w *world.World, s *world.Session, z world.GuildZone) {
	p := w.Persistence()
	w.Go(s, func() func(*world.World, *world.Session) {
		err := p.SaveGuildZone(context.Background(), z)
		return func(_ *world.World, _ *world.Session) {
			if err != nil {
				d.log.Warn("persist guild zone failed", "zone", z.Zone, "err", err)
			}
		}
	})
}

func (d *Dispatcher) sendWarInfoToGuild(w *world.World, guildID uint16) {
	warTarget := int32(d.guildWars[guildID])
	allyTarget := int32(d.guildAllies[guildID])
	body := protocol.EncodeStandardParm3(warTarget, 0, allyTarget)
	w.ForEachPlaying(-1, func(s *world.Session, e *world.Entity) {
		if e.Guild == guildID {
			w.SendTo(s, protocol.Header{Type: protocol.MsgSendWarInfo, ID: protocol.IDScene}, body)
		}
	})
}
