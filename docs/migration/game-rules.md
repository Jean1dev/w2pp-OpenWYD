# Fase 4 — Regras de Jogo e Fórmulas (w2pp-OpenWYD)

> **Objetivo:** extrair a lógica de negócio hardcoded para pseudocódigo/tabelas determinísticas,
> para reimplementar com **paridade**. Fonte: `MobKilled.cpp`, `_MSG_UseItem.cpp`,
> `_MSG_CombineItem*.cpp`, `CMob.cpp`, `CItem.cpp`, configs em `Release/.../Settings/` e `Rates.txt`.
>
> **Aviso de paridade:** muitas fórmulas usam `rand()` (libc do MSVC). Para reproduzir *exatamente*
> os mesmos números seria preciso o mesmo gerador/sequência — inviável entre stacks. A estratégia é
> reproduzir a **distribuição** e validar por amostragem (ver Fase 8 sobre seed/RNG). As constantes
> mágicas abaixo **devem** ser idênticas.

---

## 1. Curva de EXP e distribuição em party (PvE)

Fonte: `TMSrv/MobKilled.cpp:397-590` (`#pragma region PvE`). Disparado quando um mob (`target >=
MAX_USER`) morre por um jogador (`conn < MAX_USER`).

### 1.1. Base

```text
MobExp = GetExpApply(killer.extra, target.MOB.Exp, killer.Level, target.Level)   # :405
UNK_1 = 30                                   # constante base da fórmula  (:409)
UNK_3 = killer.extra.ClassMaster             # tier do personagem (party class)
```

### 1.2. Bônus de party (número de membros)

```text
if 0 < ClassMaster <= MAX_PARTY (12):                 # :419
    NumMob = g_EmptyMob + ClassMaster                 # :421
    if ClassMaster > 1:
        NumMob += PARTYBONUS - 100                     # bônus por party >1  (:424)
    eMob  = MobExp
    isExp = NumMob * MobExp / 100                       # exp escalada pelo tamanho (:427)
```

> `g_EmptyMob` e `PARTYBONUS` são constantes globais (verificar valores em `Server.cpp`). UNK_3 ser
> "ClassMaster" como proxy de tamanho de party é peculiar — **UNVERIFIED** se intencional; preservar.

### 1.3. Loop de distribuição por membro

Para cada membro `party` da `PartyList` do líder (`MAX_PARTY+1` iterações, `:434-441`):

```text
isExp_membro = GetExpApply(party.extra, target.Exp, party.Level, target.Level)
myLevel = party.Level
if ClassMaster not in {MORTAL, ARCH}:  myLevel += MAX_LEVEL + 1      # :452 (tier alto)

exp = (UNK_1 + myLevel) * isExp / (UNK_1 + myLevel)   # = isExp  (:454; simplifica)
clamp: 0 < exp <= 10_000_000
```

### 1.4. Divisores por tier e faixa de nível (tabela determinística)

`MobKilled.cpp:457-527`. Aplicar conforme `ClassMaster`:

**MORTAL** (`:457-479`):
| Nível ≤ | divide exp por |
|--------:|---------------:|
| 200 | 1.00 |
| 300 | 0.84 |
| 356 | 1.05 |
| 370 | 1.63 |
| 380 | 1.95 |
| 390 | 2.55 |
| 399 | 3.70 |

**ARCH** (`:481-506`):
| Nível ≤ | divide por |
|--------:|-----------:|
| 200 | 0.84 |
| 300 | 0.72 |
| 356 | 1.40 |
| 360 | 4.75 |
| 370 | 6.60 |
| 380 | 15 |
| 390 | 21 |
| 400 | 35 |

**Outros tiers (Celestial/SD/SP/DK/CS — não MORTAL nem ARCH)** (`:508-527`):
| Nível < | divide por |
|--------:|-----------:|
| 120 | 10 |
| 150 | 20 |
| 170 | 40 |
| 180 | 80 |
| 190 | 160 |
| ≥190 | 320 |

### 1.5. Ajustes finais e eventos

```text
exp = 6 * exp / 10                                    # corte fixo de 40%  (:529)
if 0 < killer.ExpBonus < 500:  exp += exp * ExpBonus / 100      # bônus de item  (:534)
if NewbieEventServer and party.Level < 100 and tier not Celestial*:
    exp += exp / 4                                    # +25% newbie  (:537)
if DOUBLEMODE:   exp *= 2                              # evento exp dobrada  (:540)
if KefraLive == 0:  exp /= 2                           # boss Kefra vivo penaliza?  (:543)
if NewbieEventServer:  exp += exp*15/100  else  exp -= exp*15/100   # ±15%  (:546-549)

# Log diário de exp (reset por dia)  (:552-556)
# "Hold" de exp (trava de ganho)  (:558-573)
# Clamp ao máximo:
if party.MOB.Exp + exp > g_pNextLevel[MAX_LEVEL+1]:
    party.MOB.Exp = g_pNextLevel[MAX_LEVEL+1]          # :575-577
```

`g_pNextLevel[]` é a tabela de XP por nível (curva de level-up). Flags globais `DOUBLEMODE`,
`NewbieEventServer`, `KefraLive` controlam eventos. A `Rates.txt` (não-código, é descritiva ao
jogador) lista faixas de exp por área — **não** é parseada pelo servidor; serve de referência.

---

## 2. Drop

Fonte: `MobKilled.cpp:2693-2900+`. Ocorre na morte do mob.

### 2.1. Drop de gold (`:2693-2722`) — determinístico exceto `rand()`

```text
UNKGOLD = 18
if target.Level < 10: UNKGOLD = 2
elif target.Level < 20: UNKGOLD = 4
elif target.Level < 30: UNKGOLD = 6
elif target.Level < 50: UNKGOLD = 9
UNKGOLD = rand() % (UNKGOLD + 1)          # chance: ~1/(UNKGOLD+1) de dropar

if MobCoin != 0 and UNKGOLD == 0:         # dropou
    MobCoin = 4 * (rand()%(((MobCoin+1)/4)+1) + (MobCoin+1)/4 + MobCoin)
    if MobCoin > 2000: MobCoin = 2000     # teto por kill
    killer.Coin += MobCoin                # com clamp de 2_000_000_000
```

### 2.2. Drop de item comum (`:2758-2800+`) — tabela = `Carry[]` do mob

A **loot table de cada mob é o próprio inventário** `MOB.Carry[MAX_CARRY=64]`. Para cada slot
ocupado:

```text
for i in [0, MAX_CARRY):
    if target.Carry[i].sIndex == 0: continue
    droprate  = g_pDropRate[i]                         # taxa-base por slot (:2764)
    dropbonus = g_pDropBonus[i] + killer.DropBonus     # bônus (evento + item) (:2765)
    if dropbonus != 100:
        dropbonus = 10000 / (dropbonus + 1)
        droprate  = dropbonus * droprate / 100         # bônus reduz o divisor (:2769-2770)
    pos = i / 8
    if i < 60 and pos in {0,1,2}:                      # ajuste por nível do alvo
        if target.Level < 10: droprate = 4*droprate/100
        elif target.Level < 20: ...                    # (:2779+)
    # roll final compara droprate com rand() (continua após :2800)
```

`g_pDropRate[]`/`g_pDropBonus[]` são arrays globais por slot (indexados por posição no Carry, não
por item) — **UNVERIFIED** a origem exata (provável `gameconfig.txt`/`ItemDropList.txt`).
`ItemDropList.txt` no `Release/` tem formato `Item: N: Mobs que dropam:<count>` (lista inversa
item→mobs), aparentemente **gerada** pelo `DropTool.exe`, não consumida em runtime.

### 2.3. Drop de evento global (`:2725-2755`)

```text
if evOn and evItem and evRate and evCurrentIndex < evEndIndex and rand()%evRate == 0:
    item.sIndex = evItem
    if evIndex:   # item serializado/numerado
        item.eff[0]=62/idx_hi; eff[1]=63/idx_lo; eff[2]=59/rand()
    SetItemBonus(item, target.Level, 0, 0)
    PutItem(conn, item); broadcast notice
    evCurrentIndex++
```

Controlado por variáveis globais de evento (`evOn/evRate/evItem/evStartIndex/evEndIndex`) — viram
config/feature-flags na stack nova.

---

## 3. Refino / Combine (Anct e variantes)

São ~10 handlers `_MSG_CombineItem*` (Fase 1 §3.1). Compartilham o padrão: validar combinação →
rolar sucesso → aplicar/consumir. **Consolidar numa engine de "receitas" parametrizada.**

### 3.1. Engine base — `_MSG_CombineItem.cpp` (Anct)

```text
combine = GetMatchCombine(m->Item)        # taxa da receita casada (0 = inválida)  (:46)
if combine == 0: erro "Wrong_Combination"; return                                  (:48-53)

# consome os itens de entrada
for i in MAX_COMBINE: if Item[i].sIndex: clear Carry[InvenPos[i]]; SendItem(...)    (:55-62)

# ROLL DE SUCESSO (chave para paridade):
_rand = rand() % 115                                                                # :80
if _rand >= 100: _rand -= 15        # "achata" 100..114 -> 85..99                    (:81-82)
success = (_rand <= combine) or LOCALSERVER                                          (:84)

if success:
    itemindex = Item[0].sIndex
    extra = g_pItemList[itemindex].Extra            # item resultante
    joia  = Item[1].sIndex - 2441                    # tipo da joia (0..3)            (:92)
    if 0 <= joia <= 3:
        Carry[ipos] = Item[0]; Carry[ipos].sIndex = joia + extra
        BASE_SetItemSanc(Carry[ipos], 7, 0)          # define refino/sanc = 7         (:100)
        _MSG_CombineComplete parm=1 (sucesso)
else:
    _MSG_CombineComplete parm=2 (falha)                                              (:139)
```

> **Constante de paridade crítica:** o roll é `rand()%115` com o "achatamento" `>=100 ⇒ -15`. Isso
> torna os valores 85..99 **duas vezes mais prováveis**. Reproduzir essa distribuição exatamente,
> não substituir por `rand()%100`. O sucesso é `_rand <= combine`, onde `combine` é a taxa (0..100)
> da receita.

### 3.2. Tabelas de taxa (`Release/Common/Settings/CompRate.txt`)

Carregadas por `CReadFiles::ReadCompRate()` (`CReadFiles.h:32`). Formato `Família Chave Valor`:

| Família | Chave | Taxa |
|---------|-------|-----:|
| Tiny | ChanceBase | 15 |
| Shany | ChanceBase | 30 |
| Ailyn | ChanceBase | 10 |
| Agatha | ChanceBase | 15 |
| Compositor | Item_+7 / +8 / +9 | 2 / 4 / 10 |
| Odin | Item_12_Ref_0..9 | 2,3,4,5,6,7,8,9,12,15 |
| Odin | Item_12_Minus_12..15 | 2,3,4,5 |
| Odin | Item_Celestial / Secreta | 5 / 1 |
| Ehre | Espiritual / Amunra | 40 / 10 |
| Ehre | (demais) | 100 |

(Lista completa no arquivo; cada handler `_MSG_CombineItem<Família>` usa `GetMatchCombine<Família>`.)

### 3.3. Tabela de refino por anvil/sanctificação (`Settings/SancRate.txt`)

Carregada por `ReadSancRate()` (`CReadFiles.h:30`). Taxa de sucesso por **nível de refino** (0..N):

```text
PO (pedra ?)   ref 0..2 = 100%, 3=85, 4=70, 5=40
PL (pedra ?)   ref 0..5 = 100%, 6=80, 7..8=70, 9=10
Âmago          ref 0=100,1=80,2=60,3=40,4=20,5..7=10,8..11=5
```

> Reproduzir as tabelas **exatamente**; são o coração da economia de refino.

### 3.4. Cooldown anti-spam de refino — **DESATIVADO no código**

`_MSG_UseItem.cpp:209-221`: o bloco que impedia refinar mais de 1×/segundo
(`if GetTickCount()-UseItemTime < 1000`) está **comentado**. Ou seja, **não há rate-limit** de
refino hoje.

> **Decisão de migração:** reativar um cooldown server-side (anti-macro) é recomendável, mas muda
> comportamento — registrar como divergência intencional vs. paridade pura (Fase 8/9).

### 3.5. Limites de refino
- Itens "tipo 5" (selados) não refinam além de `sanc >= 9` (`_MSG_UseItem.cpp:227`); outros não
  passam de `sanc >= 6 && Vol == 4` (`:203`). Mensagem `_NN_Cant_Refine_More`.

---

## 4. Combate (dano)

Fonte principal: `TMSrv/CMob.cpp:700-883` (cálculo de score/dano corrente). Resumo do pipeline
(`BASE_GetCurrentScore` em `:709` consolida atributos a partir de equip+affect):

```text
BASE_GetCurrentScore(MOB, Affect, &extra, &ExpBonus, &ForceMobDamage,
                     isMob=(idx>=MAX_USER), &Accuracy, &HpAbs, &ForceDamage)   # :709

# Dano de arma (mão principal/secundária):
WeaponDamage = w1 + fw2   (ou w2 + fw1 conforme arma)        # :787-789
if <condição de arma especial>: WeaponDamage += 40           # :803,817

# Bônus por sanctificação (refino) do equip:
isanc = BASE_GetItemSanc(Equip[6]/[7]/[i])                    # :800,814,825
ForceDamage  += (Grade==6 ? 80 : 40) * isanc                 # :867
ReflectDamage += (Grade==8 ? 80 : 40) * isanc                # :873

# Reflect / absorção:
ReflectDamage += (Special[3]+1)/6                            # :779
AC += (GetItemAbility(Equip[7], EF_AC)+1)/7                  # :783

PvPDamage = AtaquePvP                                         # :883 (dano específico PvP)
```

Acerto/esquiva: `Accuracy`/`HpAbs` saem de `BASE_GetCurrentScore`; rolls com `rand()%100`
aparecem em `CMob.cpp:281` (`if BaseInt < rand()%100`) e `:310` (`Rand = rand()%100`). O cálculo
final de hit/critical/dodge usa essas comparações.

> **UNVERIFIED / a aprofundar:** a fórmula completa de acerto/esquiva/crítico e a soma final de dano
> (ordem exata das parcelas, clamps) está espalhada entre `BASE_*` (lib sem fonte) e
> `CMob.cpp:700-900`. Para paridade de combate, **capturar golden cases** de ataque (Fase 8) é
> mais confiável que reconstruir só pela leitura, dado o uso de funções `BASE_*` sem código.

---

## 5. Skills / efeitos

- Modelo de efeito de item/skill: `ItemEffect.h` (constantes `EF_*`) + `STRUCT_SPELL`
  (`Basedef.h:1110+`, ver Fase 2 §3.2). Cada skill tem `Delay`, `Range`, `ManaSpent`,
  `Affect*`, `Tick*`, `Instance*`.
- **SkillDelay /4 no cliente:** o `ClientPatch` divide o `Delay` tabelado por 4 em 104 entradas
  (`ClientPatch_v7662/Hook.cpp:230-231`). Efeito prático: cooldowns de skill 4× mais rápidos do que
  a `SkillData.csv` indica. O **servidor** valida `LastAttackTick`/anti-flood (`CUser`), então a
  stack nova deve usar o mesmo delay efetivo (Delay/4) para não rejeitar ações legítimas do cliente.
- Efeitos de affect persistidos: `STRUCT_AFFECT[MAX_AFFECT=32]` com `Time` (expiração).

---

## 6. Eventos e timers

Orquestrados por `ProcessSecMinTimer.cpp` (tick de segundo/minuto) e classes dedicadas:

| Evento | Fonte | Estado/constantes |
|--------|-------|-------------------|
| Guild War / Torre | `CWarTower.*` | `GTorreHour`, `TowerCount`, `TowerStage`, `GuildTower` (`Server.h:73-75`) |
| Castle / Zakum | `CCastleZakum.*` | `KeyDrop` (`MobKilled.cpp:2870`), `Settings/CastleQuest.txt` |
| RvR | regiões `Regions.txt` (`RvR`) | `BrState` |
| Quests diárias | `_MSG_Quest`, `QuestDiaria.txt`, `Settings/QuestsRate.txt` | `STRUCT_QUEST` por char |
| Eventos de exp/drop | flags `DOUBLEMODE`, `NewbieEventServer`, `evOn` | globais (viram feature-flags) |

`CastleQuest.txt` e `QuestsRate.txt` (em `Settings/`) parametrizam recompensas/taxas — carregar
como config (Fase 7).

---

## 7. Constantes mágicas a preservar (resumo)

| Constante | Valor | Onde | Papel |
|-----------|------:|------|-------|
| Corte de exp | `6*exp/10` | `MobKilled.cpp:529` | -40% fixo |
| Newbie/evento exp | `±15%`, `+25%` | `:537,546-549` | ajustes de evento |
| Roll de combine | `rand()%115`, `>=100⇒-15` | `_MSG_CombineItem.cpp:80-82` | distribuição de sucesso |
| Joia base | `sIndex - 2441` | `:92` | offset do tipo de joia |
| Sanc pós-combine | `7` | `:100` | refino do item resultante |
| Teto de gold/kill | `2000` | `MobKilled.cpp:2713` | clamp |
| Teto de gold total | `2_000_000_000` | `:2715` | overflow guard |
| Cooldown refino | `1000ms` (DESATIVADO) | `_MSG_UseItem.cpp:214` | anti-spam comentado |
| SkillDelay client | `/4` | `Hook.cpp:230` | cooldown efetivo |
| Bônus de dano por sanc | `(Grade==6?80:40)*isanc` | `CMob.cpp:867` | force damage |

> **Status da Fase 4: PARCIAL.** EXP/party, drop (gold/comum/evento) e refino/combine documentados
> com fórmula + file:line. UNVERIFIED a fechar por captura (Fase 8): fórmula completa de combate
> (funções `BASE_*` sem fonte), origem exata de `g_pDropRate[]`, e parsing fino de `NPCGener`/AI.
