// Package exptool restamps the kill-reward Exp field of the Release npc
// templates with the balanced level curve (level.MobExpForLevel) — the Go
// heir of the legacy ExpTool (Source/Code/ExpTool/main.cpp:128), built for
// issue #43 (shipped templates mixed Exp=0 with garbage values).
package exptool

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// expOffset pins the Exp field within the canonical raw STRUCT_MOB (int64 at
// offset 32 — protocol.ParseMobBasics reads the same spot); the layout is
// routed by savefmt.DetectMobVersion, like the loaders.
const expOffset = 32

// Entry reports one restamped template.
type Entry struct {
	File   string
	Name   string
	Level  int32
	OldExp int64
	NewExp int64
}

// Result summarises a Stamp run.
type Result struct {
	Stamped []Entry
	Skipped int // directories, NPCs/merchants and level-0 templates
	// SkippedVariant counts legacy 756/920-byte templates (data-formats.md
	// §1.4.1). They are deliberately left alone: their Exp is a 32-bit field at
	// a different offset, and restamping in place would have to rewrite the
	// whole record, changing a shipped asset's size.
	SkippedVariant int
	// SkippedNonTemplate counts files that are not a STRUCT_MOB in any layout.
	SkippedNonTemplate int
}

// Stamp walks dir (one raw STRUCT_MOB per file) and rewrites the Exp field of
// every real monster (Merchant==0, Level>=1) with level.MobExpForLevel,
// leaving every other byte untouched. NPC/merchant and level-0 templates are
// skipped, and so are the legacy 756/920-byte layouts — see Result.
// With dryRun the report is produced but nothing is written.
func Stamp(dir string, dryRun bool) (Result, error) {
	var res Result
	entries, err := os.ReadDir(dir)
	if err != nil {
		return res, fmt.Errorf("reading template dir: %w", err)
	}
	for _, de := range entries {
		if de.IsDir() {
			res.Skipped++
			continue
		}
		path := filepath.Join(dir, de.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return res, fmt.Errorf("reading %s: %w", de.Name(), err)
		}
		switch savefmt.DetectMobVersion(len(raw)) {
		case savefmt.MobVersionCurrent:
			// restampable in place
		case savefmt.MobVersionLegacy756, savefmt.MobVersionLegacy756Padded:
			res.SkippedVariant++
			continue
		default:
			res.SkippedNonTemplate++
			continue
		}
		b := protocol.ParseMobBasics(raw)
		if b.Merchant != 0 || b.Level < 1 {
			res.Skipped++
			continue
		}
		newExp := level.MobExpForLevel(b.Level)
		res.Stamped = append(res.Stamped, Entry{
			File: de.Name(), Name: b.Name, Level: b.Level, OldExp: b.Exp, NewExp: newExp,
		})
		if dryRun || newExp == b.Exp {
			continue
		}
		binary.LittleEndian.PutUint64(raw[expOffset:], uint64(newExp))
		info, err := de.Info()
		if err != nil {
			return res, fmt.Errorf("stat %s: %w", de.Name(), err)
		}
		if err := os.WriteFile(path, raw, info.Mode().Perm()); err != nil {
			return res, fmt.Errorf("writing %s: %w", de.Name(), err)
		}
	}
	sort.Slice(res.Stamped, func(i, j int) bool {
		a, b := res.Stamped[i], res.Stamped[j]
		if a.Level != b.Level {
			return a.Level < b.Level
		}
		return a.File < b.File
	})
	return res, nil
}
