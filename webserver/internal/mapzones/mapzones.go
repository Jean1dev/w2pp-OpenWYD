// Package mapzones labels the fixed city zones for the moderator UI's map_id
// picker. NPCDefinition.MapID is stored but not consumed by the world today
// (npc-editing-plan.md §9.2: the world runs a single grid, so spawn position
// comes from pos_x/pos_y alone) — this list exists purely so the form can show
// a name instead of a bare int, confirmed to mirror the city order the game
// world already uses for player teleports/spawn zones.
package mapzones

// Zone is one labeled map_id value.
type Zone struct {
	ID   int32
	Name string
}

// All is the fixed 5-zone table, in the same order as
// tmserver/internal/world/city.go's cities array (index = city id there). If
// that array ever changes, update this list to match — there is no shared Go
// package between tmserver and webserver for it, since tmserver's table only
// carries spawn/limit coordinates, not names, and has no reason to depend on
// this webserver-only label list.
var All = []Zone{
	{ID: 0, Name: "Armia"},
	{ID: 1, Name: "Azran"},
	{ID: 2, Name: "Erion"},
	{ID: 3, Name: "Nippleheim"},
	{ID: 4, Name: "Noatum"},
}
