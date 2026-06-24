package content

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// NPCGenerator is one spawn block of NPCGener.txt (CNPCGene.cpp): it spawns
// MinGroup..MaxGroup mobs of Leader around (StartX,StartY) within StartRange.
type NPCGenerator struct {
	Leader     string
	StartX     int16
	StartY     int16
	StartRange int
	MinGroup   int
	MaxNumMob  int
}

// LoadNPCGenerators parses NPCGener.txt. Blocks start with '#'; lines are
// "Key:\tvalue"; '//' lines are comments. Only the fields needed to spawn are
// read (Leader, StartX/Y, StartRange, MinGroup).
func LoadNPCGenerators(path string) ([]NPCGenerator, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("content: open NPCGener: %w", err)
	}
	defer f.Close()

	var out []NPCGenerator
	var cur *NPCGenerator
	flush := func() {
		if cur != nil && cur.Leader != "" {
			out = append(out, *cur)
		}
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "#") {
			flush()
			cur = &NPCGenerator{MinGroup: 1}
			continue
		}
		if cur == nil {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "Leader":
			cur.Leader = val
		case "StartX":
			cur.StartX = int16(atoi(val))
		case "StartY":
			cur.StartY = int16(atoi(val))
		case "StartRange":
			cur.StartRange = atoi(val)
		case "MinGroup":
			cur.MinGroup = atoi(val)
		case "MaxNumMob":
			cur.MaxNumMob = atoi(val)
		}
	}
	flush()
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("content: scan NPCGener: %w", err)
	}
	return out, nil
}

// LoadNPCTemplate reads one mob template (Release/TMsrv/run/npc/<name>), a raw
// 816-byte STRUCT_MOB. Templates are cached by the caller (many generators share
// a name).
func LoadNPCTemplate(dir, name string) ([]byte, error) {
	b, err := os.ReadFile(filepath.Join(dir, "TMsrv", "run", "npc", name))
	if err != nil {
		return nil, err
	}
	if len(b) != BaseMobSize {
		return nil, fmt.Errorf("content: npc %s = %d bytes, want %d", name, len(b), BaseMobSize)
	}
	return b, nil
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
