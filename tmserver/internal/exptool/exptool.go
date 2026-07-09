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

	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/content"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/level"
	"github.com/jeanluca/w2pp-openwyd/tmserver/internal/protocol"
)

// expOffset pins the Exp field within the raw STRUCT_MOB (int64 at offset 32
// — protocol.ParseMobBasics reads the same spot); the template size comes from
// content.BaseMobSize, like the loaders.
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
	Skipped int // NPCs/merchants, level-0 templates and non-816-byte files
}

// Stamp walks dir (one raw STRUCT_MOB per file) and rewrites the Exp field of
// every real monster (Merchant==0, Level>=1) with level.MobExpForLevel,
// leaving every other byte untouched. NPC/merchant and level-0 templates are
// skipped. With dryRun the report is produced but nothing is written.
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
		if len(raw) != content.BaseMobSize {
			res.Skipped++
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
