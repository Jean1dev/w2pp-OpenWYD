# Prompt para o agente Windows — Level-up (curva de EXP + ganhos por nível)

> Cole isto na instância do Claude Code na máquina Windows (que tem a **fonte completa**
> que compila + o dumper `_layout_probe`). O objetivo é destravar o **level-up** no servidor
> Go: a experiência já acumula e a barra de exp do cliente já enche (entregue via
> `MSG_Attack.CurrentExp`), mas o servidor ainda **não incrementa o Level** porque depende de
> tabelas/funções que **não estão na nossa cópia parcial** (`Source/Code/`).

## Contexto

Migração do WYD (cliente 12000, header CPSock=12B) de C++ para Go. Já funciona: login,
mundo, lojas, teleporte, persistência (incl. **Exp**), drop de loot, e **ganho de exp ao
matar mob** (solo). Falta o incremento de nível.

A lógica de level-up que JÁ temos visível está em `TMSrv/CMob.cpp:1075-1160` (o trecho que
compara `MOB.Exp` com `g_pNextLevel[cur]`/`[cur+1]`, incrementa `Level`, soma
`g_pIncrementHp/Mp[cls]`, dá `SkillBonus`/`SpecialBonus`, `Ac++`, chama
`BASE_GetBonusScorePoint` e `GetCurrentScore`, e retorna um código 1..4 ligado aos
"segmentos" da barra). Precisamos dos **dados** e das **funções** que esse trecho usa.

## O que preciso (com valores byte-exatos / compiler-verified quando aplicável)

1. **`MAX_LEVEL` e `MAX_CLEVEL`** — os `#define`/const exatos (valores numéricos).

2. **Tabela `g_pNextLevel[]`** (e `g_pNextLevel_2[]`): a curva de XP por nível.
   - Onde ela é **preenchida** (é hardcoded, calculada por fórmula no boot, ou lida de
     arquivo?). Se for fórmula, me dê a fórmula exata; se for tabela, me dê os **valores**
     (pelo menos níveis 1..400 de `g_pNextLevel` e 1..max de `g_pNextLevel_2`).
   - Confirmar o índice base (nível 1 → índice 0 ou 1?) e o valor de
     `g_pNextLevel[MAX_LEVEL+1]` (o teto de exp, usado no clamp em `MobKilled.cpp:575`).

3. **`g_pIncrementHp[]` e `g_pIncrementMp[]`** — ganho de HP/MP por nível, indexado por
   **classe** (`cls` = 0 TK / 1 FM / 2 BM / 3 HT). Valores por classe.

4. **`GetExpApply(STRUCT_MOBEXTRA extra, int exp, int attacker, int target)`** — o corpo
   completo (`GetFunc.cpp`?). É o scaling de exp pela diferença de nível atacante×alvo. Hoje
   no Go pulamos isso (UNVERIFIED) e damos exp cheia → preciso da fórmula real.

5. **`BASE_GetBonusScorePoint(MOB*, extra*)`** — o corpo (como distribui pontos de atributo
   no level-up). E os efeitos de `SkillBonus`/`SpecialBonus` (quantos pontos, onde guardam).

6. **`GetCurrentScore`** no contexto de level-up: o que recalcula (MaxHp/MaxMp/AC a partir de
   atributos + nível?). Já temos um `computeScore` parcial no Go — quero a fórmula real.

7. **Pacotes S→C do level-up:** o que o servidor envia ao cliente quando o nível sobe.
   - Confirmar se é só `SendScore` (`MSG_UpdateScore` 0x0336) ou se há um pacote/efeito
     dedicado de "subiu de nível" (partícula, som). Type (hex), struct e offsets.
   - O código de retorno 1..4 do level-up (segmentos da barra) dispara algum pacote? Qual?

8. **Constantes do exp em party** que ficaram UNVERIFIED no nosso lado (de
   `MobKilled.cpp` §PvE): `g_EmptyMob`, `PARTYBONUS`, `UNK_1` (=30?). Valores reais em
   `Server.cpp`/globais.

## Formato da resposta

- Para cada item: **valor/fórmula exata** + **arquivo:linha** da fonte como evidência.
- Para tabelas grandes (`g_pNextLevel`), pode salvar os valores num bloco de código.
- Use o dumper `_layout_probe` se algum struct (ex.: o pacote de level-up) precisar de
  `sizeof`/`offsetof` verificados pelo compilador.
- Salve tudo em `captura-wyd-levelup.md`.
