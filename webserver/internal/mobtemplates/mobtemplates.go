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
	"sort"

	"github.com/jeanluca/w2pp-openwyd/internal/npctemplate"
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
// STRUCT_MOB in any layout savefmt knows (816 canonical plus the legacy 756/920
// forms — data-formats.md §1.4.1), sorted by TemplateName. A file that isn't a
// STRUCT_MOB at all is counted in the returned stats rather than failing the
// whole scan — the npc/ directory can contain non-template files. Callers log
// the stats once instead of warning per file.
func Scan(contentDir string, logger *slog.Logger) ([]File, npctemplate.ScanStats, error) {
	var out []File
	stats, err := npctemplate.ScanDir(contentDir, logger, func(name string, mob savefmt.Mob) {
		out = append(out, File{
			TemplateName: name,
			DisplayName:  npctemplates.CString(mob.Name[:]),
			Merchant:     mob.CurrentScore.Merchant,
		})
	})
	if err != nil {
		return nil, stats, fmt.Errorf("mobtemplates: %w", err)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].TemplateName < out[j].TemplateName })
	return out, stats, nil
}
