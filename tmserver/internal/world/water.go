package world

// IsWaterGenerator identifies the event-owned NPCGener blocks from Basedef.h.
// They must never populate at startup or enter either automatic respawn path.
func IsWaterGenerator(index int) bool {
	return index >= 10 && index <= 21 || index >= 171 && index <= 194
}
