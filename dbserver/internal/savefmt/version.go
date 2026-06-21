package savefmt

import "strings"

// Version identifies an account-file layout by its byte size. The original DBSrv
// accepts (fsize>=7500 && fsize<=7600) || fsize==sizeof(STRUCT_ACCOUNTFILE)
// (DBSrv/CRanking.cpp:175); the 4294-byte form is an even older pre-expansion
// layout seen in account/A/antonio (data-formats.md §1.2).
type Version int

// Known account-file versions.
const (
	VersionUnknown      Version = iota
	VersionLegacy4294           // 4294 B — pre-expansion; layout UNVERIFIED
	VersionIntermediate         // 7500–7600 B — layout UNVERIFIED
	VersionCurrent              // 7952 B — fully modelled (Decode/Encode)
)

// String renders the version for logs.
func (v Version) String() string {
	switch v {
	case VersionLegacy4294:
		return "legacy-4294"
	case VersionIntermediate:
		return "intermediate-7500..7600"
	case VersionCurrent:
		return "current-7952"
	default:
		return "unknown"
	}
}

// Modelled reports whether this stack can fully decode the version. Only the
// current 7952-byte format is modelled; the others are detected but their field
// layout is UNVERIFIED and must be reversed from a reference build/capture.
func (v Version) Modelled() bool { return v == VersionCurrent }

// DetectVersion maps a file size to its layout version (data-formats.md §1.2).
func DetectVersion(size int) Version {
	switch {
	case size == AccountFileSize:
		return VersionCurrent
	case size >= 7500 && size <= 7600:
		return VersionIntermediate
	case size == 4294:
		return VersionLegacy4294
	default:
		return VersionUnknown
	}
}

// AccountNameAt0 reads the AccountName field at offset 0 — the one field stable
// across all known versions (STRUCT_ACCOUNTINFO begins the file in every layout).
// It trims trailing spaces and NULs (legacy files space-pad; see the antonio
// sample). Returns "" if b is too short.
func AccountNameAt0(b []byte) string {
	if len(b) < 16 {
		return ""
	}
	return strings.TrimRight(string(b[0:16]), " \x00")
}
