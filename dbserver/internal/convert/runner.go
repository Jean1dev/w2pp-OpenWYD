package convert

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/savefmt"
)

// FileResult is the outcome of converting one account file.
type FileResult struct {
	Path    string
	Version savefmt.Version
	Account *domain.Account // non-nil only when converted
	Skipped string          // reason, when the version is not modelled
	Err     error           // conversion/IO failure
}

// Report summarizes a directory walk.
type Report struct {
	Total     int
	Converted int
	Skipped   int
	Failed    int
	Results   []FileResult
}

// WalkAccounts walks the legacy account/ directory tree and converts every
// current-format (7952-byte) file into a domain.Account. Files in an
// UNVERIFIED legacy layout (4294 / 7500–7600) are recorded as Skipped, not
// failures — they need their layout reversed first (data-formats.md §1.2). The
// returned error is only for walking the tree itself.
func WalkAccounts(root string) (*Report, error) {
	rep := &Report{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rep.Total++
		res := convertFile(path)
		switch {
		case res.Err != nil:
			rep.Failed++
		case res.Account != nil:
			rep.Converted++
		default:
			rep.Skipped++
		}
		rep.Results = append(rep.Results, res)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("convert: walk %q: %w", root, err)
	}
	sort.Slice(rep.Results, func(i, j int) bool { return rep.Results[i].Path < rep.Results[j].Path })
	return rep, nil
}

func convertFile(path string) FileResult {
	b, err := os.ReadFile(path)
	if err != nil {
		return FileResult{Path: path, Err: fmt.Errorf("read: %w", err)}
	}
	v := savefmt.DetectVersion(len(b))
	res := FileResult{Path: path, Version: v}
	if !v.Modelled() {
		res.Skipped = fmt.Sprintf("UNVERIFIED layout %s (size %d)", v, len(b))
		return res
	}
	af, err := savefmt.Decode(b)
	if err != nil {
		res.Err = fmt.Errorf("decode: %w", err)
		return res
	}
	acc, err := Account(af)
	if err != nil {
		res.Err = err
		return res
	}
	res.Account = &acc
	return res
}
