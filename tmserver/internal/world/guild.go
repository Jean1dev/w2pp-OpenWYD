package world

// GuildInfo is the minimal guild metadata not modeled on Entity itself: the
// name and fame score, keyed by guild id (issue #131). In-memory, filled from
// dbServer at boot (handler/guild_state.go), by /create as each guild is made,
// and by the /gm guildname|guildfame admin commands, which mirror the legacy
// GM-only "+guildfame set" tool (Source/Comandos GM.txt).
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

// GuildNameTaken reports whether a guild this process knows already has exactly
// this name — the same rule as the guild table's UNIQUE constraint, checked here
// so /create can say so instead of losing the answer inside dbServer's ok=false.
// Loop-only.
func (w *World) GuildNameTaken(name string) bool {
	for _, gi := range w.guilds {
		if gi.Name == name {
			return true
		}
	}
	return false
}
