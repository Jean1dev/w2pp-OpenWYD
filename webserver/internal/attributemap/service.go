// Package attributemap implements the moderator-facing AttributeMap.dat editor
// surface. It is the web-api replacement for the legacy C++ AttributeMap_Editor:
// read the configured binary asset, apply bulk transformations in memory, and
// return a new .dat payload for controlled operator replacement.
package attributemap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

const (
	// Dim is AttributeMap.dat's side length in cells.
	Dim = 1024
	// WorldScale is the number of world cells covered by one attribute cell.
	WorldScale = 4

	mapSize        = Dim * Dim
	mapFilename    = "AttributeMap.dat"
	outputFilename = "AttributeMap_New.dat"
	maxByteValue   = 255
)

// Store is the persistence surface the service needs (satisfied by *store.Store).
type Store interface {
	AccountRole(ctx context.Context, id int64) (string, error)
}

// Result is the business outcome of an admin operation. Only role lookup
// failures are returned as errors; invalid content/configuration rides in the
// response body.
type Result int

const (
	// OK means the operation succeeded.
	OK Result = iota
	// Forbidden means the caller is not a moderator/admin.
	Forbidden
	// Invalid means the request or configured AttributeMap.dat failed validation.
	Invalid
)

// Operation is the transformation applied to matching cells.
type Operation int

const (
	// OperationUnspecified is rejected.
	OperationUnspecified Operation = iota
	// OperationLegacyMarkPvPExpLoss ports the legacy AttributeMap_Editor rule:
	// if value < 64 && value != 1 { value |= 64 }.
	OperationLegacyMarkPvPExpLoss
	// OperationAssignValue replaces the cell with Operand.
	OperationAssignValue
	// OperationSetBits applies value |= Operand.
	OperationSetBits
	// OperationClearBits applies value &^= Operand.
	OperationClearBits
	// OperationToggleBits applies value ^= Operand.
	OperationToggleBits
)

// Meaning describes one known exact value or bit in AttributeMap.dat.
type Meaning struct {
	Value       int
	Name        string
	Description string
	Bit         bool
}

// ValueCount is one histogram bucket.
type ValueCount struct {
	Value int
	Count int64
}

// Info summarizes the configured AttributeMap.dat asset.
type Info struct {
	Dim        int
	WorldScale int
	SHA256     string
	Histogram  []ValueCount
	Meanings   []Meaning
}

// Rect restricts a transform to an inclusive world-coordinate rectangle.
type Rect struct {
	Enabled bool
	MinX    int
	MinY    int
	MaxX    int
	MaxY    int
}

// Filter optionally restricts which cells inside a rectangle are modified.
type Filter struct {
	Enabled    bool
	ExactValue int
	Mask       int
	MatchValue int
}

// TransformRequest is the domain form of TransformAttributeMapRequest.
type TransformRequest struct {
	Operation Operation
	Operand   int
	Rect      Rect
	Filter    Filter
}

// TransformResult contains the transformed map and change summary.
type TransformResult struct {
	ChangedCount    int
	BeforeHistogram []ValueCount
	AfterHistogram  []ValueCount
	OriginalSHA256  string
	NewSHA256       string
	Filename        string
	Data            []byte
}

// Service implements the AttributeMap editor operations.
type Service struct {
	store      Store
	contentDir string
}

// New builds the service over the given store and Release/ content directory.
func New(s Store, contentDir string) *Service {
	return &Service{store: s, contentDir: contentDir}
}

// GetInfo returns dimensions, known meanings, histogram, and SHA-256 for the
// configured AttributeMap.dat.
func (s *Service) GetInfo(ctx context.Context, moderatorID int64) (Result, Info, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, Info{}, err
	}
	data, ok := s.readMap()
	if !ok {
		return Invalid, Info{}, nil
	}
	return OK, infoFor(data), nil
}

// Transform applies req to the configured AttributeMap.dat and returns a new
// binary payload without overwriting the source file.
func (s *Service) Transform(ctx context.Context, moderatorID int64, req TransformRequest) (Result, TransformResult, error) {
	if r, err := s.authorize(ctx, moderatorID); r != OK || err != nil {
		return r, TransformResult{}, err
	}
	data, ok := s.readMap()
	if !ok {
		return Invalid, TransformResult{}, nil
	}
	before := histogram(data)
	originalHash := hash(data)

	out := append([]byte(nil), data...)
	changed, ok := apply(out, Dim, req)
	if !ok {
		return Invalid, TransformResult{}, nil
	}
	return OK, TransformResult{
		ChangedCount:    changed,
		BeforeHistogram: histogramCounts(before),
		AfterHistogram:  histogramCounts(histogram(out)),
		OriginalSHA256:  originalHash,
		NewSHA256:       hash(out),
		Filename:        outputFilename,
		Data:            out,
	}, nil
}

func (s *Service) readMap() ([]byte, bool) {
	if s.contentDir == "" {
		return nil, false
	}
	data, err := os.ReadFile(filepath.Join(s.contentDir, "TMsrv", "run", mapFilename))
	if err != nil || len(data) != mapSize {
		return nil, false
	}
	return data, true
}

func (s *Service) authorize(ctx context.Context, moderatorID int64) (Result, error) {
	if moderatorID <= 0 {
		return Forbidden, nil
	}
	role, err := s.store.AccountRole(ctx, moderatorID)
	if errors.Is(err, store.ErrNotFound) {
		return Forbidden, nil
	}
	if err != nil {
		return Invalid, fmt.Errorf("attributemap: role lookup %d: %w", moderatorID, err)
	}
	if role != "moderator" && role != "admin" {
		return Forbidden, nil
	}
	return OK, nil
}

func infoFor(data []byte) Info {
	return Info{
		Dim:        Dim,
		WorldScale: WorldScale,
		SHA256:     hash(data),
		Histogram:  histogramCounts(histogram(data)),
		Meanings:   KnownMeanings(),
	}
}

// KnownMeanings returns the AttributeMap values/bits confirmed from the legacy
// editor comments and runtime server reads.
func KnownMeanings() []Meaning {
	return []Meaning{
		{Value: 0, Name: "PvE", Description: "PvE area without PvP flag", Bit: false},
		{Value: 1, Name: "City", Description: "city/safe-area marker", Bit: false},
		{Value: 2, Name: "Blocked", Description: "blocks pathfinding when baked into HeightMap.dat", Bit: true},
		{Value: 4, Name: "No Gem Mark", Description: "prevents marking Gema in this area", Bit: true},
		{Value: 64, Name: "PvP Exp Loss", Description: "PvP area flag used by combat/XP-loss gates", Bit: true},
		{Value: 128, Name: "Newbie Zone", Description: "newbie-zone movement gate", Bit: true},
	}
}

func apply(data []byte, dim int, req TransformRequest) (int, bool) {
	if len(data) != dim*dim || dim <= 0 {
		return 0, false
	}
	x1, y1, x2, y2, ok := rectCells(dim, req.Rect)
	if !ok || !validRequest(req) {
		return 0, false
	}
	changed := 0
	for y := y1; y <= y2; y++ {
		row := data[y*dim : (y+1)*dim]
		for x := x1; x <= x2; x++ {
			old := row[x]
			if !matches(old, req.Filter) {
				continue
			}
			next := transform(old, req)
			if next != old {
				row[x] = next
				changed++
			}
		}
	}
	return changed, true
}

func validRequest(req TransformRequest) bool {
	if req.Filter.Enabled {
		if !validByte(req.Filter.ExactValue) || !validByte(req.Filter.Mask) || !validByte(req.Filter.MatchValue) {
			return false
		}
	}
	switch req.Operation {
	case OperationLegacyMarkPvPExpLoss:
		return true
	case OperationAssignValue:
		return validByte(req.Operand)
	case OperationSetBits, OperationClearBits, OperationToggleBits:
		return validByte(req.Operand) && req.Operand != 0
	default:
		return false
	}
}

func rectCells(dim int, rect Rect) (int, int, int, int, bool) {
	if !rect.Enabled {
		return 0, 0, dim - 1, dim - 1, true
	}
	maxWorld := dim*WorldScale - 1
	if rect.MinX < 0 || rect.MinY < 0 || rect.MaxX < 0 || rect.MaxY < 0 {
		return 0, 0, 0, 0, false
	}
	if rect.MinX > rect.MaxX || rect.MinY > rect.MaxY || rect.MaxX > maxWorld || rect.MaxY > maxWorld {
		return 0, 0, 0, 0, false
	}
	return rect.MinX / WorldScale, rect.MinY / WorldScale, rect.MaxX / WorldScale, rect.MaxY / WorldScale, true
}

func matches(value byte, filter Filter) bool {
	if !filter.Enabled {
		return true
	}
	if filter.Mask != 0 {
		mask := byte(filter.Mask)
		return value&mask == byte(filter.MatchValue)&mask
	}
	return value == byte(filter.ExactValue)
}

func transform(value byte, req TransformRequest) byte {
	switch req.Operation {
	case OperationLegacyMarkPvPExpLoss:
		if value < 64 && value != 1 {
			return value | 64
		}
		return value
	case OperationAssignValue:
		return byte(req.Operand)
	case OperationSetBits:
		return value | byte(req.Operand)
	case OperationClearBits:
		return value &^ byte(req.Operand)
	case OperationToggleBits:
		return value ^ byte(req.Operand)
	default:
		return value
	}
}

func validByte(v int) bool { return v >= 0 && v <= maxByteValue }

func histogram(data []byte) [256]int64 {
	var h [256]int64
	for _, b := range data {
		h[b]++
	}
	return h
}

func histogramCounts(h [256]int64) []ValueCount {
	out := make([]ValueCount, 0, len(h))
	for value, count := range h {
		out = append(out, ValueCount{Value: value, Count: count})
	}
	return out
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
