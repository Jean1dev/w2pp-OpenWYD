# Captura WYD: sistema de Affect (buffs) + Poção Divina (item 3381)

> Origem: agente Windows (fonte C++ COMPLETA que compila + dumper `_layout_probe/dump_layout.cpp`,
> MSVC x86, alinhamento natural). Header CPSock = 12B. Verificado pelo compilador.
>
> **EF_VOLATILE:** o *define* `EF_VOLATILE` = **38** (`ItemEffect.h:94`). O **66 é o VALOR** que
> `BASE_GetItemAbility(item, EF_VOLATILE)` retorna p/ a Divina de 30 dias (o `cValue` do efeito no
> item). Vol 64/65/66 = Divina 7/15/30 dias; Vol 58 = Vigor.

## A) STRUCT_AFFECT (8 bytes)
| off | size | tipo | campo |
|---:|---:|---|---|
| 0 | 1 | u8 | `Type` (34=Divina, 35=Vigor) |
| 1 | 1 | u8 | `Value` |
| 2 | 2 | u16 | `Level` |
| 4 | 4 | u32 | `Time` (ticks restantes) |

- `MAX_AFFECT` = 32 (`Basedef.h:182`).
- Vive em `CMob.Affect[MAX_AFFECT]` (`CMob.h:42`) — **NÃO** dentro do `STRUCT_MOB`.
- Persiste no quit (`MSG_SavingQuit.affect[]`, `Basedef.h:1503`). A Divina também persiste via
  `extra.DivineEnd` (`time_t` no `STRUCT_MOBEXTRA`) — **fonte da verdade do prazo**; `Affect.Time`
  é só o ícone.

## B) Consumir no `_MSG_UseItem.cpp`
### MSG_UseItem (36B, Type 0x0373)
SourType(int)@12, SourPos(int)@16, DestType(int)@20, DestPos(int)@24, GridX(u16)@28, GridY(u16)@30, WarpID(u16)@32.

### Dispatch: `Vol = BASE_GetItemAbility(item, EF_VOLATILE)` (`:95`) decide tudo
| Vol | ação |
|---:|---|
| 0 | não-volátil → **equipar** |
| 1 | poção HP/MP |
| 4/5 | PO/PL (refino, usa Dest*) |
| 6/7/8/9 | pílula/poeira/joias/minérios |
| 10/52/53/55/56/57/200/201/202 | pergaminhos → affect Type 4 |
| **58** | **Vigor** (Affect 35) |
| **64/65/66** | **Divina 7/15/30d** (Affect 34) |

### Bloco Divina (`:2128`)
```c
if (Vol >= 64 && Vol <= 66) {
    int sAffect = GetEmptyAffect(conn, 34);
    if (sAffect == -1 || Affect[sAffect].Type == 34) { SendClientMessage(_NN_CantEatMore); SendItem(...); return; }
    time(&extra.DivineEnd);
    extra.DivineEnd += 60*60*24*(Vol==64?8:Vol==65?16:31);   // 8/16/31 dias
    Affect[sAffect] = {Type:34, Level:1, Value:0, Time:2000000000};
    BASE_GetHpMp(&MOB, &extra);   // recalcula MaxHp/MaxMp base
    GetCurrentScore(conn);        // aplica +20% (ver C)
    SendScore(conn);              // MSG_UpdateScore 0x0336
    /* consome 1 do amount, ou zera o item */
    return;
}
```
Vigor (`Vol==58`, item 3313 1h): igual com Type=35, Time=AFFECT_1H.

### GetEmptyAffect(mob,type) (`GetFunc.cpp:734`)
1º slot com `Affect[i].Type==type` (já existe) → senão 1º com `Type==0` (livre) → senão -1.

### BASE_GetItemAbility(item,Type) (`Basedef.cpp:1687`)
Soma `item->stEffect[0..2]` (`cEffect==Type → +=cValue`) + catálogo `g_pItemList[idx].stEffect[0..11]`.
`EF_DAMAGEADD/EF_MAGICADD` só contam se `nUnique∈[41,50]` (joias). Montarias (idx 2330-2390) usam g_pMountBonus.

## C) Bônus real — BASE_GetCurrentScore (`Basedef.cpp:3014`), bloco Affect (`:4551`)
```c
else if (Type == 34) { // Divina → +20% HP/MP/Dano/Magia (cap MAX_*)
    MaxHp  = min(MaxHp  + MaxHp/100*20,  MAX_HP);
    MaxMp  = min(MaxMp  + MaxMp/100*20,  MAX_MP);
    Damage = min(Damage + Damage/100*20, MAX_DAMAGE);
    Magic  = min(Magic  + Magic/100*20,  MAX_DAMAGE_MG);
}
else if (Type == 35) { // Vigor → +10% HP/MP
    MaxHp = min(MaxHp + MaxHp/100*10, MAX_HP);
    MaxMp = min(MaxMp + MaxMp/100*10, MAX_MP);
}
```
**Divina = +20% MaxHp/MaxMp/Damage/Magic. Vigor = +10% MaxHp/MaxMp.** (% sobre o já computado.)

Outros Affect Types consumidos: 8 (buff bitmask em Level), 26 RSV_PARRY, 27 RSV_FROST, 28 RSV_HIDE,
30 ForceMobDamage+=Level, 31 Ac+=Level/2+Value, 36 RSV_DRAIN, 37 ForceDamage+=special2,
38 MaxHp+=MaxMp/2;MaxMp/=2, 39 ExpBonus+=100, 42 MP→HP variável, 29 Alma (Int/Con ×1.4-2.2 por Soul).

### BASE_GetHpMp (`Basedef.cpp:1214`)
`MaxHp = min(BaseSIDCHM[cls][4] + (Con-BaseSIDCHM[cls][3])*2 + Level*g_pIncrementHp[cls], MAX_HP)`;
MP análogo (Int). `BaseSIDCHM[cls]` {Str,Int,Dex,Con,HP,MP}: TK{8,4,7,6,80,45} FM{5,8,5,5,60,65}
BM{6,6,9,5,70,55} HT{8,9,13,6,75,60}.

## D) Pacote do buff: MSG_SendAffect (S→C, 268B, Type 0x03B9)
header(12, ID=conn) + `STRUCT_AFFECT Affect[32]`(256). `SendAffect(conn)` (`SendFunc.cpp:1901`)
copia os 32 slots; Divina (Type 34, Time>=32000000): `Time` exibido = `DivineEnd-now` em dias
(`AFFECT_1D`), ≤1h → Time=450. No fluxo da Divina: manda `SendScore` (recalcula +20%) **e**
`SendAffect` (ícone/tempo). O `SendScore` também leva `Affect[32]` como u16 (só tipos, ícones rápidos).

## E) Refino/joias e EF_*ADD
### BASE_GetItemSanc(item) (`Basedef.cpp:2136`) → 0..15
Lê do `stEffect[]`: `cEffect==EF_SANC` ou range 116..125 → `cValue`. `sanc==9→9`; ranges 230-253 →
REF_10..REF_15; senão `sanc%10`. Montarias → FALSE.
### BASE_GetItemGem(item) (`:2184`)
`sanc<230 → -1` (sem joia); senão `(sanc-230)%4` = índice da joia.
### Escala de dano/AC por refino = **THRESHOLD em sanc>=9**, NÃO multiplicador contínuo
- Arma (`nPos 64||192`): `CMob::GetCurrentScore` `if sanc(Equip[6/7])>=9 → WeaponDamage += 40`.
- Defesa (`Basedef.cpp:4601-4622`): cada `Equip[1..7]` com `sanc>=9`: `nPos 4/8/128 → Ac += 25`; `nPos 16 → Rsv|=RSV_CAST`.
- O dano/AC "cru" do item (que já cresce com o refino) vem dos `stEffect`/catálogo via `BASE_GetItemAbility`.
- `g_pSancRate`/`g_pSuccessRate` = **taxa de SUCESSO do refino** (combine), NÃO escala de status.
### EF_*ADD em GetCurrentScore
| efeito | nº | aplicação | tipo |
|---|---:|---|---|
| EF_ACADD | 53 | `Ac += GetMobAbility(EF_AC)+GetMobAbility(EF_ACADD)` (`:3025`) | **FLAT** |
| EF_HPADD | 45 | `MaxHp = MaxHp*(EF_HPADD+EF_HPADD2+100)/100` (`:3886`) | **% multiplicativo** |
| EF_MPADD | 46 | `MaxMp = MaxMp*(EF_MPADD+EF_MPADD2+100)/100` (`:3897`) | **% multiplicativo** |
| EF_DAMAGEADD | 67 | só `nUnique∈[41,50]` (joias), somado via BASE_GetItemAbility | **FLAT condicional** |

Ordem: Divina (+20%, loop de affect) **ANTES** do `HPADD%` final (`:3886`) → o HPADD% incide sobre o HP já +20%.
