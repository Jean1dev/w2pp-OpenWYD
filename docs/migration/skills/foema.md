# Foema skills 24..47

## Identidade mecanica

Foema concentra magia elemental, cura, buffs de suporte e utilitarios de controle. O dano elemental, cura, ticks, buffs e utilitarios classicos (`Flash`, `Desintoxicar`, `Renascimento`, `Velocidade`, `Controle de Mana`, `Cancelamento`) ja passam pelo fluxo Go com regras especificas da `_MSG_Attack` e do sweep de affects.

Evidencia usa os codigos definidos em `README.md`.

## Catalogo

| Índice | Nome | Árvore | Tipo | Target | Mana | SkillPoint | Instance | Affect/Tick | Fórmula/Efeito | Status | Evidência |
|-------:|------|--------|------|--------|-----:|-----------:|----------|-------------|----------------|--------|-----------|
| 24 | Flecha_Mágica | 1 | dano | 1 | 5 | 12 | 4/15 | - | dano elemental type 4 base 15 | IMPLEMENTED | CSV+CAST+DMG |
| 25 | Desintoxicar | 1 | utilitário | 2 | 12 | 42 | 8/0 | - | limpeza seletiva de debuffs | IMPLEMENTED | CSV+CAST+LEGACY |
| 26 | Flash | 1 | utilitário | 4 | 40 | 66 | 7/8 | - | Flash limpa combate do alvo | IMPLEMENTED | CSV+CAST+LEGACY |
| 27 | Cura | 1 | heal | 2 | 15 | 48 | 6/100 | - | heal especial, cap, fada e EXP | IMPLEMENTED | CSV+CAST+DMG+LEGACY |
| 28 | Choque_Divino | 1 | dano | 1 | 30 | 78 | 4/155 | - | dano elemental type 4 base 155 | IMPLEMENTED | CSV+CAST+DMG |
| 29 | Recuperar | 1 | heal | 0 | 30 | 48 | 6/150 | - | heal especial, cap, fada e EXP | IMPLEMENTED | CSV+CAST+DMG+LEGACY |
| 30 | Julgamento_Divino | 1 | dano | 4 | 150 | 102 | 4/200 | - | dano soma HP e reduz HP do caster | IMPLEMENTED | CSV+CAST+DMG+LEGACY |
| 31 | Renascimento | 1 | utilitário | 1 | 0 | 281 | 8/0 | - | detox + ressurreicao com MP zero | IMPLEMENTED | CSV+CAST+LEGACY |
| 32 | Ataque_de_Fogo | 2 | dano | 1 | 7 | 18 | 2/20 | - | dano elemental type 2 base 20 | IMPLEMENTED | CSV+CAST+DMG |
| 33 | Relâmpago | 2 | dano | 1 | 10 | 51 | 5/65 | - | dano elemental type 5 base 65 | IMPLEMENTED | CSV+CAST+DMG |
| 34 | Lança_de_Gelo | 2 | dano | 1 | 15 | 69 | 3/95 | A1/2/0 | dano elemental type 3; slow/attack-speed | IMPLEMENTED | CSV+CAST+DMG+AFF+SCORE |
| 35 | Tempestade_de_Meteoros | 2 | dano | 4 | 25 | 72 | 2/55 | - | dano elemental type 2 base 55 | IMPLEMENTED | CSV+CAST+DMG |
| 36 | Nevasca | 2 | dano | 1 | 35 | 72 | 3/200 | A1/2/0 | dano elemental type 3; slow/attack-speed | IMPLEMENTED | CSV+CAST+DMG+AFF+SCORE |
| 37 | Trovão | 2 | buff | 0 | 75 | 90 | 0/0 | T22/100 | tick 22 dispara Relâmpago sintético | IMPLEMENTED | CSV+CAST+AFF+LEGACY |
| 38 | Fênix_de_Fogo | 2 | dano | 1 | 85 | 81 | 2/350 | - | dano elemental type 2 base 350 | IMPLEMENTED | CSV+CAST+DMG |
| 39 | Inferno | 2 | dano | 6 | 220 | 224 | 2/340 | - | dano elemental type 2 base 340 | IMPLEMENTED | CSV+CAST+DMG |
| 40 | Névoa_Venenosa | 3 | dano | 1 | 9 | 24 | 4/20 | T20/10 | dano elemental + poison tick 20 | IMPLEMENTED | CSV+CAST+DMG+AFF+SCORE |
| 41 | Teleporte | 3 | buff | 0 | 52 | 39 | 0/0 | A2/1/15 | haste/run-speed com limite multi-alvo | IMPLEMENTED | CSV+AFF+SCORE+LEGACY |
| 42 | Velocidade | 3 | utilitário | 0 | 0 | 54 | 9/0 | - | summon/teleporte de player para coordenada alvo | IMPLEMENTED | CSV+CAST+LEGACY |
| 43 | Escudo_Mágico | 3 | buff | 2 | 52 | 63 | 0/0 | A11/15/150 | buff de AC | IMPLEMENTED | CSV+AFF+SCORE |
| 44 | Arma_Mágica | 3 | buff | 0 | 78 | 75 | 0/0 | A9/90/150 | buff de dano, bit 19 e limite multi-alvo | IMPLEMENTED | CSV+AFF+SCORE+LEGACY |
| 45 | Toque_de_Athena | 3 | buff | 2 | 98 | 72 | 0/0 | A15/7/150 | buff de Special | IMPLEMENTED | CSV+AFF+SCORE |
| 46 | Controle_de_Mana | 3 | buff | 0 | 130 | 81 | 0/0 | A18/0/150 | mana shield reduz dano recebido | IMPLEMENTED | CSV+AFF+LEGACY |
| 47 | Cancelamento | 3 | debuff | 5 | 34 | 269 | 0/0 | A32/0/0 | remove block type 19 antes do generic affect | IMPLEMENTED | CSV+CAST+LEGACY |

## Regras comuns

- Dano FM usa `Int/30 + Int/3 + Level + base + 2*Special`, multiplicador de `Magic`, 5/4 e resist elemental.
- Heal retorna dano negativo no fio Go e aplica formulas especiais, caps 1100/2200, restricao de clan 4, reducao por fada e EXP por cura fora de vila.
- Buffs de suporte passam por `SetAffect`/`SetTick`; os tipos 2, 9, 11 e 15 tem contribuicao de score implementada, e o type 18 atua no caminho de dano recebido.
- Foema com learned bit 19 triplica o bonus de damage buff type 9 no `applyAffectScore`.
- `Trovão`/tick 22 usa um ataque sintético de `Relâmpago` (skill 33) contra mobs proximos; alvos player ficam bloqueados enquanto PK-mode/arena attrs nao existirem no modelo Go.

## Lacunas e riscos

- Captura visual do cliente real ainda e util para confirmar animacoes/efeitos de movimento, mas nao bloqueia a regra server-side.
- `Trovão` nao executa PvP automatico porque o modelo Go ainda nao tem `PKMode`/arena attrs da fonte legada.
- Os nomes de algumas linhas no CSV nao devem ser usados como autoridade quando o legado trata por indice numerico.

## Testes minimos esperados

- `TestFoemaHealFormulaAndCap`, `TestFoemaHealFairyReduction`, `TestFoemaHealExpGain`.
- `TestClearDetoxAffects`, `TestCancelamentoClearsBlockAffect`, `TestManaControlDamage`.
- `TestThunderTargetsSkipProtectedAndDeduplicate`, `TestFoemaMultiBuffTargetCap`.
- Casos existentes de cast/buff cobrem `SetAffect`, score, icone e `MSG_SendAffect`.
