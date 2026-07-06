// Package itemcatalog reads the item name/index lookup the moderator UI needs
// for the shop-item picker (SetNpcShop/SetItemPrice take a raw item_index,
// easy to fat-finger). It parses only the first two columns of
// Release/Common/ItemList.csv (index,Name,...); the rest of that row (price,
// grade, effects) is tmserver/internal/content.ItemEntry's concern, not this
// one — webserver only needs the name to label a picker.
//
// ItemList.csv is ISO-8859-1 (confirmed with `file`), not UTF-8 — accented
// names ("Poção") would come out mojibake over gRPC/JSON otherwise. tmserver's
// own parser has the same raw-bytes-as-string quirk, but never surfaces Name
// to a UI, so it never needed the fix; this package does.
package itemcatalog

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Entry is one item's index/name pair.
type Entry struct {
	Index int32
	Name  string
}

// Scan reads <contentDir>/Common/ItemList.csv and returns every row's
// index/name, sorted by name (then index, for a stable order among the many
// duplicate names in the catalog). Blank lines and malformed rows are
// skipped, mirroring content.parseItemList's tolerance; a duplicate index
// keeps its last occurrence in the file, also matching that parser.
func Scan(contentDir string) ([]Entry, error) {
	path := filepath.Join(contentDir, "Common", "ItemList.csv")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("itemcatalog: open %s: %w", path, err)
	}
	defer f.Close()

	byIndex := make(map[int32]string)
	sc := bufio.NewScanner(f)
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
		if err != nil {
			continue
		}
		byIndex[int32(idx)] = latin1ToUTF8(strings.TrimSpace(fields[1]))
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("itemcatalog: scan %s: %w", path, err)
	}

	out := make([]Entry, 0, len(byIndex))
	for idx, name := range byIndex {
		out = append(out, Entry{Index: idx, Name: name})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Index < out[j].Index
	})
	return out, nil
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
