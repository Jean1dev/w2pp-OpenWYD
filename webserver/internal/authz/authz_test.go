package authz

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	webv1 "github.com/jeanluca/w2pp-openwyd/api/web/v1"
)

// servicosDeAdmin is the other half of the split, written out rather than
// inferred. Together with servicosDoJogador it has to name every service the
// .proto declares — see TestTodoServicoFoiClassificado.
//
// It exists only so that adding a service to the .proto fails a test until
// somebody decides which side it belongs on. The runtime does not read it: an
// unclassified service is administrative by default, so forgetting is safe in
// production and merely loud here.
var servicosDeAdmin = map[string]bool{
	"NpcAdminService":           true,
	"MobTemplateAdminService":   true,
	"AttributeMapAdminService":  true,
	"DonateAdminService":        true,
	"DailyRewardAdminService":   true,
	"WorldEventAdminService":    true,
	"DonateRevenueAdminService": true,
	"ItemStatAdminService":      true,
	"MountGrowthAdminService":   true,
}

// TestTodoServicoFoiClassificado walks the compiled .proto and demands a
// decision for every service in it.
//
// This is the test that keeps the hole from reopening. The failure it catches is
// nobody's mistake in particular: somebody adds a service, it works, it ships,
// and two years later it turns out the site could always call it.
func TestTodoServicoFoiClassificado(t *testing.T) {
	svcs := webv1.File_api_web_v1_web_proto.Services()
	if svcs.Len() == 0 {
		t.Fatal("nenhum serviço no descritor; o teste não está olhando o .proto certo")
	}
	for i := range svcs.Len() {
		nome := string(svcs.Get(i).Name())
		jogador, admin := servicosDoJogador[nome], servicosDeAdmin[nome]
		switch {
		case jogador && admin:
			t.Errorf("%s está nas duas listas; decida qual", nome)
		case !jogador && !admin:
			t.Errorf("%s não foi classificado. Em produção ele já nasce fechado "+
				"(só a chave do painel abre); este teste existe para você confirmar "+
				"que é isso mesmo, ou pôr ele na lista do jogador.", nome)
		}
	}
}

// TestSemChaveDeixaPassar is the migration step, and only that: the web-api has
// to be deployable to a running server before the keys exist on either side.
func TestSemChaveDeixaPassar(t *testing.T) {
	chamou := false
	_, err := Interceptor(Chaves{})(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/web.v1.NpcAdminService/DeleteNpc"},
		func(context.Context, any) (any, error) { chamou = true; return nil, nil })
	if err != nil || !chamou {
		t.Errorf("sem chave configurada devia deixar passar; err=%v chamou=%v", err, chamou)
	}
}

func TestChaves(t *testing.T) {
	c := Chaves{Painel: "chave-do-painel", Site: "chave-do-site"}
	casos := []struct {
		nome   string
		token  string
		metodo string
		quer   codes.Code
	}{
		{"painel abre a administração", "chave-do-painel", "/web.v1.NpcAdminService/DeleteNpc", codes.OK},
		{"painel abre o do jogador", "chave-do-painel", "/web.v1.RankingWebService/ListExpRanking", codes.OK},
		{"site abre o do jogador", "chave-do-site", "/web.v1.DonateShopService/Buy", codes.OK},
		{"site NÃO abre a administração", "chave-do-site", "/web.v1.NpcAdminService/DeleteNpc", codes.PermissionDenied},
		{"site não abre preço de item", "chave-do-site", "/web.v1.ItemStatAdminService/SetItemStat", codes.PermissionDenied},
		{"chave errada", "chute", "/web.v1.RankingWebService/ListExpRanking", codes.Unauthenticated},
		{"chave vazia", "", "/web.v1.RankingWebService/ListExpRanking", codes.Unauthenticated},
		// Serviço que este build não conhece: fechado, não aberto.
		{"serviço desconhecido é da administração", "chave-do-site", "/web.v1.ServicoDoFuturo/Faz", codes.PermissionDenied},
		{"método sem forma de método", "chave-do-site", "sem-barra-nenhuma", codes.PermissionDenied},
	}
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(context.Background(),
				metadata.Pairs(TokenHeader, caso.token))
			_, err := Interceptor(c)(ctx, nil,
				&grpc.UnaryServerInfo{FullMethod: caso.metodo},
				func(context.Context, any) (any, error) { return "ok", nil })
			if got := status.Code(err); got != caso.quer {
				t.Errorf("código = %s, queria %s (err=%v)", got, caso.quer, err)
			}
		})
	}
}

// TestSemMetadadoNaoPassa: a caller that sends nothing must be refused, not
// treated as the site.
func TestSemMetadadoNaoPassa(t *testing.T) {
	_, err := Interceptor(Chaves{Painel: "x"})(context.Background(), nil,
		&grpc.UnaryServerInfo{FullMethod: "/web.v1.RankingWebService/ListExpRanking"},
		func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("código = %s, queria Unauthenticated", status.Code(err))
	}
}

// TestChaveVaziaNaoAbreNada is the trap this guards: Site stays empty until the
// player site exists, and a naive comparison would let a caller sending an empty
// key in as the site.
func TestChaveVaziaNaoAbreNada(t *testing.T) {
	c := Chaves{Painel: "chave-do-painel"} // Site vazia, como hoje
	ctx := metadata.NewIncomingContext(context.Background(),
		metadata.Pairs(TokenHeader, ""))
	_, err := Interceptor(c)(ctx, nil,
		&grpc.UnaryServerInfo{FullMethod: "/web.v1.RankingWebService/ListExpRanking"},
		func(context.Context, any) (any, error) { return nil, nil })
	if status.Code(err) != codes.Unauthenticated {
		t.Errorf("uma chave vazia passou como se fosse a do site: %v", err)
	}
	// E o mesmo por espaços, que é o que um env var mal copiado produz.
	for _, branco := range []string{" ", "\t", "\n"} {
		if confere(branco, branco) {
			t.Errorf("a chave %q abriu; espaço em branco não é chave", branco)
		}
	}
}

func TestServicoDe(t *testing.T) {
	casos := map[string]string{
		"/web.v1.NpcAdminService/UpsertNpc": "NpcAdminService",
		"/SemPacote/Metodo":                 "SemPacote",
		"":                                  "",
		"/so-uma-parte":                     "",
	}
	for entrada, quer := range casos {
		if got := servicoDe(entrada); got != quer {
			t.Errorf("servicoDe(%q) = %q, queria %q", entrada, got, quer)
		}
	}
}
