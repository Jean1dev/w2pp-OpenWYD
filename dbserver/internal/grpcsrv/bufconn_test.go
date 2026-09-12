package grpcsrv

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/internal/secret"
	"github.com/jeanluca/w2pp-openwyd/internal/store"
)

// TestServiceOverWire runs the real AccountService through a gRPC connection
// (bufconn) end to end against the in-memory fakeStore, proving the generated
// codec + registration wire up correctly (not just the method logic).
func TestServiceOverWire(t *testing.T) {
	hash, _ := secret.HashSecret("pw")
	fs := &fakeStore{
		byName: map[string]store.AccountAuth{"alice": {ID: 1, PassHash: hash}},
		chars:  map[int64][]domain.Character{1: {{Slot: 0, Name: "hero", Level: 9}}},
	}

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	dbv1.RegisterAccountServiceServer(srv, New(fs))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := dbv1.NewAccountServiceClient(conn)

	resp, err := client.AccountLogin(context.Background(),
		&dbv1.AccountLoginRequest{AccountName: "alice", Password: "pw"})
	if err != nil {
		t.Fatalf("AccountLogin over wire: %v", err)
	}
	if resp.GetResult() != dbv1.LoginResult_LOGIN_RESULT_OK || resp.GetAccountId() != 1 {
		t.Fatalf("unexpected: result=%v id=%d", resp.GetResult(), resp.GetAccountId())
	}

	list, err := client.ListCharacters(context.Background(), &dbv1.ListCharactersRequest{AccountId: 1})
	if err != nil {
		t.Fatalf("ListCharacters over wire: %v", err)
	}
	if len(list.GetCharacters()) != 1 || list.GetCharacters()[0].GetName() != "hero" {
		t.Fatalf("characters not returned: %+v", list.GetCharacters())
	}
}

func (f *fakeStore) RecordTrade(context.Context, domain.TradeRecord) error { return nil }

// ReserveSerials hands out consecutive blocks, so a test can assert that two
// reservations never overlap.
func (f *fakeStore) ReserveSerials(_ context.Context, quantos int64) (int64, error) {
	if f.serialErr != nil {
		return 0, f.serialErr
	}
	primeiro := f.serialProximo + 1
	f.serialProximo += quantos
	return primeiro, nil
}

func (f *fakeStore) RecordChat(_ context.Context, linhas []domain.ChatLinha) error {
	f.chat = append(f.chat, linhas...)
	f.lotes = append(f.lotes, len(linhas))
	return nil
}

func (f *fakeStore) RecordGround(_ context.Context, g domain.GroundEvent) error {
	f.chao = append(f.chao, g)
	return nil
}

func (f *fakeStore) RecordReport(_ context.Context, r domain.PlayerReport) error {
	f.reports = append(f.reports, r)
	return nil
}

func (f *fakeStore) SetCharacterPresence(_ context.Context, name string, online bool) (bool, error) {
	if f.presence == nil {
		f.presence = map[string]bool{}
	}
	f.presence[name] = online
	return true, nil
}

func (f *fakeStore) ClearAllPresence(context.Context) (int64, error) {
	var n int64
	for name, online := range f.presence {
		if online {
			f.presence[name] = false
			n++
		}
	}
	return n, nil
}

// shopPoints is the in-memory personal-shop wallet, keyed by account id. Credits
// accumulate here exactly as the real store accumulates them in Postgres, so a
// test can assert on the running balance and not just on the last call.
func (f *fakeStore) AddShopPoints(_ context.Context, accountID int64, delta int32, _, _ string) (int32, error) {
	if f.shopPoints == nil {
		f.shopPoints = map[int64]int32{}
	}
	f.shopPoints[accountID] += delta
	return f.shopPoints[accountID], nil
}

func (f *fakeStore) ShopPoints(_ context.Context, accountID int64) (int32, error) {
	return f.shopPoints[accountID], nil
}
