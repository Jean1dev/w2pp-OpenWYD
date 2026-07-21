// Package npctemplate resolves raw STRUCT_MOB template files from the legacy
// Release/TMsrv/run/npc content tree.
package npctemplate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
)

const (
	// Size is sizeof(STRUCT_MOB), the raw NPC/mob template format.
	Size = savefmt.MobSize
)

// Resolution identifies the concrete file selected for a requested template.
type Resolution struct {
	// Name is the actual file name under Release/TMsrv/run/npc.
	Name string
	// Path is the absolute or relative path used to read the file.
	Path string
}

// Dir returns the NPC template directory for a Release/ content root.
func Dir(contentDir string) string {
	return filepath.Join(contentDir, "TMsrv", "run", "npc")
}

// Resolve maps a template name from legacy content/config to the actual file in
// Release/TMsrv/run/npc. It first honors exact Linux file names, then falls back
// to Windows-like matching for the original server data: case-insensitive, with
// trailing dots ignored in path components.
func Resolve(contentDir, name string) (Resolution, error) {
	if name == "" {
		return Resolution{}, fmt.Errorf("npctemplate: empty template name")
	}
	if filepath.Base(name) != name || strings.ContainsAny(name, `/\`) {
		return Resolution{}, fmt.Errorf("npctemplate: invalid template name %q", name)
	}
	dir := Dir(contentDir)
	exactPath := filepath.Join(dir, name)
	if st, err := os.Stat(exactPath); err == nil {
		if st.IsDir() {
			return Resolution{}, fmt.Errorf("npctemplate: %s is a directory", exactPath)
		}
		return Resolution{Name: name, Path: exactPath}, nil
	} else if !os.IsNotExist(err) {
		return Resolution{}, fmt.Errorf("npctemplate: stat %s: %w", exactPath, err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return Resolution{}, fmt.Errorf("npctemplate: read %s: %w", dir, err)
	}
	want := canonicalName(name)
	var matches []Resolution
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		entryName := entry.Name()
		if canonicalName(entryName) == want {
			matches = append(matches, Resolution{Name: entryName, Path: filepath.Join(dir, entryName)})
		}
	}
	switch len(matches) {
	case 0:
		return Resolution{}, fmt.Errorf("npctemplate: template %q not found in %s", name, dir)
	case 1:
		return matches[0], nil
	default:
		names := make([]string, len(matches))
		for i, match := range matches {
			names[i] = match.Name
		}
		return Resolution{}, fmt.Errorf("npctemplate: template %q is ambiguous in %s: %s", name, dir, strings.Join(names, ", "))
	}
}

// Load reads a template resolved by Resolve and verifies it is one raw
// STRUCT_MOB blob.
func Load(contentDir, name string) ([]byte, Resolution, error) {
	res, err := Resolve(contentDir, name)
	if err != nil {
		return nil, Resolution{}, err
	}
	b, err := os.ReadFile(res.Path)
	if err != nil {
		return nil, Resolution{}, fmt.Errorf("npctemplate: read %s: %w", res.Path, err)
	}
	if len(b) != Size {
		return nil, Resolution{}, fmt.Errorf("npctemplate: npc %s = %d bytes, want %d", res.Name, len(b), Size)
	}
	return b, res, nil
}

func canonicalName(name string) string {
	return strings.ToLower(strings.TrimRight(name, "."))
}
