package store

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/jeanluca/w2pp-openwyd/internal/migrations"
)

// tabelasDeTeste is every table the migrations create, dropped before an
// integration run so it starts from an empty database.
//
// It has to list them ALL: `DROP TABLE account CASCADE` drops dependent
// *constraints*, not dependent *tables*, so a table omitted here survives and
// the next Migrate fails with 42P07 (relation already exists) — pointing at
// whichever migration happens to create it, which is never the one at fault.
//
// Roughly child-before-parent, though CASCADE in a single statement makes the
// order a courtesy rather than a requirement. schema_migrations is last and is
// not created by a migration file; migrate.go makes it.
var tabelasDeTeste = []string{
	"shop_points_audit",
	"shop_points",
	"combine_tag",
	"combine_rate_meta",
	"combine_band",
	"combine_rate",
	"chat_log_meta",
	"chat_log",
	"item_serial_seq",
	"item_census_meta",
	"item_census",
	"ground_log",
	"player_report",
	"trade_log",
	"item_stat",
	"admin_audit_log",
	"sapphire_balance",
	"world_event_audit",
	"world_event_meta",
	"world_event_config",
	"npc_audit",
	"npc_shop_item",
	"npc_definition",
	"item_price",
	"npc_config_meta",
	"mob_template_equip",
	"mob_template_stat",
	"mount_bonus",
	"mount_absorb",
	"mount_growth_rate",
	"xp_rule_meta",
	"xp_rule",
	"quest_reward_meta",
	"quest_reward",
	"drop_bonus_meta",
	"drop_bonus",
	"dungeon_gate_meta",
	"dungeon_gate",
	"spawn_rate_meta",
	"spawn_rate",
	"combat_rule_meta",
	"combat_rule",
	"npc_generator_off_meta",
	"npc_generator_off",
	"drop_rule_meta",
	"drop_rule",
	"affect",
	"item",
	"character_pvp_stats",
	"character",
	"castle_quest_state",
	"guild_tower_state",
	"guild_zone",
	"guild_relation",
	"guild_member",
	"guild",
	"donate_topup_order",
	"donate_payer_profile",
	"account",
	"donate_shop_audit",
	"donate_shop_item",
	"daily_reward_audit",
	"daily_reward_claim",
	"daily_reward_item",
	"delivery_queue",
	"schema_migrations",
}

// Sobre o número da migração: ele não é a identidade. migrate.go grava o nome
// inteiro do arquivo em schema_migrations, e nove números já estão repetidos
// neste repositório desde antes — 0007, 0008, 0012, 0013, 0024, 0025, 0030.
// Todas rodam, em ordem alfabética do nome completo. Renumerar uma que já
// rodou em produção faria o servidor ver uma versão nova e aplicar o CREATE
// TABLE de novo, então repetido fica repetido; o número novo vale só para
// migração que ainda não subiu.

var criaTabela = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-z_][a-z0-9_]*)`)

// TestResetTestSchemaCobreTodasAsTabelas keeps tabelasDeTeste honest.
//
// It runs without a database, and on purpose: the list has fallen behind the
// migrations three times, and each time the cost landed on whoever next ran the
// integration suite, as a 42P07 blaming an innocent migration. This is the
// cheap check that catches it on the commit that adds the table.
func TestResetTestSchemaCobreTodasAsTabelas(t *testing.T) {
	dropadas := make(map[string]bool, len(tabelasDeTeste))
	for _, nome := range tabelasDeTeste {
		if dropadas[nome] {
			t.Errorf("%q aparece duas vezes na lista", nome)
		}
		dropadas[nome] = true
	}

	nomes, err := fs.Glob(migrations.FS, "*.up.sql")
	if err != nil {
		t.Fatalf("listar migrações: %v", err)
	}
	criadas := map[string]string{} // tabela → migração que a cria
	for _, n := range nomes {
		sql, err := fs.ReadFile(migrations.FS, n)
		if err != nil {
			t.Fatalf("ler %s: %v", n, err)
		}
		for _, m := range criaTabela.FindAllStringSubmatch(string(sql), -1) {
			tabela := strings.ToLower(m[1])
			if _, visto := criadas[tabela]; !visto {
				criadas[tabela] = n
			}
		}
	}
	if len(criadas) == 0 {
		t.Fatal("nenhum CREATE TABLE encontrado; a varredura das migrações quebrou")
	}

	for tabela, migracao := range criadas {
		if !dropadas[tabela] {
			t.Errorf("%s cria a tabela %q e ela não está em tabelasDeTeste; "+
				"a próxima rodada de integração vai falhar com 42P07", migracao, tabela)
		}
	}
	for _, nome := range tabelasDeTeste {
		if nome == "schema_migrations" {
			continue // criada por migrate.go, não por um arquivo de migração
		}
		if _, ok := criadas[nome]; !ok {
			t.Errorf("tabelasDeTeste dropa %q, que nenhuma migração cria", nome)
		}
	}
}
