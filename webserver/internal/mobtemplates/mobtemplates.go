// Package mobtemplates scans the read-only content tree for EVERY STRUCT_MOB
// template under Release/TMsrv/run/npc/, so the moderator UI can offer a
// searchable picker for MobTemplateAdminService.GetMobTemplateStat/
// UpsertMobTemplateStat's template_name (mob-template-editing-plan.md, the
// equivalent-tool successor to the legacy EDITAPPMOB).
//
// Unlike webserver/internal/npctemplates.Scan (which curates the list down to
// merchant templates for the NPC placement/shop picker), this scan is
// deliberately UNFILTERED: monsters — not merchants — are the primary
// rebalancing use case this tool serves.
//
// STRUCT_MOB.Name is ISO-8859-1 (same source tree, same encoding as
// itemcatalog's ItemList.csv) — see npctemplates.CString for the transcoding.
package mobtemplates

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"github.com/jeanluca/w2pp-openwyd/internal/savefmt"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/npctemplates"
)

// File is one STRUCT_MOB template file found under Release/TMsrv/run/npc/.
type File struct {
	TemplateName string
	DisplayName  string
	Merchant     uint8
}

// Scan reads every file in <contentDir>/TMsrv/run/npc/ and decodes it as a
// STRUCT_MOB, sorted by TemplateName. A file that isn't a valid 816-byte
// STRUCT_MOB is logged and skipped rather than failing the whole scan — the
// npc/ directory can contain non-template files.
func Scan(contentDir string, logger *slog.Logger) ([]File, error) {
	dir := filepath.Join(contentDir, "TMsrv", "run", "npc")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("mobtemplates: read %s: %w", dir, err)
	}

	var out []File
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			logger.Warn("mob template read failed", "name", name, "err", err)
			continue
		}
		mob, err := savefmt.DecodeMob(b)
		if err != nil {
			logger.Warn("mob template decode failed", "name", name, "err", err)
			continue
		}
		out = append(out, File{
			TemplateName: name,
			DisplayName:  npctemplates.CString(mob.Name[:]),
			Merchant:     mob.CurrentScore.Merchant,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].TemplateName < out[j].TemplateName })
	return out, nil
}
