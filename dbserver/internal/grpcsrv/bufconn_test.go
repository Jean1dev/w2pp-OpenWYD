package grpcsrv

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	dbv1 "github.com/jeanluca/w2pp-openwyd/api/db/v1"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/convert"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/domain"
	"github.com/jeanluca/w2pp-openwyd/dbserver/internal/store"
)

// TestServiceOverWire runs the real AccountService through a gRPC connection
// (bufconn) end to end against the in-memory fakeStore, proving the generated
// codec + registration wire up correctly (not just the method logic).
func TestServiceOverWire(t *testing.T) {
	hash, _ := convert.HashSecret("pw")
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
