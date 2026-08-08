package grpcsrv

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/webserver/internal/donaterevenue"
)

// DonateRevenue is the revenue reporting surface (satisfied by
// *donaterevenue.Service). Kept as an interface so the server is unit-testable.
type DonateRevenue interface {
	Summary(ctx context.Context, moderatorID, fromUnix, toUnix int64, bucket donaterevenue.Bucket, accountID int64) (donaterevenue.Result, donaterevenue.Summary, error)
	Orders(ctx context.Context, moderatorID, fromUnix, toUnix int64, status, method int16, accountID int64, limit, offset int) (donaterevenue.Result, []domain.TopupOrderRow, int, donaterevenue.Window, error)
	TopBuyers(ctx context.Context, moderatorID, fromUnix, toUnix int64, limit, offset int) (donaterevenue.Result, []domain.TopBuyer, int, donaterevenue.Window, error)
	Spend(ctx context.Context, moderatorID, fromUnix, toUnix int64, action donaterevenue.LedgerAction, accountID int64, limit, offset int) (donaterevenue.Result, []domain.DonateLedgerEntry, int, donaterevenue.Window, error)
	SearchAccounts(ctx context.Context, moderatorID int64, prefix string, limit int) (donaterevenue.Result, []domain.AccountSummary, error)
}

// DonateRevenueServer implements webv1.DonateRevenueAdminServiceServer.
type DonateRevenueServer struct {
	webv1.UnimplementedDonateRevenueAdminServiceServer
	revenue DonateRevenue
}

// NewDonateRevenue builds the DonateRevenueAdminService over the given logic.
func NewDonateRevenue(r DonateRevenue) *DonateRevenueServer { return &DonateRevenueServer{revenue: r} }

// GetRevenueSummary returns the KPI header, the per-gateway split and the series.
func (s *DonateRevenueServer) GetRevenueSummary(ctx context.Context, req *webv1.GetRevenueSummaryRequest) (*webv1.GetRevenueSummaryResponse, error) {
	res, sum, err := s.revenue.Summary(ctx, req.GetModeratorId(),
		req.GetWindow().GetFromUnix(), req.GetWindow().GetToUnix(),
		revenueBucketFromProto(req.GetBucket()), req.GetAccountId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "revenue summary: %v", err)
	}
	out := &webv1.GetRevenueSummaryResponse{
		Result:   revenueResultToProto(res),
		FromUnix: sum.Window.FromUnix,
		ToUnix:   sum.Window.ToUnix,
	}
	if res != donaterevenue.OK {
		return out, nil
	}
	// The wire message flattens the money counters and the credit counters into
	// one RevenueTotals; the service keeps them apart because they are different
	// units read from different tables.
	out.Totals = &webv1.RevenueTotals{
		PaidOrders:     sum.Totals.PaidOrders,
		GrossCents:     sum.Totals.GrossCents,
		CreditsSold:    sum.Totals.CreditsSold,
		DistinctBuyers: sum.Totals.DistinctBuyers,
		CreatedOrders:  sum.Totals.CreatedOrders,
		PendingOrders:  sum.Totals.PendingOrders,
		PendingCents:   sum.Totals.PendingCents,
		ShopPurchases:  sum.Ledger.ShopPurchases,
		CreditsSpent:   sum.Ledger.CreditsSpent,
		ManualCredits:  sum.Ledger.ManualCredits,
		CreditsGranted: sum.Ledger.CreditsGranted,
	}
	out.ByMethod = make([]*webv1.RevenueByMethod, 0, len(sum.ByMethod))
	for _, m := range sum.ByMethod {
		out.ByMethod = append(out.ByMethod, &webv1.RevenueByMethod{
			PaymentMethod: paymentMethodToProto(m.PaymentMethod),
			PaidOrders:    m.PaidOrders,
			GrossCents:    m.GrossCents,
		})
	}
	out.Series = make([]*webv1.RevenuePoint, 0, len(sum.Series))
	for _, p := range sum.Series {
		out.Series = append(out.Series, &webv1.RevenuePoint{
			BucketStartUnix: unixOrZero(p.BucketStart),
			PaidOrders:      p.PaidOrders,
			GrossCents:      p.GrossCents,
			CreditsSold:     p.CreditsSold,
			DistinctBuyers:  p.DistinctBuyers,
		})
	}
	return out, nil
}

// ListTopupOrders returns one page of the order table.
func (s *DonateRevenueServer) ListTopupOrders(ctx context.Context, req *webv1.ListTopupOrdersRequest) (*webv1.ListTopupOrdersResponse, error) {
	res, rows, total, w, err := s.revenue.Orders(ctx, req.GetModeratorId(),
		req.GetWindow().GetFromUnix(), req.GetWindow().GetToUnix(),
		int16(req.GetStatus()), int16(req.GetPaymentMethod()), req.GetAccountId(),
		int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list topup orders: %v", err)
	}
	out := &webv1.ListTopupOrdersResponse{
		Result:     revenueResultToProto(res),
		TotalCount: int32(total),
		FromUnix:   w.FromUnix,
		ToUnix:     w.ToUnix,
		Orders:     make([]*webv1.TopupOrderRow, 0, len(rows)),
	}
	for _, r := range rows {
		out.Orders = append(out.Orders, &webv1.TopupOrderRow{
			Id:                r.ID,
			ExternalReference: r.ExternalReference,
			AccountId:         r.AccountID,
			AccountName:       r.AccountName,
			AccountEmail:      r.AccountEmail,
			PayerName:         r.PayerName,
			PayerCpfMasked:    r.PayerCPF, // already masked by the service
			Credits:           r.Credits,
			AmountCents:       r.AmountCents,
			PaymentMethod:     paymentMethodToProto(r.PaymentMethod),
			Provider:          r.Provider,
			Status:            topupStatusToProto(r.Status),
			CreatedAtUnix:     unixOrZero(r.CreatedAt),
			ConfirmedAtUnix:   unixPtrOrZero(r.ConfirmedAt),
		})
	}
	return out, nil
}

// ListTopBuyers ranks accounts by revenue in the window.
func (s *DonateRevenueServer) ListTopBuyers(ctx context.Context, req *webv1.ListTopBuyersRequest) (*webv1.ListTopBuyersResponse, error) {
	res, rows, total, w, err := s.revenue.TopBuyers(ctx, req.GetModeratorId(),
		req.GetWindow().GetFromUnix(), req.GetWindow().GetToUnix(),
		int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list top buyers: %v", err)
	}
	out := &webv1.ListTopBuyersResponse{
		Result:     revenueResultToProto(res),
		TotalCount: int32(total),
		FromUnix:   w.FromUnix,
		ToUnix:     w.ToUnix,
		Buyers:     make([]*webv1.TopBuyerRow, 0, len(rows)),
	}
	for _, b := range rows {
		out.Buyers = append(out.Buyers, &webv1.TopBuyerRow{
			AccountId:          b.AccountID,
			AccountName:        b.AccountName,
			AccountEmail:       b.AccountEmail,
			WindowPaidOrders:   b.WindowPaidOrders,
			WindowGrossCents:   b.WindowGrossCents,
			LifetimePaidOrders: b.LifetimePaidOrders,
			LifetimeGrossCents: b.LifetimeGrossCents,
			LifetimeCredits:    b.LifetimeCredits,
			FirstPaidAtUnix:    unixOrZero(b.FirstPaidAt),
			LastPaidAtUnix:     unixOrZero(b.LastPaidAt),
			DonateBalance:      b.DonateBalance,
		})
	}
	return out, nil
}

// ListDonateSpend returns one page of the donate wallet ledger.
func (s *DonateRevenueServer) ListDonateSpend(ctx context.Context, req *webv1.ListDonateSpendRequest) (*webv1.ListDonateSpendResponse, error) {
	res, rows, total, w, err := s.revenue.Spend(ctx, req.GetModeratorId(),
		req.GetWindow().GetFromUnix(), req.GetWindow().GetToUnix(),
		ledgerActionFromProto(req.GetAction()), req.GetAccountId(),
		int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list donate spend: %v", err)
	}
	out := &webv1.ListDonateSpendResponse{
		Result:     revenueResultToProto(res),
		TotalCount: int32(total),
		FromUnix:   w.FromUnix,
		ToUnix:     w.ToUnix,
		Entries:    make([]*webv1.DonateLedgerRow, 0, len(rows)),
	}
	for _, e := range rows {
		out.Entries = append(out.Entries, &webv1.DonateLedgerRow{
			Id:                 e.ID,
			Action:             ledgerActionToProto(e.Action),
			CreatedAtUnix:      unixOrZero(e.CreatedAt),
			SubjectAccountId:   e.SubjectAccountID,
			SubjectAccountName: e.SubjectAccountName,
			ActorAccountId:     e.ActorAccountID,
			ActorAccountName:   e.ActorAccountName,
			CreditsDelta:       e.CreditsDelta,
			BalanceAfter:       e.BalanceAfter,
			ShopItemId:         e.ShopItemID,
			ShopItemTitle:      e.ShopItemTitle,
			Reason:             e.Reason,
		})
	}
	return out, nil
}

// SearchAccounts resolves a login prefix to account identities.
func (s *DonateRevenueServer) SearchAccounts(ctx context.Context, req *webv1.SearchAccountsRequest) (*webv1.SearchAccountsResponse, error) {
	res, rows, err := s.revenue.SearchAccounts(ctx, req.GetModeratorId(), req.GetNamePrefix(), int(req.GetLimit()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search accounts: %v", err)
	}
	out := &webv1.SearchAccountsResponse{
		Result:   revenueResultToProto(res),
		Accounts: make([]*webv1.AccountSummary, 0, len(rows)),
	}
	for _, a := range rows {
		out.Accounts = append(out.Accounts, &webv1.AccountSummary{
			Id:            a.ID,
			Name:          a.Name,
			Email:         a.Email,
			DonateBalance: a.DonateBalance,
			Role:          a.Role,
			IsBlocked:     a.IsBlocked,
		})
	}
	return out, nil
}

// --- mappers ---

// revenueResultToProto maps the service outcome to the shared AdminResult.
func revenueResultToProto(r donaterevenue.Result) webv1.AdminResult {
	switch r {
	case donaterevenue.OK:
		return webv1.AdminResult_ADMIN_RESULT_OK
	case donaterevenue.Forbidden:
		return webv1.AdminResult_ADMIN_RESULT_FORBIDDEN
	case donaterevenue.NotFound:
		return webv1.AdminResult_ADMIN_RESULT_NOT_FOUND
	default:
		return webv1.AdminResult_ADMIN_RESULT_INVALID
	}
}

func revenueBucketFromProto(b webv1.RevenueBucket) donaterevenue.Bucket {
	switch b {
	case webv1.RevenueBucket_REVENUE_BUCKET_DAY:
		return donaterevenue.BucketDay
	case webv1.RevenueBucket_REVENUE_BUCKET_WEEK:
		return donaterevenue.BucketWeek
	case webv1.RevenueBucket_REVENUE_BUCKET_MONTH:
		return donaterevenue.BucketMonth
	default:
		return donaterevenue.BucketUnspecified
	}
}

func ledgerActionFromProto(a webv1.DonateLedgerAction) donaterevenue.LedgerAction {
	switch a {
	case webv1.DonateLedgerAction_DONATE_LEDGER_ACTION_PURCHASE:
		return donaterevenue.LedgerPurchase
	case webv1.DonateLedgerAction_DONATE_LEDGER_ACTION_CREDIT:
		return donaterevenue.LedgerCredit
	default:
		return donaterevenue.LedgerAny
	}
}

// ledgerActionToProto maps the stored audit action keyword to the wire enum.
func ledgerActionToProto(action string) webv1.DonateLedgerAction {
	switch action {
	case "purchase":
		return webv1.DonateLedgerAction_DONATE_LEDGER_ACTION_PURCHASE
	case "credit_balance":
		return webv1.DonateLedgerAction_DONATE_LEDGER_ACTION_CREDIT
	default:
		return webv1.DonateLedgerAction_DONATE_LEDGER_ACTION_UNSPECIFIED
	}
}

// paymentMethodToProto maps the stored method int (1=PIX, 2=CREDIT_CARD).
func paymentMethodToProto(m int16) webv1.PaymentMethod {
	switch m {
	case 1:
		return webv1.PaymentMethod_PAYMENT_METHOD_PIX
	case 2:
		return webv1.PaymentMethod_PAYMENT_METHOD_CREDIT_CARD
	default:
		return webv1.PaymentMethod_PAYMENT_METHOD_UNSPECIFIED
	}
}

// unixOrZero renders an instant as Unix seconds, mapping the zero time to 0
// rather than to the negative epoch that time.Time{}.Unix() would produce.
func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// unixPtrOrZero is unixOrZero for a nullable column (a PENDING order's
// confirmed_at), which the wire represents as 0.
func unixPtrOrZero(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return unixOrZero(*t)
}
