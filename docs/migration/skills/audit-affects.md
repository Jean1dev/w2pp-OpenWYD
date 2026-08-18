# Auditoria do pipeline de affects (ago/2026)

Levantada junto com a issue #267 (Samaritano), quando a pergunta virou *"por que uma skill marcada
`IMPLEMENTED` não fazia o que se espera dela em jogo?"*. A resposta da #267 está em
`../ingame-bugs.md` B16. Este arquivo guarda o **resto** do que a varredura achou — coisas reais, com
`file:line`, que **não** viraram código naquela PR. Nada aqui é bug conhecido do jogo: é dívida
mapeada, para a próxima frente não precisar redescobrir.

Escopo varrido: `tmserver/internal/handler/{affect_score,affect_tick,combat,item,summon,transform}.go`,
`tmserver/internal/world/affect.go`, `Release/Common/SkillData.csv`, `Source/Code/Basedef.cpp`,
`Source/Code/TMSrv/{Server,_MSG_Attack}.cpp` e `Source/Buff Loop.txt`.

## 1. `Con` é um stat de saída morta

`AffCon` é **escrito** pelos affects 14 e 24 (`affect_score.go`, `applyConHpBuff`) e 29
(`applySoulScore`), e **lido em um único lugar**: `computeScore` (`item.go:2055`,
`Con: e.Con + e.AffCon`) — ou seja, só o display do cliente.

- Não existe `effectiveCon()`, ao contrário de `effectiveStr/Int/Dex` (`affect_score.go:309-313`).
- `MaxHP` não deriva de `Con`: `refreshScore` faz `e.MaxHP = e.BaseMaxHP + b.maxHP`
  (`item.go:1958`). O único elo Con→HP é em tempo de **alocação** de ponto
  (`misc.go:58-61`, `BaseMaxHP += 2*points` junto de `BaseCon += points`).
- O gate de requisito de item lê `e.Con` cru (`item.go:1546`), então buff de CON não ajuda a equipar.

Isso não é divergência: o legado também deriva o HP a partir do Con **antes** do Buff Loop
(`Basedef.cpp:3152-3163`), então lá um CON de buff também não realimenta o MaxHp. É por isso que os
affects 14/24 somam `AffMaxHP` explicitamente — quem escrever um buff de CON novo tem de fazer o
mesmo, ou o efeito sai cosmético.

## 2. Affect 14 (Possuído) sem o `×3` de ClassMaster

`Basedef.cpp:4053-4064`:

```cpp
else if (Type == 14)// Possuído
{
    int value = Level * 3 / 4 + Value;
    if (extra->ClassMaster != MORTAL && extra->ClassMaster != ARCH)
        value *= 3;
    MOB.CurrentScore.MaxHp += value * 22;          // (106k)
    int tv = MOB.CurrentScore.Con + value;
    MOB.CurrentScore.Con += tv * 125 / 100;        // (5004)
}
```

O Go faz `MaxHP += 2v` / `Con += v` e **não** aplica o `×3`. Registrado sem corrigir de propósito: os
multiplicadores `*22` e `*125/100` desse trecho não existem no `Source/Buff Loop.txt:127-139`, que traz
a mesma skill com `MaxHp += value*2` e `Con += value`, e os próprios comentários do `Basedef.cpp`
(`// (106k)`, `// (5004)`) são anotação de tuning de servidor privado. O port seguiu o Buff Loop, e os
prints da #267 confirmam a fórmula do Go em jogo (`Special 200 → CON +160`). Mexer nisso é decisão de
balanceamento, não de paridade — e afeta só personagens fora de `MORTAL`/`ARCH`, o que depende do
sistema de tiers celestiais (`celestial-system-plan.md`).

## 3. Tick types 3, 12 e 46 do CSV não são tratados

`processAffect` (`affect_tick.go:66-89`) cobre 17/20/22/23. A coluna 9 (`TickType`) do
`Release/Common/SkillData.csv` também traz:

| Skill | Nome | TickType |
|------:|------|---------:|
| 226 | `Resi_Decrease` | 3 |
| 202 | `Blaze_Luncher` | 12 |
| 225 | `Chama_Resistente` | 46 |

São no-ops silenciosos: o affect é instalado por `SetTick`, ocupa slot, mostra ícone, conta o tempo e
não faz nada.

## 4. `DoRemoveHide` não foi portado

`_MSG_Attack.cpp:271-272` faz `if (Class == 3) DoRemoveHide(conn)` — a Huntress sai da invisibilidade
ao atacar. `RsvHide` é ligado pelo affect 28 (`affect_score.go`) e nunca é removido no Go. O
`DoRemoveSamaritano`, que fica na linha seguinte do mesmo arquivo, foi portado pela #267; este não.

## 5. `SetAffect`/`SetTick` não invalidam o score

`world/affect.go` instala/remove affect e **nunca** dispara recomputo — quem mexe em `e.Affect` tem de
chamar `refreshScore` na mão, senão os caches `Aff*` ficam velhos. Dos 29 call sites de `refreshScore`,
só cinco vêm do caminho de affect: `combat.go` (cast, proc de on-hit, Cancelamento, o novo
`removeSamaritano`), `affect_tick.go:119` (expiração) e `mobai.go:461` (Divina). É o seam mais fácil de
furar em código novo.

## 6. Affects sem case de score, por desenho

Não são lacunas — anotados para não serem "consertados" por engano:

- **18** (Controle de Mana, skill 46): consumido no combate via `HasAffect(18)` (`combat.go`).
- **32** (Cancelamento): é uma ação que limpa slots, não um modificador (`combat.go`).
- **34/35** (Divina/Vigor): read-time em `affectMul` (`item.go:2004-2012`).
- **40-48**: icon-only, travado por `TestAffect40PlusAreIconOnly` (`affect_score_test.go:19`).

## 7. Confiabilidade das tabelas por classe

O `README.md` desta pasta declara `151 IMPLEMENTED / 0 PARTIAL / 0 MISSING / 0 UNVERIFIED`. A matriz
sobre-declara: os itens 3 e 6 acima são efeito de conteúdo sem tratamento no Go.

A leitura prática da coluna **Evidência**: `SCORE` diz que existe um case no `affect_score.go`, não que
alguém já conferiu o número. As linhas com `SCORE` e **sem** `TEST` são as que não se sustentam sem ler
o código — no TransKnight eram 3, 5, 13 e 16. A #267 fechou a 3 e a 13 com teste; **5** (`Aura_da_Vida`,
tick 17) e **16** (`Perseguição`, affect 3) continuam descobertas.

Regra que a #267 deixa: buff novo ou alterado entra com teste do efeito de score. Foi exatamente a
ausência dele que deixou a #267 nascer.
