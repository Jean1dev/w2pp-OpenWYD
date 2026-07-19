package world

// GuildInfo is the minimal guild metadata not modeled on Entity itself: the
// name and fame score, keyed by guild id (issue #131). In-memory only — there
// is no guild-creation flow yet to persist against (guildRelay, chat.go
// leaveGuild); populated today only via the /gm guildname|guildfame admin
// commands, mirroring the legacy GM-only "+guildfame set" tool
// (Source/Comandos GM.txt).
type GuildInfo struct {
	Name string
	Fame int32
}

// GuildInfo returns the registered name/fame for a guild id, or false if
// nothing has been registered for it yet. Loop-only.
func (w *World) GuildInfo(id uint16) (GuildInfo, bool) {
	gi, ok := w.guilds[id]
	return gi, ok
}

// SetGuildName sets (or creates) a guild's registered name. Loop-only.
func (w *World) SetGuildName(id uint16, name string) {
	gi := w.guilds[id]
	gi.Name = name
	w.guilds[id] = gi
}

// SetGuildFame sets (or creates) a guild's registered fame. Loop-only.
func (w *World) SetGuildFame(id uint16, fame int32) {
	gi := w.guilds[id]
	gi.Fame = fame
	w.guilds[id] = gi
}
