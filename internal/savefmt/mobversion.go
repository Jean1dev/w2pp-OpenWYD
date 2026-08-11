package savefmt

// MobVersion identifies a standalone STRUCT_MOB template layout by its byte
// size, the same way DetectVersion routes account files (version.go).
//
// Release/TMsrv/run/npc/ mixes three sizes (data-formats.md §1.4.1): 1769 files
// at the canonical 816, 207 at 756 and 15 at 920. The 756-byte form is a
// LEGITIMATE older STRUCT_MOB — 32-bit Exp, no Quest/Magic/GuildLevel and a
// compact 28-byte STRUCT_SCORE — not a truncated 816 record. The 920-byte form
// is that same 756-byte record followed by 164 bytes of uninitialized heap
// garbage from whichever legacy tool wrote it, so one decoder serves both.
type MobVersion int

// Known STRUCT_MOB template versions.
const (
	MobVersionUnknown MobVersion = iota
	// MobVersionLegacy756 is the pre-expansion layout (mobLegacy756 offsets).
	MobVersionLegacy756
	// MobVersionLegacy756Padded is MobVersionLegacy756 plus trailing junk.
	MobVersionLegacy756Padded
	// MobVersionCurrent is the 816-byte layout modelled by decodeMob/encodeMob.
	MobVersionCurrent
)

// String renders the version for logs and inventory reports.
func (v MobVersion) String() string {
	switch v {
	case MobVersionLegacy756:
		return "legacy-756"
	case MobVersionLegacy756Padded:
		return "legacy-756+pad920"
	case MobVersionCurrent:
		return "current-816"
	default:
		return "unknown"
	}
}

// DetectMobVersion maps a template file size to its layout version.
func DetectMobVersion(size int) MobVersion {
	switch size {
	case MobSize:
		return MobVersionCurrent
	case MobSizeLegacy756:
		return MobVersionLegacy756
	case MobSizeLegacy756Padded:
		return MobVersionLegacy756Padded
	default:
		return MobVersionUnknown
	}
}
