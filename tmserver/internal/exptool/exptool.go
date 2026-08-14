// Package exptool restamps the kill-reward Exp field of the Release npc
// templates with the balanced level curve (level.MobExpForLevel) — the Go
// heir of the legacy ExpTool (Source/Code/ExpTool/main.cpp:128), built for
// issue #43 (shipped templates mixed Exp=0 with garbage values).
package exptool

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
)

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
	// StampedVariant counts how many of Stamped were in one of the legacy
	// 756/920-byte layouts (data-formats.md §1.4.1). Those store Exp as a 32-bit
	// field at a different offset, so savefmt.PutMobExp routes the write; the
	// record is patched in place and the file keeps its size.
	StampedVariant int
	// SkippedNonTemplate counts files that are not a STRUCT_MOB in any layout.
	SkippedNonTemplate int
}

// Stamp walks dir (one raw STRUCT_MOB per file) and rewrites the Exp field of
// every real monster (Merchant==0, Level>=1) with level.MobExpForLevel,
// leaving every other byte untouched. NPC/merchant and level-0 templates are
// skipped. All three template layouts are restamped — the mixed content tree
// would otherwise keep 222 monsters on their shipped Exp while the boot log
// told operators to run this tool (issue #244).
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
		// DecodeMobAny (not protocol.ParseMobBasics) because that one indexes the
		// canonical offsets and would read garbage out of a legacy record.
		mob, version, derr := savefmt.DecodeMobAny(raw)
		if derr != nil {
			res.SkippedNonTemplate++
			continue
		}
		// CurrentScore.Merchant is the flag the tmServer honours, not the
		// top-level Mob.Merchant — same field the NPC importer reads.
		lvl := mob.CurrentScore.Level
		if mob.CurrentScore.Merchant != 0 || lvl < 1 {
			res.Skipped++
			continue
		}
		newExp := level.MobExpForLevel(lvl)
		res.Stamped = append(res.Stamped, Entry{
			File: de.Name(), Name: cstr(mob.Name[:]), Level: lvl, OldExp: mob.Exp, NewExp: newExp,
		})
		if version != savefmt.MobVersionCurrent {
			res.StampedVariant++
		}
		if dryRun || newExp == mob.Exp {
			continue
		}
		if err := savefmt.PutMobExp(raw, newExp); err != nil {
			return res, fmt.Errorf("stamping %s: %w", de.Name(), err)
		}
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

// cstr trims a fixed-width name field at the first NUL.
func cstr(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}
