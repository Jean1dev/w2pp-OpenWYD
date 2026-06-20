# Fase 7 — Configuração e Operação (w2pp-OpenWYD)

> Catálogo de TODOS os arquivos de config em `Release/`, topologia de deploy, IPs/portas hardcoded e
> o que deve virar variável de ambiente/secret na stack nova.

---

## 1. Topologia de deploy

**3 processos** (todos Win32), tipicamente na mesma máquina ou LAN:

```
            ┌───────────────┐         ┌──────────────┐
 WYD.exe ──►│ TMSrv  :8281  │────────►│ DBSrv :7514  │──► arquivos ./account/*
 (cliente)  │ (jogo)        │         │ (persistência)│
            └──────┬────────┘         └──────┬───────┘
                   │ :3000 (billing)          │ (NPServer / contas externas)
                   ▼                          ▼
            ┌──────────────┐          ┌──────────────┐
            │ BISrv        │          │ NPTool/NPSrv │
            │ (billing)    │          └──────────────┘
            └──────────────┘
```

- **Portas hardcoded** (`Basedef.h:104-105`): `GAME_PORT = 8281` (cliente→TMSrv),
  `DB_PORT = 7514` (TMSrv→DBSrv). Billing em `:3000` (de `biserver.txt`).
- **Auto-restart por `.bat`:** ex. `DBsrv/run/DBsrv.bat` é um loop `:Loop … DBsrv.exe … goto Loop`
  — reinicia o processo se ele cair. (Há `.bat` equivalente para o TMSrv.) Substituir por
  supervisor real (systemd/k8s/Docker restart policy) na stack nova.
- `deletar Logs.bat` na raiz de `Release/` — housekeeping manual de logs.

---

## 2. Catálogo de configuração

### 2.1. `TMsrv/run/gameconfig.txt` (config principal do jogo)

Parseado posicionalmente por seção. Campos confirmados (valores do exemplo):

| Seção | Campo | Exemplo | Efeito |
|-------|-------|--------:|--------|
| Drop Item Event | `evindex/evdelete/evon/evitem/evrate/evstart` | 0/0/0/0/0/0 0 0 1 | evento de drop global (Fase 4 §2.3) |
| Etc Event | `double` | 200 | exp/drop dobrado (DOUBLEMODE) |
| | `deadpoint` | 1 | penalidade de morte |
| | `dungeonevent` | 1 | evento de dungeon |
| | `statsapphire` | 30 | safira/stats |
| | `battleroyal` | 0 | BR on/off (`BrState`) |
| Billing | `billmode` | 0 | modo de billing (`BILLING`) |
| | `freeexp` | 35 | nível até onde é grátis (`FREEEXP`) |
| | `charselbill` | 0 | cobrança na seleção |
| | `potioncount` | 10 | limite de poções |
| | `partybonus` | 100 | bônus de party (`PARTYBONUS`, Fase 4 §1.2) |
| | `guildboard` | 0 | quadro de guilda |
| Item Drop Bonus | matriz 8×8 (`g_pDropBonus[]`) | 50… | bônus de drop por slot (Fase 4 §2.2) |
| Treasure | tabela | 0… | `g_pTreasure[8]` |
| Etc Settings | `partydif` | 200 | diferença de nível p/ party (`PARTY_DIF`) |
| | `kefrastatus` | 156 | estado boss Kefra (`KefraLive`) |
| | `GTorreHour` | 22 | hora da Guerra de Torre |
| | `RVRHour` | 23 | hora do RvR |
| | `DropItem` | 0 | drop habilitado (`isDropItem`) |
| | `BRHour` | 19 | hora do Battle Royal |
| | `maxNightmare` | 3 | limite de pesadelo |
| | `PotionDelay` | 100 | cooldown de poção (ms) |

### 2.2. Rede / IPs

| Arquivo | Conteúdo do exemplo | Significado |
|---------|---------------------|-------------|
| `TMsrv/run/localip.txt` | `192.168.18.12` | IP de bind do TMSrv |
| `DBsrv/run/localip.txt` | `192.168.18.12` | IP de bind do DBSrv |
| `TMsrv/run/biserver.txt` | `54.207.102.145 3000` | **IP+porta do BISrv** (billing) — IP público AWS! |
| `TMsrv/run/serverlist.txt` | `0 0 192.168.18.12` / `0 1 …` | `<grupo> <canal> <ip>` dos canais |
| `DBsrv/run/serverlist.txt` | idem | mesma lista (DB) |
| `*/serverlist.bin` | binário | versão compilada da serverlist (enviada ao cliente) |
| `DBsrv/run/redirect.sample.txt` | `192.168.18.12 8895 admineu admineu1` | redirect/admin (host porta user pass) |

### 2.3. Admin / segurança / MAC

| Arquivo | Exemplo | Significado |
|---------|---------|-------------|
| `*/admin.txt` | `0 192.168.18.12` | `<nível?> <IP autorizado a admin>` |
| `DBsrv/run/Mac.txt` | `0 1060404036.-1187033502.…` | MAC bloqueado/autorizado (`STRUCT_BLOCKMAC.Mac[4]`) |
| `TMsrv/run/admin.txt` | `0 192.168.18.12` | idem TMSrv |

> Comandos de admin chegam por `_MSG_MessageChat` (comandos `/`) e são autorizados por IP
> (`admin.txt`) — ver handler `_MSG_MessageChat` (Fase 5, lote 2).

### 2.4. DBSrv

| Arquivo | Exemplo | Significado |
|---------|---------|-------------|
| `DBsrv/run/config.txt` | `Sapphire 1` / `LastCapsule 0` | estado global (safira/cápsula) |
| `DBsrv/run/settings.txt` | URLs + strings de UI (launcher) | textos do cliente/launcher (i18n) |
| `DBsrv/run/Server.txt` | `0` | `ServerIndex` |

### 2.5. Conteúdo / regras (referência cruzada)

Carregados na inicialização (detalhados nas Fases 2 e 4): `ItemList.csv/.bin`, `SkillData.csv`,
`NPCGener.txt`, `HeightMap.dat`, `AttributeMap.dat`, `ItemDropList.txt`, `LevelItem.txt`,
`Regions.txt`, `Guard.txt`, `InitItem.csv`, `QuestDiaria.txt`, `Language.txt`, e
`Settings/{CompRate,SancRate,CastleQuest,QuestsRate,MobMerc}.txt`.

### 2.6. Estado persistente de mundo (escrito em runtime)

`account/`, `Guilds.txt`, `GuildInfo`, `Guild_<x>_<y>.txt`, `ChampionCity_<x>_<y>.txt`,
`Chall_<x>_<y>.txt`, `Ranking.txt`, `Guild.txt`, `Chall.txt` (estado de zona/guerra).

---

## 3. Hardcodes que precisam virar config/ambiente

| Hardcode | Local | Vira |
|----------|-------|------|
| `GAME_PORT = 8281`, `DB_PORT = 7514` | `Basedef.h:104-105` | env/config |
| `BillServer 54.207.102.145:3000` | `biserver.txt` | env (host/porta) |
| `S:/export/account%d/...` | `CFileDB.cpp:2513` | path configurável |
| `./account/<Key>/<NAME>` | `CFileDB.cpp:2436` | base path configurável |
| IPs em `localip/admin/serverlist` | configs | env/secret |
| Nomes reservados DOS (`COM*/LPT*`) | `CFileDB.cpp:2425` | irrelevante fora de Windows (remover) |

---

## 4. O que vira variável de ambiente / secret na stack nova

**Env (config):**
- `GAME_PORT`, `DB_HOST`, `DB_PORT`, `BILLING_HOST`, `BILLING_PORT`, `SERVER_INDEX`, `SERVER_GROUP`.
- Caminhos: `DATA_DIR` (conteúdo), `ACCOUNT_STORE` (DB/arquivos), `LOG_DIR`, `EXPORT_DIR`.
- Flags de evento: `DOUBLE_EXP`, `FREE_EXP_LEVEL`, `GTORRE_HOUR`, `RVR_HOUR`, `BR_HOUR`,
  `DROP_ENABLED`, `PARTY_BONUS`, `PARTY_DIF`, `MAX_NIGHTMARE`, `POTION_DELAY` (todos do
  `gameconfig.txt`).

**Secret:**
- Credenciais de DB/conexão entre serviços.
- Chave de assinatura de sessão (substituir INITCODE/keytable estática — Fase 1/9).
- Credenciais de billing/redirect (`redirect.sample.txt` tem user/pass em claro → secret).
- `Mac.txt`/listas de admin → mover para storage seguro, não arquivo plano versionado.

---

## 5. Operação (runbook resumido)

1. Subir DBSrv (porta 7514), depois TMSrv (conecta ao DB, escuta 8281), depois BISrv (3000).
2. `.bat` de auto-restart hoje; migrar para supervisor com health-check e backoff.
3. Backup: copiar `account/` + `Guilds*`/estado de zona (são os dados vivos). Na stack nova: backup
   do banco.
4. Logs: `fLogFile`/`fChatLogFile`/`fItemLogFile` (TMSrv) — centralizar (stdout/JSON) na migração.

> **Status da Fase 7: COMPLETO** para o catálogo de config e topologia. Parsing posicional exato de
> `gameconfig.txt`/`Treasure`/`Guild.txt` deve ser confirmado contra `CReadFiles.cpp` ao
> reimplementar o loader.
