# BeastMaster skills 48..71

## Identidade mecanica

BeastMaster combina dano magico, debuffs, protecao elemental, summons e transformacoes. O Go cobre dano, buffs/debuffs, Chamas Etereas, Aura Bestial, summons e transformacoes com score derivado, sem gravar buffs no score persistido.

Evidencia usa os codigos definidos em `README.md`.

## Catalogo

| Índice | Nome | Árvore | Tipo | Target | Mana | SkillPoint | Instance | Affect/Tick | Fórmula/Efeito | Status | Evidência |
|-------:|------|--------|------|--------|-----:|-----------:|----------|-------------|----------------|--------|-----------|
| 48 | Fera_Flamejante | 1 | dano | 1 | 8 | 24 | 2/25 | - | dano elemental type 2 base 25 | IMPLEMENTED | CSV+CAST+DMG |
| 49 | Chamas_Etéreas | 1 | utilitário | 1 | 35 | 36 | 12/0 | - | mana burn ou dispel de 14/16/18/19 | IMPLEMENTED | CSV+CAST+LEGACY |
| 50 | Som_das_Fadas | 1 | dano | 1 | 20 | 69 | 4/115 | - | dano elemental type 4 base 115 | IMPLEMENTED | CSV+CAST+DMG |
| 51 | Enfraquecer | 1 | debuff | 3 | 35 | 45 | 0/0 | A10/10/1 | debuff de dano type 10 | IMPLEMENTED | CSV+AFF+SCORE |
| 52 | Furia_de_Gaia | 1 | dano | 1 | 25 | 75 | 4/220 | - | dano elemental type 4 base 220 | IMPLEMENTED | CSV+CAST+DMG |
| 53 | Proteção_Elemental | 1 | buff | 0 | 105 | 78 | 0/0 | A25/10/150 | resist type 25 nos indices 0/1/3 (Fogo/Gelo/Trovao) — diverge do legado, issue #233 | IMPLEMENTED | CSV+AFF+SCORE |
| 54 | Aura_Bestial | 1 | buff | 0 | 128 | 90 | 0/0 | T23/160 | tick 23 dispara Furia de Gaia sintetica | IMPLEMENTED | CSV+CAST+AFF+LEGACY |
| 55 | Espírito_Vingador | 1 | dano | 6 | 160 | 239 | 4/180 | - | dano elemental type 4 base 180 | IMPLEMENTED | CSV+CAST+DMG |
| 56 | Evocar_Condor | 2 | summon | 0 | 30 | 33 | 11/1 | - | evoca summon value 1, count/party/vida | IMPLEMENTED | CSV+SUMMON+LEGACY |
| 57 | Evocar_Javali_Selvagem | 2 | summon | 0 | 55 | 51 | 11/2 | - | evoca summon value 2, count/party/vida | IMPLEMENTED | CSV+SUMMON+LEGACY |
| 58 | Evocar_Lobo | 2 | summon | 0 | 75 | 72 | 11/3 | - | evoca summon value 3, count/party/vida | IMPLEMENTED | CSV+SUMMON+LEGACY |
| 59 | Evocar_Grande_Tigre | 2 | summon | 0 | 90 | 72 | 11/4 | - | evoca summon value 4, count/party/vida | IMPLEMENTED | CSV+SUMMON+LEGACY |
| 60 | Evocar_Urso_Selvagem | 2 | summon | 0 | 120 | 84 | 11/5 | - | evoca summon value 5, count/party/vida | IMPLEMENTED | CSV+SUMMON+LEGACY |
| 61 | Evocar_Gorila_Gigante | 2 | summon | 0 | 130 | 93 | 11/6 | - | evoca summon value 6, count/party/vida | IMPLEMENTED | CSV+SUMMON+LEGACY |
| 62 | Evocar_Dragão_Negro | 2 | summon | 0 | 155 | 102 | 11/7 | - | evoca summon value 7, count/party/vida | IMPLEMENTED | CSV+SUMMON+LEGACY |
| 63 | Evocar_Succubus | 2 | summon | 0 | 240 | 220 | 11/8 | - | evoca summon value 8, count/party/vida | IMPLEMENTED | CSV+SUMMON+LEGACY |
| 64 | Lobisomem | 3 | buff | 0 | 72 | 33 | 0/0 | A16/1/150 | transform Lobo mesh/score/speed/crit | IMPLEMENTED | CSV+AFF+SCORE+TRANSFORM |
| 65 | Coração_de_Lobo | 3 | passivo | 0 | 0 | 45 | 0/0 | - | bonus passivo do Lobo | IMPLEMENTED | CSV+SCORE+TRANSFORM |
| 66 | Homem-Urso | 3 | buff | 0 | 144 | 66 | 0/0 | A16/2/150 | transform Urso mesh/score/speed | IMPLEMENTED | CSV+AFF+SCORE+TRANSFORM |
| 67 | Escudo_do_Tormento | 3 | passivo | 0 | 0 | 45 | 0/0 | - | bonus passivo do Urso | IMPLEMENTED | CSV+SCORE+TRANSFORM |
| 68 | Astaroth | 3 | buff | 0 | 260 | 99 | 0/0 | A16/3/150 | transform Astaroth mesh/score/speed/crit | IMPLEMENTED | CSV+AFF+SCORE+TRANSFORM |
| 69 | Asas_do_Inferno | 3 | passivo | 0 | 0 | 45 | 0/0 | - | bonus passivo do Astaroth | IMPLEMENTED | CSV+SCORE+TRANSFORM |
| 70 | Titã | 3 | buff | 0 | 300 | 132 | 0/0 | A16/4/150 | transform Tita mesh/score/speed | IMPLEMENTED | CSV+AFF+SCORE+TRANSFORM |
| 71 | Éden | 3 | buff | 0 | 300 | 212 | 0/0 | A16/5/150 | transform Eden mesh/score/speed/crit | IMPLEMENTED | CSV+AFF+SCORE+TRANSFORM |

## Regras comuns

- Dano BM usa a mesma formula de caster da Foema: `Int/30 + Int/3 + Level + base + 2*Special`, Magic multiplier e 5/4.
- Summons usam `InstanceType 11`, `InstanceValue 1..8`, `pSummonBonus` local, `Special[2]` para quantidade/escalonamento, `PartyList` para limite e `MSG_CNFAddParty` para UI.
- Transformacoes usam affect type 16, valores 1..5, `pTransBonus`, learned bits 17/19/21 para flat adds de Lobo/Urso/Astaroth, mesh 22/23/24/25/32, critical e attack/run speed.

### Ciclo de vida do pet (issue #234)

O `PartyList` do lider **e** o unico registro de pets: `generateSummon` conta por ele e
`commandSummons` acha os pets por ele. Logo todo caminho que zera slot de party tem de matar o pet
que estava ali, senao ele sobrevive fora de qualquer lista — e a proxima evocacao conta zero e
empilha um novo conjunto (o bug relatado em #234).

Quem remove pet, e por que:

| Gatilho | Onde | Evidencia |
|---|---|---|
| membro sai / e expulso | `handler/party.go` `leaveParty`/`kickPartyMember` → `despawnSummonsOf(leader, conn)` | `Server.cpp:8185-8190` |
| lider sai (party dissolvida) e **BM solo** clicando sair | `leaderLeaveParty` → `despawnSummonsOf(leader, 0)` | `Server.cpp:8237-8243` |
| logout / desconexao | `Dispatcher.SessionEnd` (`characterLogout` + `world.SetSessionEndHandler`) | `_MSG_CharacterLogout.cpp:23`, `Server.cpp:7654` |
| dono morto / fora de `USER_PLAY` | `summonTick` | `ProcessSecMinTimer.cpp:2381` |
| pet fora do `PartyList` do seu lider (rede de seguranca) | `summonTick` + `petIsListed` | `CMob.cpp:118-124` |
| fim do affect type 24 | `summonTick` | `Server.cpp:5843` |
| montaria bebe desequipada/morta | `removeBabyMountSummons` | `Server.cpp:4650-4665` |

Duas notas de paridade:

- **Divergencia deliberada:** ao dissolver a party o Go mata os pets de **todos** os membros. O
  legado deleta apenas os do lider e so zera `Summoner` nos demais (`Server.cpp:8242`), contando com
  a varredura global do affect 24 (`Server.cpp:5843`) para recolher o resto. Aqui a contagem de vida
  roda dentro de `summonTick`, que exige `Summoner != 0` (`mobai.go:61`), entao copiar essa linha
  vazaria mobs sem dono para sempre.
- `DespawnMob` so envia `MSG_RemoveMob`; a linha de party do pet precisa de `MSG_RemoveParty`
  explicito (no legado vem de graca via `DeleteMob → RemoveParty`, `Server.cpp:7840`).

O orcamento de pets e **da party inteira**, nao por jogador: `generateSummon` conta todo pet no
`PartyList` do lider, de quem for (`Server.cpp:2991-2997`).
- O score derivado de transform fica em caches de affect/read-time, nao no score persistido.

## Divergencias deliberadas

- `Proteção_Elemental` (53) buffa os indices `Resist[0]`, `[1]` e `[3]` — Fogo, Gelo e Trovao —
  pulando `[2]` (Sagrado). O legado (`Basedef.cpp:4239`) soma em locais chamados `Fogo/Trovao/Gelo`,
  mas esses locais estao trocados na origem (`Basedef.cpp:3919` liga `Sagrado←Resist[0]`,
  `Trovao←[1]`, `Fogo←[2]`, `Gelo←[3]`), entao os indices que ele realmente toca sao `1/2/3`:
  buffa Sagrado e deixa o Fogo de fora. A ordem real dos elementos vem do conteudo —
  `Orb_de_Fogo` carrega `EF_RESIST1` e `Orb_Sagrada` carrega `EF_RESIST3`
  (`Release/Common/ItemList.csv:1013,1019`), e `EF_RESIST1..4` mapeiam para `Resist[0..3]`
  (`Source/Code/TMSrv/CMob.cpp:640-643`) → `[0]`=Fogo, `[1]`=Gelo, `[2]`=Sagrado, `[3]`=Trovao.
  Reportado na issue #233 (print do cliente: Fogo parado em 0, os outros tres em +27 com mastery
  Elemental 255). Mesmo espirito da divergencia de duracao de buff em `world.AffectDuration` (issue #229).
  Testes: `TestElementalProtectionResists`.

## Lacunas e riscos

- Glow `EF_SANC` da transformacao continua visualmente deferido: `MSG_UpdateEquip`/`MSG_CreateMob` locais carregam apenas indices visuais, nao efeitos do item de corpo.
- `Aura_Bestial` evita PvP automatico pelo mesmo motivo de `Trovão`: `PKMode`/arena attrs ainda nao existem no modelo Go.
- O centro do tick 23 usa a posicao atual do caster; o legado lia `TargetX/TargetY`, que no Go nao existe separado apos o movimento destino-autoritativo.

## Testes minimos esperados

- `TestEtherealFlameBurnsPlayerMP`, `TestBeastAuraTickUsesSkill52`.
- `TestSummonCount`, `TestEvocationSpawnsScaledSummons`, `TestEvocationSendsAddPartyForSummon`, `TestEvocationDoesNotDuplicateExistingSummon`.
- `TestSummonExpires`, `TestSummonGoneAfterRelogin`, `TestSummonAssistsAgainstMob`.
- Issue #234: `TestSoloSummonDespawnsOnRemoveParty`, `TestEvocationAfterLeavingPartyStaysCapped`,
  `TestSummonDespawnsWhenOwnerLeavesParty`, `TestSummonDespawnsWhenKicked`,
  `TestSummonDespawnsOnPartyDisband`, `TestSummonPartySlotClearedOnDespawn`.
- `TestApplyTransformScore`, `TestTransformCastBroadcastsMesh`, `TestTransformExpiryRevertsMesh`.
