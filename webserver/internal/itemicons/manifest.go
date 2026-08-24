// Package itemicons reads and generates the item-icon pack derived from the
// classic WYD client. The generated pixels are deployment artifacts and are
// deliberately kept outside the repository.
package itemicons

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

const (
	// ItemCount is MAX_ITEMLIST in the classic client.
	ItemCount = 6500
	// CellSize is the width and height of one classic inventory icon.
	CellSize = 35
	// Columns is the number of icon cells in each atlas row.
	Columns = 10
	// IconsPerAtlas is the number of cells addressed in each numbered atlas.
	IconsPerAtlas = 100
)

// Manifest maps item indices to cells in the generated icon pack.
type Manifest struct {
	Version       int      `json:"version"`
	PackVersion   string   `json:"pack_version"`
	CellSize      int      `json:"cell_size"`
	Columns       int      `json:"columns"`
	IconsPerAtlas int      `json:"icons_per_atlas"`
	Atlases       []string `json:"atlases"`
	ItemToIcon    []int    `json:"item_to_icon"`
	MappedItems   int      `json:"mapped_items"`
	DistinctIcons int      `json:"distinct_icons"`
	// IconURLs is populated by the storage-manager uploader. Generated packs
	// leave it empty until every referenced cell has a public URL.
	IconURLs map[string]string `json:"icon_urls,omitempty"`
}

// Load reads and validates a generated item-icon manifest.
func Load(path string) (Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("itemicons: read manifest %s: %w", path, err)
	}
	var manifest Manifest
	if err := json.Unmarshal(b, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("itemicons: decode manifest %s: %w", path, err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, fmt.Errorf("itemicons: manifest %s: %w", path, err)
	}
	return manifest, nil
}

// Validate checks the stable v1 manifest contract.
func (m Manifest) Validate() error {
	if m.Version != 1 {
		return fmt.Errorf("unsupported version %d", m.Version)
	}
	if m.PackVersion == "" {
		return fmt.Errorf("empty pack_version")
	}
	if m.CellSize != CellSize || m.Columns != Columns || m.IconsPerAtlas != IconsPerAtlas {
		return fmt.Errorf("geometry %dx%d/%d, want %dx%d/%d", m.CellSize, m.Columns, m.IconsPerAtlas, CellSize, Columns, IconsPerAtlas)
	}
	if len(m.Atlases) == 0 {
		return fmt.Errorf("no atlases")
	}
	if len(m.ItemToIcon) != ItemCount {
		return fmt.Errorf("item_to_icon has %d entries, want %d", len(m.ItemToIcon), ItemCount)
	}
	maxIcon := len(m.Atlases)*IconsPerAtlas - 1
	mapped := 0
	icons := make(map[int]struct{})
	for itemIndex, icon := range m.ItemToIcon {
		if icon < 0 {
			continue
		}
		if icon > maxIcon {
			return fmt.Errorf("item %d references icon %d outside [0,%d]", itemIndex, icon, maxIcon)
		}
		mapped++
		icons[icon] = struct{}{}
	}
	if m.MappedItems != mapped || m.DistinctIcons != len(icons) {
		return fmt.Errorf("counts mapped/distinct = %d/%d, calculated %d/%d", m.MappedItems, m.DistinctIcons, mapped, len(icons))
	}
	for key, rawURL := range m.IconURLs {
		var icon int
		if _, err := fmt.Sscanf(key, "i%04d", &icon); err != nil || key != fmt.Sprintf("i%04d", icon) {
			return fmt.Errorf("invalid icon_urls key %q", key)
		}
		if _, ok := icons[icon]; !ok {
			return fmt.Errorf("icon_urls key %q is not referenced by item_to_icon", key)
		}
		u, err := url.Parse(rawURL)
		if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Host == "" {
			return fmt.Errorf("icon_urls[%q] is not an absolute HTTPS URL", key)
		}
	}
	return nil
}

// IconKey returns the CDN filename stem for an item, or an empty string when
// the classic client has no icon mapping for it.
func (m Manifest) IconKey(itemIndex int32) string {
	if itemIndex < 0 || int(itemIndex) >= len(m.ItemToIcon) {
		return ""
	}
	icon := m.ItemToIcon[itemIndex]
	if icon < 0 {
		return ""
	}
	return fmt.Sprintf("i%04d", icon)
}

// IconURL returns the public URL uploaded for an item, or an empty string while
// the pack has not been published or the item has no classic icon.
func (m Manifest) IconURL(itemIndex int32) string {
	key := m.IconKey(itemIndex)
	if key == "" {
		return ""
	}
	return m.IconURLs[key]
}
