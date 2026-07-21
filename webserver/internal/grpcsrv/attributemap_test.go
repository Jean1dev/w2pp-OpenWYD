package grpcsrv

import (
	"context"
	"errors"
	"testing"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/attributemap"
)

type fakeAttributeMapAdmin struct {
	infoRes attributemap.Result
	info    attributemap.Info
	infoErr error

	transformRes attributemap.Result
	transform    attributemap.TransformResult
	transformErr error
	lastRequest  attributemap.TransformRequest

	lastModeratorID int64
}

func (f *fakeAttributeMapAdmin) GetInfo(_ context.Context, moderatorID int64) (attributemap.Result, attributemap.Info, error) {
	f.lastModeratorID = moderatorID
	return f.infoRes, f.info, f.infoErr
}

func (f *fakeAttributeMapAdmin) Transform(_ context.Context, moderatorID int64, req attributemap.TransformRequest) (attributemap.Result, attributemap.TransformResult, error) {
	f.lastModeratorID = moderatorID
	f.lastRequest = req
	return f.transformRes, f.transform, f.transformErr
}

func TestGetAttributeMapInfoMapping(t *testing.T) {
	fake := &fakeAttributeMapAdmin{
		infoRes: attributemap.OK,
		info: attributemap.Info{
			Dim:        1024,
			WorldScale: 4,
			SHA256:     "abc",
			Histogram:  []attributemap.ValueCount{{Value: 0, Count: 10}, {Value: 64, Count: 2}},
			Meanings:   []attributemap.Meaning{{Value: 64, Name: "PvP Exp Loss", Bit: true}},
		},
	}
	s := NewAttributeMapAdmin(fake)

	resp, err := s.GetAttributeMapInfo(context.Background(), &webv1.GetAttributeMapInfoRequest{ModeratorId: 7})
	if err != nil {
		t.Fatalf("GetAttributeMapInfo: %v", err)
	}
	if fake.lastModeratorID != 7 {
		t.Errorf("moderator id = %d, want 7", fake.lastModeratorID)
	}
	if resp.GetResult() != webv1.AdminResult_ADMIN_RESULT_OK {
		t.Fatalf("result = %v, want OK", resp.GetResult())
	}
	if resp.GetInfo().GetDim() != 1024 || resp.GetInfo().GetWorldScale() != 4 || resp.GetInfo().GetSha256() != "abc" {
		t.Errorf("info = %+v, want dim/scale/hash mapped", resp.GetInfo())
	}
	if len(resp.GetInfo().GetHistogram()) != 2 || resp.GetInfo().GetHistogram()[1].GetValue() != 64 {
		t.Errorf("histogram = %+v, want mapped buckets", resp.GetInfo().GetHistogram())
	}
	if len(resp.GetInfo().GetMeanings()) != 1 || !resp.GetInfo().GetMeanings()[0].GetBit() {
		t.Errorf("meanings = %+v, want mapped bit meaning", resp.GetInfo().GetMeanings())
	}
}

func TestTransformAttributeMapMapping(t *testing.T) {
	fake := &fakeAttributeMapAdmin{
		transformRes: attributemap.OK,
		transform: attributemap.TransformResult{
			ChangedCount:    12,
			BeforeHistogram: []attributemap.ValueCount{{Value: 0, Count: 12}},
			AfterHistogram:  []attributemap.ValueCount{{Value: 64, Count: 12}},
			OriginalSHA256:  "old",
			NewSHA256:       "new",
			Filename:        "AttributeMap_New.dat",
			Data:            []byte{64, 64},
		},
	}
	s := NewAttributeMapAdmin(fake)

	resp, err := s.TransformAttributeMap(context.Background(), &webv1.TransformAttributeMapRequest{
		ModeratorId: 7,
		Operation:   webv1.AttributeMapTransformOperation_ATTRIBUTE_MAP_TRANSFORM_OPERATION_SET_BITS,
		Operand:     64,
		Rect:        &webv1.AttributeMapRect{MinX: 4, MinY: 8, MaxX: 12, MaxY: 16},
		Filter:      &webv1.AttributeMapTransformFilter{Enabled: true, Mask: 64, MatchValue: 0},
	})
	if err != nil {
		t.Fatalf("TransformAttributeMap: %v", err)
	}
	if fake.lastModeratorID != 7 {
		t.Errorf("moderator id = %d, want 7", fake.lastModeratorID)
	}
	if fake.lastRequest.Operation != attributemap.OperationSetBits || fake.lastRequest.Operand != 64 {
		t.Errorf("request operation/operand = %+v, want set-bits 64", fake.lastRequest)
	}
	if !fake.lastRequest.Rect.Enabled || fake.lastRequest.Rect.MinX != 4 || fake.lastRequest.Rect.MaxY != 16 {
		t.Errorf("request rect = %+v, want mapped rect", fake.lastRequest.Rect)
	}
	if !fake.lastRequest.Filter.Enabled || fake.lastRequest.Filter.Mask != 64 {
		t.Errorf("request filter = %+v, want mapped filter", fake.lastRequest.Filter)
	}
	if resp.GetResult() != webv1.AdminResult_ADMIN_RESULT_OK || resp.GetChangedCount() != 12 {
		t.Fatalf("response = %+v, want OK changed=12", resp)
	}
	if resp.GetOriginalSha256() != "old" || resp.GetNewSha256() != "new" || string(resp.GetData()) != "@@" {
		t.Errorf("response hashes/data = %+v, want mapped transform result", resp)
	}
}

func TestAttributeMapAdminForbiddenAndInfraError(t *testing.T) {
	forbidden := NewAttributeMapAdmin(&fakeAttributeMapAdmin{infoRes: attributemap.Forbidden})
	resp, err := forbidden.GetAttributeMapInfo(context.Background(), &webv1.GetAttributeMapInfoRequest{ModeratorId: 3})
	if err != nil {
		t.Fatalf("GetAttributeMapInfo forbidden: %v", err)
	}
	if resp.GetResult() != webv1.AdminResult_ADMIN_RESULT_FORBIDDEN {
		t.Errorf("result = %v, want FORBIDDEN", resp.GetResult())
	}
	if resp.GetInfo() != nil {
		t.Errorf("info = %+v, want nil on forbidden", resp.GetInfo())
	}

	infra := NewAttributeMapAdmin(&fakeAttributeMapAdmin{transformErr: errors.New("disk failed")})
	if _, err := infra.TransformAttributeMap(context.Background(), &webv1.TransformAttributeMapRequest{}); err == nil {
		t.Fatal("expected gRPC error on infra failure")
	}
}
