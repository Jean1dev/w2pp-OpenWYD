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

`g_pDropRate[]`/`g_pDropBonus[]` são arrays globais **por slot** (64 posições do `Carry`, não por
item) — **origem confirmada:** valores estáticos em `Basedef.cpp:222-238`, ajustáveis em runtime via
`gameconfig.txt` (carregado em `Server.cpp:1302-1342`) e por comando GM (`imple.cpp:1095-1109`).

Valores-base reais (`Basedef.cpp`):
```text
g_pDropBonus[64] = todos 100               # 100 = sem bônus (neutro)
g_pDropRate[64]  = {                        # quanto MAIOR, mais raro (é divisor/odds)
  slots  0-7  : 900            (equip comum)
  slots  8-11 : 4              (muito comum — provável gold/poção)
  slots 12-15 : 900
  slots 16-23 : 20000          (raríssimo)
  slots 24-47 : 2000
  slots 48-55 : 3000
  slot  56    : 1              (sempre dropa)
  slots 57-63 : 35,500,2500,5000,5000,10000,20000
}
```
> O slot do `Carry` do mob determina a raridade — não o item em si. `ItemDropList.txt` no `Release/`
> (formato `Item: N: Mobs:<count>`) é **gerado** pelo `DropTool.exe` (relatório inverso), **não**
> consumido em runtime. Na migração, modelar drop como `(mob_slot → item, rate)` e tornar
> `g_pDropRate`/`Bonus` configuráveis.

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

## 4. Combate (dano) — **fórmulas verificadas**

> Correção vs. versão anterior: as funções `BASE_*` **têm fonte** em `Basedef.cpp` (não são lib
> opaca). As fórmulas abaixo são o código real. RNG via `rand()` (validar por distribuição, Fase 8).

### 4.1. Fórmula de mitigação de dano — `BASE_GetDamage(dam, ac, combat)` (`Basedef.cpp:1265`)

```text
tdam  = dam - ac/2                       # AC mitiga metade do seu valor
combat = min(combat/2, 7)                # "combat" = nível de maestria da arma, teto 7
delta = 12 - combat
rnd   = rand() % delta + combat + 99     # fator % ∈ [combat+99 , 110]  (variância da arma)
tdam  = rnd * tdam / 100                 # aplica o fator percentual

# "piso" não-linear quando o dano fica baixo/negativo:
if tdam < -50:           tdam = 0
elif -50 <= tdam < 0:    tdam = (tdam+50)/7
elif 0 <= tdam <= 50:    tdam = 5*tdam/4 + 7
if tdam <= 0:            tdam = 1         # dano mínimo sempre 1
```

> Reproduzir **exatamente** o `rnd` (faixa depende de `combat`) e a escada de pisos. Quanto maior a
> maestria (`combat`), menor a `delta` → menos variância e piso de dano mais alto.

### 4.2. Dano de skill — `BASE_GetSkillDamage(dam, ac, combat)` (`Basedef.cpp:1486`)

```text
tdam  = dam - ac/2
combat = min(combat, 15)                 # teto 15 (skills usam mais maestria que melee)
delta = 21 - combat
rnd   = rand() % delta + combat + 90      # fator % ∈ [combat+90 , 110]
tdam  = tdam * rnd / 100
if tdam < -50:          tdam = 0
elif -50 < tdam < 0:    tdam = (tdam+50)/10
elif 0 <= tdam <= 45:   tdam = 5*tdam/4 + 5
if tdam <= 0:           tdam = 1
```

### 4.3. Pipeline de um golpe — `_MSG_Attack.cpp` (por alvo em `Dam[i]`)

```text
dam = atacante.CurrentScore.Damage                              # :442

# Crítico duplo (BASE_GetDoubleCritical decide bits em DoubleCritical):       :440
if DoubleCritical & 2:   # "critical parcial"
    dam = ((rand()%2 + 13) * dam)/10   se alvo é player (×1.3–1.4)            # :447
    dam = ((rand()%2 + 15) * dam)/10   se alvo é mob    (×1.5–1.6)            # :449

Ac = alvo.CurrentScore.Ac
if alvo é player:  Ac *= 3              # players têm AC 3× mais eficaz em PvP  :454-455
dam = BASE_GetDamage(dam, Ac, master)  # master = maestria da arma (§4.1)      :457

# (FM/Class 3 com skill 0x200000: 25% de chance de golpe em área extra)       :459-478
if DoubleCritical & 1:   dam *= 2       # crítico total dobra                  :480

# ... resolução de acerto/parry (§4.4), reflect (§4.5), clamps ...
if alvo é mob e dam>=1:  dam += atacante.ForceMobDamage                        :1470
if dam >= MAX_DAMAGE:    dam = MAX_DAMAGE                                      :1473
```

### 4.4. Acerto / esquiva / parry — `_MSG_Attack.cpp:1415-1440`

```text
attackdex = ... (+500 se Rsv & 0x40)
parryretn = GetParryRate(alvo.MOB, alvo.Parry, attackdex, atacante.Rsv)
if skill ∈ {79,22}:  parryretn = 30*parryretn/100      # certas skills reduzem parry
rd = rand() % 1000 + 1
if rd < parryretn:                # ESQUIVA/PARRY
    dam = -3                      # código de "miss/block"
    if (alvo.Rsv & 0x200) and rd < 100:  dam = -4
```

> Esquiva é um roll em **mil** (`rand()%1000+1`) contra `parryretn` (de `GetParryRate`). `dam<0` é o
> sinal de "errou/bloqueou" propagado ao cliente.

### 4.5. Reflect / absorção (PvP) — `_MSG_Attack.cpp:1496-1508`

```text
if alvo é player e dam>0:
    dam -= alvo.ReflectDamage                 # reflect plano; min 1
    dam -= dam/100 * alvo.ReflectPvP          # reflect percentual; min 1
```
Efeitos de status no golpe: `RSV_FROST` (50% → affect 36) e `RSV_DRAIN` (50% → affect 40) com
`Special[1]` (`:1511+`).

### 4.6. Conversão atributo→dano (no `BASE_GetCurrentScore`, `Basedef.cpp:3014+`)

`CurrentScore.Damage` parte do equip e recebe `EF_DAMAGE` (`:3028`); o **balanceamento por classe**
soma `Dex*kd + Str*ks` conforme a arma. Ex.: TK (`Class==0`) com skill "Confiança" (`:3259+`):

| Arma (`nUnique`) | Fórmula de bônus de dano |
|------------------|--------------------------|
| 43 Garra | `Dex*0.38 + Str*0.42` |
| 42 Arco | `Dex*0.55 + Str*0.60` |
| 46 Hermai | `Dex*0.36 + Str*0.40` |
| 48 Espada 2 mãos | `Dex*0.56 + Str*0.60` |

`Critical = BASE_GetMobAbility(EF_CRITICAL)/4` (`:3209`). Cada classe tem seu bloco de coeficientes
(continuar lendo `Basedef.cpp:3245+` por classe) — **tabelar todos na implementação**.

> **Ainda UNVERIFIED (menor):** os coeficientes Dex/Str de **todas** as classes/armas (só TK
> exemplificado aqui) e a árvore completa de `BASE_GetDoubleCritical`/`GetParryRate`. O esqueleto e as
> fórmulas-núcleo (§4.1–4.5) estão fechados; complementar com golden cases (Fase 8) para validar.

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
| Dano: mitigação AC | `dam - ac/2` | `Basedef.cpp:1267` | melee/skill |
| Dano: fator melee | `rand()%(12-min(combat/2,7)) + min(combat/2,7)+99` | `Basedef.cpp:1273` | variância por maestria |
| Dano: fator skill | `rand()%(21-min(combat,15)) + min(combat,15)+90` | `Basedef.cpp:1493` | variância skill |
| AC de player em PvP | `Ac *= 3` | `_MSG_Attack.cpp:455` | players resistem 3× |
| Crítico parcial | `×1.3–1.4` (player) / `×1.5–1.6` (mob) | `_MSG_Attack.cpp:447-449` | `DoubleCritical&2` |
| Crítico total | `dam *= 2` | `_MSG_Attack.cpp:480` | `DoubleCritical&1` |
| Esquiva/parry | `rand()%1000+1 < parryretn` → `dam=-3` | `_MSG_Attack.cpp:1423-1428` | miss/block |
| Dano mínimo | `1` | `Basedef.cpp:1294/1515` | piso |

> **Status da Fase 4: COMPLETO (núcleo).** EXP/party, drop (gold/comum/evento, com valores reais de
> `g_pDropRate`), refino/combine (rolls + tabelas), e **combate** (fórmulas reais de
> `BASE_GetDamage`/`BASE_GetSkillDamage` + pipeline do `_MSG_Attack`, acerto/parry, reflect) estão
> documentados com fórmula + file:line. UNVERIFIED **menor** (a complementar por golden cases, Fase 8):
> os coeficientes Dex/Str por **classe×arma** (só TK exemplificado), a árvore de
> `BASE_GetDoubleCritical`/`GetParryRate`, e o parsing fino de `NPCGener`/AI de mob.
