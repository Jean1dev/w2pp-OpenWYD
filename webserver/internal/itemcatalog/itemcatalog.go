// Package itemcatalog reads the item lookup the web front-end needs from
// Release/Common/ItemList.csv: the index/name pair the moderator shop-item
// picker labels its rows with, plus the fallback metadata every item-facing
// screen needs when a classic-client icon is unavailable.
//
// The classic client resolves real pixels through itemicon.bin, a direct map
// from item index to a numbered atlas cell. ApplyIcons joins that separately
// generated manifest; a catalog scanned on its own remains fallback-only.
//
// The rest of the row (price, effects, requirements) stays
// tmserver/internal/content.ItemEntry's concern — webserver only needs what a
// picker and an icon need.
//
// ItemList.csv is ISO-8859-1 (confirmed with `file`), not UTF-8 — accented
// names ("Poção") would come out mojibake over gRPC/JSON otherwise. tmserver's
// own parser has the same raw-bytes-as-string quirk, but never surfaces Name
// to a UI, so it never needed the fix; this package does.
package itemcatalog

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jeanluca/w2pp-openwyd/webserver/internal/itemicons"
)

// Entry is one catalog row, trimmed to what the web needs.
type Entry struct {
	Index int32
	Name  string // raw catalog name, e.g. "Botas_Douradas(N)"
	// DisplayName is Name with underscores turned back into spaces — the
	// catalog stores "Botas_Douradas(N)" because the legacy CSV has no
	// quoting and a space would still parse, but reads badly in a UI.
	DisplayName string
	Mesh        int32 // IndexMesh: the model/appearance (the "set")
	Texture     int32 // IndexTexture: colour variant of that model
	// SlotMask is nPos, a bitmask over STRUCT_MOB.Equip[16]: bit i means the
	// item goes in Equip[i]. 0 means not equippable (potions, coupons, quest
	// items); 64|128 is a two-handed weapon, which claims the shield slot too.
	SlotMask int32
	Slots    []string // SlotMask decoded; empty when SlotMask is 0
	Grade    int32    // 1=Normal 2=Místico 3=Arcano 4=Lendário …
	// IconKey is the generated atlas-cell key ("iNNNN"). Empty means no mapping.
	IconKey string
	// IconURL is the public URL returned by storage-manager-server. Empty means
	// the generated pack has not been uploaded or the item has no mapping.
	IconURL string
}

// Catalog is the scanned item list plus a fingerprint of the file it came from.
type Catalog struct {
	Items []Entry
	// Version fingerprints ItemList.csv so a caller can cache the list and
	// revalidate cheaply. The catalog is immutable at runtime (the content
	// tree is mounted read-only), so this only changes on redeploy.
	Version string
	// IconPackVersion fingerprints itemicon.bin and its source atlases. Empty
	// means this catalog is operating in fallback-only mode.
	IconPackVersion string
}

// ApplyIcons joins a validated classic-client icon manifest onto a catalog.
// itemicon.bin is indexed directly by item index; mesh, texture and position
// are fallback metadata and are not an icon lookup key.
func ApplyIcons(catalog *Catalog, manifest itemicons.Manifest) {
	for i := range catalog.Items {
		catalog.Items[i].IconKey = manifest.IconKey(catalog.Items[i].Index)
		catalog.Items[i].IconURL = manifest.IconURL(catalog.Items[i].Index)
	}
	catalog.IconPackVersion = manifest.PackVersion
}

// slotNames maps each nPos bit to the Equip[] slot it selects. Bit 13 is the
// fairy (CMob.cpp:712 keys off Equip[13] for the 39xx fairies) and bit 14 the
// mount (mountEquipSlot in tmserver/internal/handler/character.go).
var slotNames = [16]string{
	"face", "helmet", "armor", "pants", "gloves", "boots", "weapon", "shield",
	"accessory", "amulet", "orb", "gem", "medal", "fairy", "mount", "cape",
}

// decodeSlots turns an nPos bitmask into slot names, low bit first.
func decodeSlots(mask int32) []string {
	if mask == 0 {
		return nil
	}
	out := make([]string, 0, 2)
	for bit, name := range slotNames {
		if mask&(1<<uint(bit)) != 0 {
			out = append(out, name)
		}
	}
	return out
}

// Scan reads <contentDir>/Common/ItemList.csv and returns every row's web-facing
// fields, sorted by name (then index, for a stable order among the many
// duplicate names in the catalog). Blank lines and malformed rows are skipped,
// mirroring content.parseItemList's tolerance; a duplicate index keeps its last
// occurrence in the file, also matching that parser. Columns past the end of a
// short row read as zero rather than dropping the row.
//
// Column layout follows BASE_ReadItemListFile (Basedef.cpp:5717):
// index, name, mesh.texture, lvl.str.int.dex.con, unique, price, nPos, extra,
// grade, then up to 12 "EF_<name>,value" pairs.
func Scan(contentDir string) (Catalog, error) {
	path := filepath.Join(contentDir, "Common", "ItemList.csv")
	f, err := os.Open(path)
	if err != nil {
		return Catalog{}, fmt.Errorf("itemcatalog: open %s: %w", path, err)
	}
	defer f.Close()

	// Hash exactly the bytes the scanner consumes, so Version covers the whole
	// file without a second read.
	sum := sha256.New()
	byIndex := make(map[int32]Entry)
	sc := bufio.NewScanner(io.TeeReader(f, sum))
	sc.Buffer(make([]byte, 64*1024), 1024*1024) // rows can be long
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		if len(fields) < 2 {
			continue
		}
		idx, err := strconv.Atoi(strings.TrimSpace(fields[0]))
		if err != nil || idx < 0 {
			continue
		}
		name := latin1ToUTF8(strings.TrimSpace(fields[1]))
		if name == "" {
			continue
		}
		mesh, texture, _ := strings.Cut(column(fields, 2), ".")
		// nPos is a signed short, so capes (bit 15) arrive as -32768; mask to
		// 16 bits before decoding or the whole mask reads as negative.
		slotMask := number(column(fields, 6)) & 0xFFFF
		byIndex[int32(idx)] = Entry{
			Index:       int32(idx),
			Name:        name,
			DisplayName: strings.TrimSpace(strings.ReplaceAll(name, "_", " ")),
			Mesh:        number(mesh),
			Texture:     number(texture),
			SlotMask:    slotMask,
			Slots:       decodeSlots(slotMask),
			Grade:       number(column(fields, 8)),
		}
	}
	if err := sc.Err(); err != nil {
		return Catalog{}, fmt.Errorf("itemcatalog: scan %s: %w", path, err)
	}

	out := make([]Entry, 0, len(byIndex))
	for _, e := range byIndex {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Index < out[j].Index
	})
	return Catalog{Items: out, Version: hex.EncodeToString(sum.Sum(nil))[:16]}, nil
}

// column returns field i, or "" when the row is shorter than that.
func column(fields []string, i int) string {
	if i >= len(fields) {
		return ""
	}
	return fields[i]
}

// number parses a catalog integer, treating anything unparseable as 0 — the
// same tolerance the rest of the row parsers apply.
func number(s string) int32 {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return int32(v)
}

// latin1ToUTF8 converts a string holding raw ISO-8859-1 bytes (each byte's
// value IS its Unicode code point, by definition of that encoding) into
// proper UTF-8. bufio.Scanner's Text() just wraps the raw bytes in a Go
// string without transcoding, so this must run before the name reaches gRPC.
func latin1ToUTF8(s string) string {
	b := []byte(s)
	runes := make([]rune, len(b))
	for i, c := range b {
		runes[i] = rune(c)
	}
	return string(runes)
}
