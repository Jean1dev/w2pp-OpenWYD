# Mapa de NPCs — tipos, gatilho e status de implementação

> Levantamento de **todos** os tipos de NPC presentes no conteúdo
> (`Release/TMsrv/run/npc/`, 598 templates) por `MOB.Merchant` (byte @ `CurrentScore+12`),
> cruzado com [_MSG_Quest-npcs.md](./_MSG_Quest-npcs.md) e os handlers já implementados.
> Objetivo: gap-analysis para garantir cobertura. Contagem = nº de templates com aquele Merchant.

## Como o cliente dispara a interação
O **`Merchant`** do NPC decide o que o clique manda:
- **Loja** (`Merchant==1`, `19` especial) → `_MSG_REQShopList` (0x027B) → `reqShopList/buy/sell`.
- **Cargo/banco** (`Merchant==2`) → `_MSG_REQShopList` → abre o cofre.
- **Combine/refino** (Odin/Lindy/Shany/…) → família `_MSG_CombineItem*`.
- **Quest/serviço/troca** → `_MSG_Quest` (0x028B), `Parm1=npcIndex`, `Parm2=confirm`.
  O sub-tipo vem de `Merchant` e, p/ `Merchant==100`, do `EF_GRADE0` (efeito 100) do `Equip[0]`.

## Tabela de Merchant → propósito → status

| Merchant | Nº | Exemplos | Gatilho | Status |
|---------:|---:|----------|---------|--------|
| 0 | 1231 | Thor_, Orc_Penado | — (monstro) | N/A (sem interação) |
| 1 | 160 | Acessorios, CustomShop, MountCaptor | REQShopList | ✅ loja |
| 2 | 9 | Guarda_Carga | REQShopList | ✅ cargo |
| 3 | 3 | Knight_Master, Mestre_Ancian | _MSG_Quest | ❌ mestre de classe |
| 4 | 24 | Torre_II, Fenix, Kei | _MSG_Quest (GOLD_DRAGON?) | ❌ |
| 5 | 1 | QUEST_Teste | _MSG_Quest | ❌ teste |
| 6 | 6 | Empis, Balmus, Judith | _MSG_Quest | ❌ |
| 8 | 1 | Chefe_Treina. | _MSG_Quest (CAPAVERDE_TRADE) | ❌ |
| 9 | 2 | Freyja | _MSG_Quest | ❌ |
| 10 | 2 | Cap.Mercenario | _MSG_Quest (AMU_MISTICO) | ❌ |
| 11 | 2 | ExploitLeader | _MSG_Quest (EXPLOIT_LEADER) | ❌ |
| 12 | 3 | Jeffi, Alchemy_Jeffi | _MSG_Quest (JEFFI) | ❌ |
| 13 | 2 | Curandeiro, Shaman | _MSG_Quest (SHAMA) | ❌ |
| 16 | 81 | BarebackHorse, WildBoar, Sem_Sela | REQShopList? | ❌ **montarias** (captura/loja de mount) |
| 19 | 5 | Foema_Ancian, Mestre_Archi | REQShopList | ✅ loja especial (shopType 3) |
| 20 | 1 | BlackOracle | _MSG_Quest | ❌ |
| 23 | 2 | Griphon_Master, Mestre_Grifo | _MSG_Quest | ❌ montaria grifo |
| 24 | 1 | Alquimista_Odin | CombineItem (Odin) | ✅ combine |
| 26 | 2 | (Kingdom) | _MSG_Quest (KINGDOM) | ❌ |
| 30 | 2 | Leaky_Zakum | _MSG_Quest (ZAKUM info) | ❌ |
| 31 | 2 | Mestre_Haby | _MSG_Quest (MESTREHAB) | ❌ mestre de skill |
| 32 | 36 | Feiticeira, Wizard, Talos_Statue | _MSG_Quest | ❌ |
| 36 | 5 | GoldDragon, Teleport_Kefra | _MSG_Quest (TREINADORNEWBIE1/teleporte) | ❌ |
| 48 | 1 | Dragao_teste | _MSG_Quest | ❌ teste |
| 58 | 5 | MountMaster, M._de_Montaria | _MSG_Quest (MOUNT_MASTER) | ❌ **cura/ressuscita montaria** |
| 62 | 3 | SilverDragon, Dragao_de_Azran | _MSG_Quest (ARZAN_DRAGON) | ❌ |
| 64 | 65 | Magician, Imp, Wizard | _MSG_Quest | ❌ |
| 68 | 17 | Kemi, Aylin, Jasmine | _MSG_Quest (GODGOVERNMENT) | ❌ |
| 72 | — | Uxmal (ver merchant 104) | _MSG_Quest (UXMAL) | ❌ |
| 74 | 3 | Kibita, Lindy | CombineItem (Lindy) / _MSG_Quest | ⚠️ parcial (Lindy combine ✅) |
| 76 | 3 | Urnammu | _MSG_Quest (URNAMMU) | ❌ |
| 78 | 3 | QuestOffice, Blue/RedOracle | _MSG_Quest (BLACKORACLE) | ❌ |
| 80/84 | 1/1 | Aaryan, Oraculo_Negro | _MSG_Quest | ❌ |
| 96 | 23 | King_Harabard, Lanceiro | _MSG_Quest | ❌ |
| 97 | 2 | Armeiro_Odalac | _MSG_Quest | ❌ |
| 99/116 | 2/2 | God_Government | _MSG_Quest (GODGOVERNMENT) | ❌ |
| 100 | 34 | **Perzen**, Treinador1, Guarda_da_Sorte | _MSG_Quest (por grade) | ⚠️ **Perzen (grade 7/8/9) ✅**; resto ❌ |
| 101 | 8 | Cecilia, U_Ni_Corn | _MSG_Quest | ❌ montaria unicórnio? |
| 104/105/107 | 4/2/1 | Uxmal, Treinador2/3, TrainerChief | _MSG_Quest (tutoriais) | ❌ |
| 110 | 1 | Unicornio_Puro | _MSG_Quest | ❌ |
| 111 | 4 | Rei_Glantuar, Rei_Harabard | _MSG_Quest (KING) | ❌ |
| 113 | 5 | Mercador_Noel, Arqueologo | _MSG_Quest | ❌ |
| 120 | 4 | Carbuncle_Wind, Ajudante | _MSG_Quest (CARBUNCLE_WIND) | ❌ |
| 128 | 5 | Gate_Keeper, Town_Watcher | _MSG_Quest | ❌ guardas/teleporte |
| 224/229 | 1/1 | Compositor, Zangets | _MSG_Quest | ❌ |

## Merchant==100 — sub-tipos por `EF_GRADE0`
Ver a tabela completa em [_MSG_Quest-npcs.md](./_MSG_Quest-npcs.md). Resumo dos grades presentes
no conteúdo: 0–16, 24, 26–30. **Implementado:** grade **7/8/9 = PERZEN** (troca esfera→montaria,
data-driven por `npc.Carry[0]→[1]`). Os demais (cadeia Quest 256 grades 0–4, líder grade 5,
quests grade 11–16/24–30) **não**.

### Perzen — pares item→recompensa (data-driven, do template do NPC)
| NPC | grade | input (Carry[0]) | recompensa (Carry[1]) |
|-----|------:|------------------|-----------------------|
| Perzen_Normal | 7 | 4128 Esfera da Sorte | 3987 Thoroughbred (30d) |
| Perzen_Mistico | 8 | 4129 Esfera da Sorte (M) | 3988 Klazedale (30d) |
| Perzen_Arcano | 9 | 4130 Esfera da Sorte (A) | 3987 Thoroughbred (30d) |
| Perzen | 10 | — | — (não é troca; grade fora de 7-9) |

> **Atenção (não é bug):** cada Perzen exige a esfera correspondente. Esfera **(A)** → Arcano,
> **(M)** → Místico. Clicar no Místico com a Esfera (A) é no-op ("lacks input item want=4129").

## Já implementado (fora do _MSG_Quest)
- **Loja NPC** (Merchant 1/19): `reqShopList`, `buy`, `sell`.
- **Cargo/banco** (Merchant 2): `reqShopList` → `openCargo`, deposit/withdraw.
- **Combine/refino**: família `combineItem` (Odin/Lindy/Shany/… via opcodes dedicados).
- **Perzen** (Merchant 100 grade 7/8/9): troca esfera→montaria.

## Gaps prioritários (sugestão)
1. **Montarias** (Merchant 16 captura, 58 Mount Master cura/ressuscita, 23 grifo, 101/110 unicórnio)
   — alinha com o sistema de montaria (Shire/esfera) que estamos implementando.
2. **Mestres de skill/classe** (Merchant 3, 31) — progressão.
3. **Teleportes/guardas** (Merchant 36, 128) — locomoção.
4. **Cadeia Quest 256** (Merchant 100 grades 0–4) — progressão de level (hardcoded; candidato a
   tabela data-driven `quest`/`quest_step`).
5. Restante (reis, oráculos, governo, etc.) — quests pontuais.

> **Nota:** o engine de quest completo é hardcoded (level, tickets, coords) — forte candidato a
> **conteúdo data-driven** (ver game-rules.md Fase 4). A troca Perzen já é data-driven (pelo Carry
> do NPC) e é o modelo a seguir para os demais NPCs de troca.
