package attributemap

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

type fakeStore struct {
	roles map[int64]string
	err   error
}

func (f fakeStore) AccountRole(_ context.Context, id int64) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	role, ok := f.roles[id]
	if !ok {
		return "", store.ErrNotFound
	}
	return role, nil
}

func TestApplyLegacyMarkPvPExpLoss(t *testing.T) {
	data := []byte{0, 1, 2, 4, 63, 64, 65, 128, 0}
	got, ok := apply(data, 3, TransformRequest{Operation: OperationLegacyMarkPvPExpLoss})
	if !ok {
		t.Fatal("apply rejected legacy request")
	}
	if got != 5 {
		t.Fatalf("changed = %d, want 5", got)
	}
	want := []byte{64, 1, 66, 68, 127, 64, 65, 128, 64}
	if !bytes.Equal(data, want) {
		t.Errorf("data = %v, want %v", data, want)
	}
}

func TestApplyWorldRectUsesFourToOneScale(t *testing.T) {
	data := make([]byte, 16)
	got, ok := apply(data, 4, TransformRequest{
		Operation: OperationAssignValue,
		Operand:   7,
		Rect:      Rect{Enabled: true, MinX: 4, MinY: 4, MaxX: 8, MaxY: 8},
	})
	if !ok {
		t.Fatal("apply rejected rectangle request")
	}
	if got != 4 {
		t.Fatalf("changed = %d, want 4", got)
	}
	want := []byte{
		0, 0, 0, 0,
		0, 7, 7, 0,
		0, 7, 7, 0,
		0, 0, 0, 0,
	}
	if !bytes.Equal(data, want) {
		t.Errorf("data = %v, want %v", data, want)
	}
}

func TestApplyFilters(t *testing.T) {
	data := []byte{1, 2, 64, 66}
	got, ok := apply(data, 2, TransformRequest{
		Operation: OperationClearBits,
		Operand:   64,
		Filter:    Filter{Enabled: true, Mask: 64, MatchValue: 64},
	})
	if !ok {
		t.Fatal("apply rejected mask filter request")
	}
	if got != 2 {
		t.Fatalf("changed = %d, want 2", got)
	}
	want := []byte{1, 2, 0, 2}
	if !bytes.Equal(data, want) {
		t.Errorf("data = %v, want %v", data, want)
	}

	got, ok = apply(data, 2, TransformRequest{
		Operation: OperationSetBits,
		Operand:   4,
		Filter:    Filter{Enabled: true, ExactValue: 2},
	})
	if !ok {
		t.Fatal("apply rejected exact filter request")
	}
	if got != 2 {
		t.Fatalf("changed = %d, want 2", got)
	}
	want = []byte{1, 6, 0, 6}
	if !bytes.Equal(data, want) {
		t.Errorf("data = %v, want %v", data, want)
	}
}

func TestApplyRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name string
		req  TransformRequest
	}{
		{"unspecified op", TransformRequest{}},
		{"bad assign operand", TransformRequest{Operation: OperationAssignValue, Operand: 256}},
		{"zero mask", TransformRequest{Operation: OperationSetBits, Operand: 0}},
		{"bad rect", TransformRequest{Operation: OperationAssignValue, Operand: 1, Rect: Rect{Enabled: true, MinX: 8, MaxX: 4}}},
		{"bad filter byte", TransformRequest{Operation: OperationAssignValue, Operand: 1, Filter: Filter{Enabled: true, ExactValue: 300}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := apply(make([]byte, 4), 2, tt.req); ok {
				t.Fatal("apply accepted invalid request")
			}
		})
	}
}

func TestServiceAuthorization(t *testing.T) {
	path := writeContentMap(t, bytes.Repeat([]byte{0}, mapSize))
	s := New(fakeStore{roles: map[int64]string{1: "moderator", 2: "admin", 3: "player"}}, path)

	for _, moderatorID := range []int64{1, 2} {
		got, _, err := s.GetInfo(context.Background(), moderatorID)
		if err != nil {
			t.Fatalf("GetInfo(%d): %v", moderatorID, err)
		}
		if got != OK {
			t.Fatalf("GetInfo(%d) = %v, want OK", moderatorID, got)
		}
	}
	if got, _, err := s.GetInfo(context.Background(), 3); err != nil || got != Forbidden {
		t.Fatalf("player GetInfo result = %v, err %v, want Forbidden, nil", got, err)
	}
	if got, _, err := s.GetInfo(context.Background(), 99); err != nil || got != Forbidden {
		t.Fatalf("missing GetInfo result = %v, err %v, want Forbidden, nil", got, err)
	}
}

func TestGetInfoInvalidWhenContentMissingOrWrongSize(t *testing.T) {
	s := New(fakeStore{roles: map[int64]string{1: "moderator"}}, "")
	if got, _, err := s.GetInfo(context.Background(), 1); err != nil || got != Invalid {
		t.Fatalf("empty content result = %v, err %v, want Invalid, nil", got, err)
	}

	path := writeContentMap(t, []byte{1, 2, 3})
	s = New(fakeStore{roles: map[int64]string{1: "moderator"}}, path)
	if got, _, err := s.GetInfo(context.Background(), 1); err != nil || got != Invalid {
		t.Fatalf("wrong size result = %v, err %v, want Invalid, nil", got, err)
	}
}

func TestTransformReturnsNewDataWithoutOverwritingSource(t *testing.T) {
	original := bytes.Repeat([]byte{0}, mapSize)
	path := writeContentMap(t, original)
	s := New(fakeStore{roles: map[int64]string{1: "moderator"}}, path)

	got, res, err := s.Transform(context.Background(), 1, TransformRequest{
		Operation: OperationSetBits,
		Operand:   64,
	})
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if got != OK {
		t.Fatalf("Transform result = %v, want OK", got)
	}
	if res.ChangedCount != mapSize {
		t.Fatalf("changed = %d, want %d", res.ChangedCount, mapSize)
	}
	if len(res.Data) != mapSize || res.Data[0] != 64 || res.Filename != outputFilename {
		t.Fatalf("result data len=%d first=%d filename=%q", len(res.Data), res.Data[0], res.Filename)
	}

	onDisk, err := os.ReadFile(filepath.Join(path, "TMsrv", "run", mapFilename))
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if !bytes.Equal(onDisk, original) {
		t.Fatal("source AttributeMap.dat was overwritten")
	}
}

func writeContentMap(t *testing.T, data []byte) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "TMsrv", "run")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, mapFilename), data, 0o644); err != nil {
		t.Fatalf("write map: %v", err)
	}
	return filepath.Dir(filepath.Dir(dir))
}
