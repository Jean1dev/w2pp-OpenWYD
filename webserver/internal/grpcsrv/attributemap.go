package grpcsrv

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/attributemap"
)

// AttributeMapAdmin is the AttributeMap editor surface the server depends on
// (satisfied by *attributemap.Service).
type AttributeMapAdmin interface {
	GetInfo(ctx context.Context, moderatorID int64) (attributemap.Result, attributemap.Info, error)
	Transform(ctx context.Context, moderatorID int64, req attributemap.TransformRequest) (attributemap.Result, attributemap.TransformResult, error)
}

// AttributeMapAdminServer implements webv1.AttributeMapAdminServiceServer.
type AttributeMapAdminServer struct {
	webv1.UnimplementedAttributeMapAdminServiceServer
	admin AttributeMapAdmin
}

// NewAttributeMapAdmin builds the AttributeMapAdminService over the given admin
// logic.
func NewAttributeMapAdmin(a AttributeMapAdmin) *AttributeMapAdminServer {
	return &AttributeMapAdminServer{admin: a}
}

// GetAttributeMapInfo returns metadata and a value histogram for AttributeMap.dat.
func (s *AttributeMapAdminServer) GetAttributeMapInfo(ctx context.Context, req *webv1.GetAttributeMapInfoRequest) (*webv1.GetAttributeMapInfoResponse, error) {
	res, info, err := s.admin.GetInfo(ctx, req.GetModeratorId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get attribute map info: %v", err)
	}
	resp := &webv1.GetAttributeMapInfoResponse{Result: attributeMapResultToProto(res)}
	if res == attributemap.OK {
		resp.Info = attributeMapInfoToProto(info)
	}
	return resp, nil
}

// TransformAttributeMap applies one bulk transformation and returns a new .dat
// payload without overwriting the configured source file.
func (s *AttributeMapAdminServer) TransformAttributeMap(ctx context.Context, req *webv1.TransformAttributeMapRequest) (*webv1.TransformAttributeMapResponse, error) {
	res, out, err := s.admin.Transform(ctx, req.GetModeratorId(), protoToAttributeMapTransform(req))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "transform attribute map: %v", err)
	}
	resp := &webv1.TransformAttributeMapResponse{Result: attributeMapResultToProto(res)}
	if res == attributemap.OK {
		resp.ChangedCount = int32(out.ChangedCount)
		resp.BeforeHistogram = attributeMapHistogramToProto(out.BeforeHistogram)
		resp.AfterHistogram = attributeMapHistogramToProto(out.AfterHistogram)
		resp.OriginalSha256 = out.OriginalSHA256
		resp.NewSha256 = out.NewSHA256
		resp.Filename = out.Filename
		resp.Data = out.Data
	}
	return resp, nil
}

func attributeMapInfoToProto(info attributemap.Info) *webv1.AttributeMapInfo {
	return &webv1.AttributeMapInfo{
		Dim:        int32(info.Dim),
		WorldScale: int32(info.WorldScale),
		Sha256:     info.SHA256,
		Histogram:  attributeMapHistogramToProto(info.Histogram),
		Meanings:   attributeMapMeaningsToProto(info.Meanings),
	}
}

func attributeMapHistogramToProto(hist []attributemap.ValueCount) []*webv1.AttributeMapValueCount {
	out := make([]*webv1.AttributeMapValueCount, 0, len(hist))
	for _, h := range hist {
		out = append(out, &webv1.AttributeMapValueCount{Value: int32(h.Value), Count: h.Count})
	}
	return out
}

func attributeMapMeaningsToProto(meanings []attributemap.Meaning) []*webv1.AttributeMapMeaning {
	out := make([]*webv1.AttributeMapMeaning, 0, len(meanings))
	for _, m := range meanings {
		out = append(out, &webv1.AttributeMapMeaning{
			Value:       int32(m.Value),
			Name:        m.Name,
			Description: m.Description,
			Bit:         m.Bit,
		})
	}
	return out
}

func protoToAttributeMapTransform(req *webv1.TransformAttributeMapRequest) attributemap.TransformRequest {
	out := attributemap.TransformRequest{
		Operation: protoToAttributeMapOperation(req.GetOperation()),
		Operand:   int(req.GetOperand()),
	}
	if r := req.GetRect(); r != nil {
		out.Rect = attributemap.Rect{
			Enabled: true,
			MinX:    int(r.GetMinX()),
			MinY:    int(r.GetMinY()),
			MaxX:    int(r.GetMaxX()),
			MaxY:    int(r.GetMaxY()),
		}
	}
	if f := req.GetFilter(); f != nil {
		out.Filter = attributemap.Filter{
			Enabled:    f.GetEnabled(),
			ExactValue: int(f.GetExactValue()),
			Mask:       int(f.GetMask()),
			MatchValue: int(f.GetMatchValue()),
		}
	}
	return out
}

func protoToAttributeMapOperation(op webv1.AttributeMapTransformOperation) attributemap.Operation {
	switch op {
	case webv1.AttributeMapTransformOperation_ATTRIBUTE_MAP_TRANSFORM_OPERATION_LEGACY_MARK_PVP_EXP_LOSS:
		return attributemap.OperationLegacyMarkPvPExpLoss
	case webv1.AttributeMapTransformOperation_ATTRIBUTE_MAP_TRANSFORM_OPERATION_ASSIGN_VALUE:
		return attributemap.OperationAssignValue
	case webv1.AttributeMapTransformOperation_ATTRIBUTE_MAP_TRANSFORM_OPERATION_SET_BITS:
		return attributemap.OperationSetBits
	case webv1.AttributeMapTransformOperation_ATTRIBUTE_MAP_TRANSFORM_OPERATION_CLEAR_BITS:
		return attributemap.OperationClearBits
	case webv1.AttributeMapTransformOperation_ATTRIBUTE_MAP_TRANSFORM_OPERATION_TOGGLE_BITS:
		return attributemap.OperationToggleBits
	default:
		return attributemap.OperationUnspecified
	}
}

func attributeMapResultToProto(r attributemap.Result) webv1.AdminResult {
	switch r {
	case attributemap.OK:
		return webv1.AdminResult_ADMIN_RESULT_OK
	case attributemap.Forbidden:
		return webv1.AdminResult_ADMIN_RESULT_FORBIDDEN
	default:
		return webv1.AdminResult_ADMIN_RESULT_INVALID
	}
}
