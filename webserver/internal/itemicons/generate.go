package itemicons

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"sort"
)

// Generate reads the icon table and numbered atlases from clientDir and writes
// a versioned, CDN-ready pack below outputDir.
func Generate(clientDir, outputDir string) (Manifest, error) {
	tablePath := filepath.Join(clientDir, "itemicon.bin")
	table, err := os.ReadFile(tablePath)
	if err != nil {
		return Manifest{}, fmt.Errorf("itemicons: read %s: %w", tablePath, err)
	}
	itemToIcon, err := decodeIconTable(table)
	if err != nil {
		return Manifest{}, fmt.Errorf("itemicons: decode %s: %w", tablePath, err)
	}
	distinct := make(map[int]struct{})
	maxIcon := -1
	mapped := 0
	for _, icon := range itemToIcon {
		if icon < 0 {
			continue
		}
		mapped++
		distinct[icon] = struct{}{}
		if icon > maxIcon {
			maxIcon = icon
		}
	}
	if maxIcon < 0 {
		return Manifest{}, fmt.Errorf("itemicons: itemicon.bin contains no mappings")
	}

	atlasCount := maxIcon/IconsPerAtlas + 1
	atlasNames := make([]string, atlasCount)
	atlases := make([]*image.NRGBA, atlasCount)
	hash := sha256.New()
	_, _ = hash.Write(table) // hash.Hash.Write never returns an error.
	for i := range atlasCount {
		name := fmt.Sprintf("itemicon%02d.wyt", i+1)
		path := filepath.Join(clientDir, "UI", name)
		data, err := os.ReadFile(path)
		if err != nil {
			return Manifest{}, fmt.Errorf("itemicons: read %s: %w", path, err)
		}
		img, err := decodeWYT(data)
		if err != nil {
			return Manifest{}, fmt.Errorf("itemicons: decode %s: %w", path, err)
		}
		if img.Bounds().Dx() < Columns*CellSize || img.Bounds().Dy() < (IconsPerAtlas/Columns)*CellSize {
			return Manifest{}, fmt.Errorf("itemicons: atlas %s is %dx%d, want at least %dx%d", name, img.Bounds().Dx(), img.Bounds().Dy(), Columns*CellSize, (IconsPerAtlas/Columns)*CellSize)
		}
		atlasNames[i] = name
		atlases[i] = img
		_, _ = hash.Write([]byte(name))
		_, _ = hash.Write(data)
	}
	packVersion := hex.EncodeToString(hash.Sum(nil))[:16]
	versionDir := filepath.Join(outputDir, packVersion)
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		return Manifest{}, fmt.Errorf("itemicons: create %s: %w", versionDir, err)
	}

	iconIDs := make([]int, 0, len(distinct))
	for icon := range distinct {
		iconIDs = append(iconIDs, icon)
	}
	sort.Ints(iconIDs)
	for _, icon := range iconIDs {
		atlas := atlases[icon/IconsPerAtlas]
		cell := icon % IconsPerAtlas
		x := (cell % Columns) * CellSize
		y := (cell / Columns) * CellSize
		crop := image.NewNRGBA(image.Rect(0, 0, CellSize, CellSize))
		draw.Draw(crop, crop.Bounds(), atlas, image.Pt(x, y), draw.Src)
		path := filepath.Join(versionDir, fmt.Sprintf("i%04d.png", icon))
		if err := writePNG(path, crop); err != nil {
			return Manifest{}, err
		}
	}

	manifest := Manifest{
		Version: 1, PackVersion: packVersion, CellSize: CellSize, Columns: Columns,
		IconsPerAtlas: IconsPerAtlas, Atlases: atlasNames, ItemToIcon: itemToIcon,
		MappedItems: mapped, DistinctIcons: len(distinct),
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, fmt.Errorf("itemicons: generated manifest: %w", err)
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, fmt.Errorf("itemicons: encode manifest: %w", err)
	}
	b = append(b, '\n')
	manifestPath := filepath.Join(outputDir, "manifest.json")
	if err := os.WriteFile(manifestPath, b, 0o644); err != nil {
		return Manifest{}, fmt.Errorf("itemicons: write %s: %w", manifestPath, err)
	}
	return manifest, nil
}

func decodeIconTable(data []byte) ([]int, error) {
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("size %d is not a multiple of 4", len(data))
	}
	if len(data)/4 > ItemCount {
		return nil, fmt.Errorf("contains %d entries, maximum is %d", len(data)/4, ItemCount)
	}
	out := make([]int, ItemCount)
	for i := range out {
		out[i] = -1
	}
	for i := 0; i < len(data)/4; i++ {
		oneBased := int(int32(binary.LittleEndian.Uint32(data[i*4:])))
		if oneBased > 0 {
			out[i] = oneBased - 1
		}
	}
	return out, nil
}

func writePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("itemicons: create %s: %w", path, err)
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		return fmt.Errorf("itemicons: encode %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("itemicons: close %s: %w", path, err)
	}
	return nil
}
