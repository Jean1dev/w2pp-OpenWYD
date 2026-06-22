# `_MSG_MessageWhisper` (0x0334) — enumeração de comandos

> Detalha o "console de comandos" embutido no sussurro (resumo em [lote2-chat.md](lote2-chat.md)).
> Fonte: `Source/Code/TMSrv/_MSG_MessageWhisper.cpp` (1710 linhas). Linhas citadas conferidas via
> `#pragma region`. **Despacho:** o handler compara `m->MobName` (o "destinatário" do sussurro) com
> palavras-chave; se casar, executa o comando e **retorna**; senão, trata como sussurro normal
> (PM) para o personagem de nome `MobName`.

## Modelo de autorização (importante)
- **Não há campo de "Authority".** Os comandos de GM são gated por **`pMob[conn].MOB.CurrentScore.Level >= 1000`**
  (personagens GM têm nível inflado). Ex.: `/gm` e `/cmd` chamam `ProcessImple(conn, Level-1000, String)`
  (`:1073`,`:1668`) — o **interpretador de GM real** vive em `imple.cpp` (com gating próprio de
  admin/IP via `admin.txt`). `/not`, `/summon` (modo admin) e `/relo` também exigem `Level>=1000`.
- **Pré-condição comum:** `pUser[conn].Mode == USER_PLAY`. Chat (não-comando) também checa
  `pUser[conn].MuteChat` (`:1422`).

---

## Comandos utilitários do jogador (sem gate especial)
| Cmd | Linha | Efeito |
|-----|------:|--------|
| `cp` | 33 | mostra "pontos de chão"/PK (`GetPKPoint-75`). |
| `buffs` | 41 | **limpa todos os `Affect`** do personagem (zera buffs) + `SendScore`. |
| `getout` | 56 | encerra cidadania (`extra.Citizen = 0`). |
| `srv <n>` | 67 | troca de servidor/canal (`1..MAX_SERVER`). |
| `index` | 96 | (info de índice — relay; ver fonte). |
| `spk` / `gritar` | 114 | chat global **cross-server** ("falar para todos os servers"). |
| `qst` | 164 | mostra info de quest (`QuestInfo`). |
| `nt` | 512 | mostra contador `extra.NT` (`_DN_CHANGE_COUNT`). |
| `nig` | 521 | hora atual formatada `!!HHMMSS` (timer "Pesadelo"). |
| `day` | 602 | envia `"!#11  2"` (sinal de skill/dia ao cliente). |
| `online` | 610 | conta jogadores em `USER_PLAY` e responde "Somos N jogador(es) online." |
| `time` | 1056 | data/hora do servidor (`dd-mm-YYYY HH:MM:SS`). |
| `donate` | 1390 | mostra saldo `pUser[conn].Donate`. |
| `tab` | 538 | troca de aba/classe — gate de nível (ex.: `<69` MORTAL → "Level Limit 70"). |

## Teleportes (gratuitos; jitter `rand()%3`)
| Cmd | Linha | Destino / regra |
|-----|------:|------|
| `red` | 703 | ~(1744,1880). |
| `blue` | 711 | ~(1745,1573). |
| `arch` | 719 | ~(1706,1723). |
| `selados` | 727 | ~(1843,3652). |
| `amagos` | 734 | ~(3910,2878). |
| `armia` | 741 | ~(2100,2100) — **bloqueado** se `RvRState`/`GTorreState`/`CastleState` ativos (`_NN_TP_DENY`). |
| `azran` | 758 | idem armia (bloqueio por evento). |
| `torre` | 775 | idem (bloqueio por evento). |
| `erion` | 800 | idem (bloqueio por evento). |
| `gelo` | 817 | idem (bloqueio por evento). |
| `kefra` | 834 | idem (bloqueio por evento). |
| `agua` | 857 | teleporte (água). |
| `kingdom` | 868 | teleporte por **Clan** (7/8 → posições distintas; senão neutro). |
| `king` | 883 | teleporte por Clan (entrada do "rei"). |

> **Risco:** teleportes livres por comando de chat (mobilidade instantânea) — coordenadas/regras
> hardcoded. Na migração, mover para config e revisar se devem mesmo ser gratuitos/abertos.

## Guilda
| Cmd | Linha | Pré-condições | Efeito |
|-----|------:|---------------|--------|
| `create <nome>` | 186 | sem guilda; `Coin >= 100.000.000`; `Clan` 7 ou 8 | cria guilda (debita gold). |
| `subcreate <char> <nome>` | 300 | tem guilda | define sub-líder (`GetUserByName`; relay `MSG_GuildInfo` ao DBSrv). |
| `sair`/`expulsar`/`abandonar` | 395 | tem guilda | sai da guilda; se sub-líder (lvl 6–8) limpa `GuildInfo.SubN` e **avisa o DBSrv**; zera `Guild`/`GuildLevel`; `GridMulticast` da tag. |
| `handover <char>` | 444 | líder (`GuildLevel==9`); alvo `USER_PLAY` na mesma guilda | **transfere a liderança** da guilda. |
| `summonguild` | 897 | líder (`GuildLevel>0`); em vila; atributo de mapa permitido | invoca membros da guilda (`SummonGuild2`). |
| `fimguerra` | 1216 | (líder) | encerra guerra de guilda. |
| `fimirma` | 1273 | (líder) | encerra aliança ("irmandade"). |

## Desbloqueios de progressão (⚠ sem gate de permissão)
| Cmd | Linha | Efeito |
|-----|------:|--------|
| `destravar40` | 627 | seta `extra.QuestInfo.Celestial.Lv40 = 1` + "Processing Complete" + `_MSG_CombineComplete`. |
| `destravar90` | 644 | seta `Celestial.Lv90 = 1` (idem). |
| `arcana` | 675 | concede item `sIndex 3507` em `Equip[1]`, seta `QuestInfo.Circle = 1`, emoção. |

> **🔴 Achado de segurança:** estes três concedem progressão/itens **sem nenhuma checagem de
> permissão nem de pré-requisito de quest** — qualquer jogador que envie o comando recebe o
> desbloqueio. São prováveis **atalhos de teste/dev** (ou recompensas que deveriam ser disparadas só
> pelo fluxo de quest). **Revisar/remover** na migração; hoje são backdoors de progressão.

## Cash / conta (relay → DBSrv)
| Cmd | Linha | Efeito |
|-----|------:|--------|
| `pin <code>` | 1330 | ativa PIN code (`MSG_DBActivatePinCode`); exige `strlen==36`; **anti-brute: 3 tentativas / 2h** (`DonateInfo`). |
| `contaprincipal` | 1398 | marca conta principal por MAC/IP (`MSG_DBPrimaryAccount`). |
| `not <msg>` | 1374 | **GM (`Level>=1000`)** — broadcast de aviso global (`MSG_DBNotice`). |

## GM / admin (`Level >= 1000`)
| Cmd | Linha | Efeito |
|-----|------:|--------|
| `gm` / `GM <args>` | 1073 | → `ProcessImple(conn, Level-1000, String)` — **console de GM** (`imple.cpp`). |
| `cmd` / `CMD <args>` | 1668 | idem `/gm` (alias). |
| `summon` | 934 | invoca jogador/alvo; caminho admin quando `Level>=1000`. |
| `relo` | 1082 | `_NN_Relocate` — relocar (mover jogador/objeto). |
| `expulsar` | 1197 | `_NN_Deprivate` — expulsar/privar (deprivate). |

> Os comandos de GM reais (kick, criar item, etc.) ficam em **`ProcessImple`/`imple.cpp`** — auditar
> à parte (subsistema próprio; gating por `Level>=1000` + `admin.txt` por IP).

## Canais de chat (prefixo no início de `String`, não em `MobName`)
| Prefixo | Linha | Canal |
|--------:|------:|-------|
| `-` | 1425 | **chat de guilda**. |
| `=` | 1471 | **chat de party** (usa `pMob[conn].Leader`). |
| `@@` | 1515 | **chat de reino** (kingdom) — com rate-limit (`pUser.Message`/`GetTickCount`). |
| `@` | 1542 | **chat de cidadão** — idem rate-limit. |
| (nenhum) | 1570 | **PM**: sussurro normal ao personagem `MobName`; `/r` responde ao último. Gate: `MuteChat`. |

---

## Notas de migração
- **Separar em 3 superfícies:** (a) comandos de jogador, (b) comandos de GM (→ módulo `imple`), (c)
  chat/PM/canais — cada uma com autorização explícita. Hoje tudo está fundido neste handler.
- **Remover/auditar os backdoors de progressão** (`destravar40/90`, `arcana`) e revisar teleportes
  livres.
- **`Level>=1000` como "é GM" é frágil** (depende de dado de personagem). Na stack nova, usar um campo
  de papel/role explícito + autenticação, não o nível.
- **Relays ao DBSrv** (`pin`, `not`, `contaprincipal`, `subcreate`, `sair`) viram chamadas gRPC ao
  `dbServer` (Fase 9 §3.5).
- Coordenadas de teleporte, custos de guilda e índices de item (3507, etc.) → configuração.

> **Status:** enumeração dos **55 `#pragma region`** completa (gatilho + gate + efeito). Detalhes
> multi-etapa de alguns blocos longos (`create`, `subcreate`, `handover`, `summon`, `fimguerra`,
> `fimirma`) resumidos — abrir a fonte para o passo-a-passo fino quando for implementar.
