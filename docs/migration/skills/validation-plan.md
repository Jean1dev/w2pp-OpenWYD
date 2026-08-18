# Plano de validacao de skills

## Validacao automatizada

Unit tests em `tmserver/internal/combat`:
- `ManaSpent`: SaveMana, Special impar, custo zero e cap por integer order.
- `SkillBaseDamage`: uma skill elemental por classe, skill 79, skill 97, InstanceType 6 e InstanceType 11.
- `SkillDamage`/`Damage`: golden cases com RNG MSVC e ordem de chamadas preservada.
- `SkillResistScale`: player vs mob, types 1..5 e resist com affects.
- `GetParryRate` e `BASE_GetDoubleCritical` apos port, incluindo consumo de RNG.

Handler tests em `tmserver/internal/handler`:
- Cast aprendido com mana suficiente desconta `CurrentScore.Mp` e `ReqMp`, clampa `ReqMp>=Mp`, e ecoa `CurrentMp`/`ReqMp` no `_MSG_Attack`.
- Mana insuficiente cancela cast, nao desconta MP, envia `MSG_SetHpMp` (`0x0181`, 28 B) e nao executa a skill.
- Skill nao aprendida, classe errada e passiva rejeitam sem broadcast.
- Skill de outra classe nao deve ser aceita, exceto `SkillIndex >= 96` com bit Sephira correto.
- Skill 85 deve cobrar gold `100*Level` antes do gasto de MP; skill 86 nao deve cobrar gold especial.
- Alvo morto so aceita resurrection; merchant NPC nao toma dano; alvo inexistente zera dano.
- Agressive affect pula aliado/guild/leader e respeita `RsvBlock`.
- Multialvo respeita `MaxTarget` depois que essa validacao for implementada.

Affect tests:
- `SetAffect`: player-only, reuso de slot, `AffectTime` ja dividido por 4 no loader, timer `(AffectTime+1)*delay/100`, tick de 8 s e clamp de types 1/3/10.
- `SetTick`: mob permitido, merchant bloqueado, tick 17 HoT, tick 20 DoT, tick 22 quando implementado.
- `sweepAffects`: expiracao zera slot, refresh `MSG_UpdateScore` em grid, `MSG_SendAffect` self, `MSG_SetHpDam` quando aplicavel, transform expiry revertendo mesh.
- Samaritano/type 24: player recebe CON/MaxHP sem AC, mob trata o type como vida de summon e o
  primeiro ataque remove o buff sem cancelar o proprio cast.
- Persistencia: learned skill, special, skillbar, short skill, affects, Divine deadline e score sem double-count.

Tests por classe:
- TK: uma skill de dano por arvore, Furia Divina, Exterminar, Samaritano e passivas de weapon/AC.
- FM: Cura/Recuperar, Flash, Renascimento, Cancelamento, buffs 41/43/44/45.
- BM: Chamas Etereas, summon 1 e 8, transform 64/66/68/70/71, passivas 65/67/69.
- HT: Ilusao, Tempestade 79, Imunidade/Evasao/Invisibilidade, skill 85/86.
- Sephira/shared: livro Sephira, 96, 97, 98, 99, 216, 226 e um caso 200+ ausente.

Regressoes obrigatorias:
- Melee sempre causa dano contra mob valido com catalogo `Spells` ligado quando `Dam=-2`; skill so entra no caminho de skill com `Dam=-1` e `SkillIndex` valido; `Dam=0` e slot vazio.
- `_MSG_Attack` server-authoritative usa `HEADER.ID = ESCENE_FIELD` para HP/MP/EXP do atacante.
- Buffs nao vazam entre personagens na mesma conexao.
- Score persistido nao fica contaminado por buff derivado.
- Skill cast nunca confia em dano, HP, MP, learned mask ou alvo vindo do cliente.

## Validacao manual com cliente real

### Aprender skill no mestre

- Classe/nivel/skill: TK level suficiente, skill 0 e depois skill 7.
- Preparacao: personagem com `SkillBonus` suficiente e 50M gold para a 8a skill.
- Acao: clicar no NPC mestre e aprender.
- Pacote/log esperado: `MSG_ApplyBonus{BonusType:2, Detail:5000+idx, TargetID:npc}`; depois `MSG_UpdateScore` e `MSG_UpdateEtc`.
- Visual esperado: skill aparece aprendida e pontos/gold atualizam.
- PASS/FAIL: PASS se learned mask e hotbar sobrevivem logout/login.

### Cast de buff curto

- Classe/nivel/skill: FM com skill 41, 43 ou 44 aprendida.
- Preparacao: MP suficiente, outro player em visao se possivel.
- Acao: castar em si e em outro alvo valido.
- Pacote/log esperado: `_MSG_Attack`, `MSG_UpdateScore` com affect icon para a grid, `MSG_SendAffect` completo so para o alvo.
- Visual esperado: icone surge, timer decai e some no tempo esperado.
- PASS/FAIL: PASS se expiracao remove efeito e outro personagem nao herda buff.

### Skill de dano em mob e player

- Classe/nivel/skill: uma skill de dano por arvore de cada classe.
- Preparacao: mob vivo, player alvo em area segura controlada, MP suficiente.
- Acao: castar e observar HP/MP/EXP.
- Pacote/log esperado: `_MSG_Attack` com dano sobrescrito pelo servidor, `CurrentMp` reduzido e `HEADER.ID=ESCENE_FIELD`.
- Visual esperado: dano flutua, HP alvo muda, EXP anda quando mob morre.
- PASS/FAIL: PASS se melee ainda funciona depois de casts invalidos.

### Transformacao BM

- Classe/nivel/skill: BM com skills 64/66/68/70/71.
- Preparacao: BM e outro player em visao.
- Acao: castar transformacao, esperar expirar.
- Pacote/log esperado: `SetAffect` type 16, `MSG_UpdateScore`, `MSG_UpdateEquip`, `MSG_SendAffect`.
- Visual esperado: mesh muda para a fera correta para self e outro player; volta ao normal na expiracao.
- PASS/FAIL: PASS se score volta sem persistir bonus derivado.

### Summon BM

- Classe/nivel/skill: BM com Evocar Condor e Evocar Succubus.
- Preparacao: Special[2] variando abaixo/acima dos thresholds.
- Acao: castar summon, atacar mob, logout/morte/espera de expiracao.
- Pacote/log esperado: `MSG_CreateMob` com create type de summon, pet com `Summoner`, life affect type 24.
- Visual esperado: pet aparece perto, segue, ataca alvo do dono e desaparece.
- PASS/FAIL: PASS se limite/cleanup nao deixa mob preso no mundo.

### Resurrection/heal

- Classe/nivel/skill: FM Cura/Recuperar/Renascimento e Sephira 99 quando implementado.
- Preparacao: alvo ferido e alvo morto controlado.
- Acao: castar heal e resurrection.
- Pacote/log esperado: `_MSG_Attack` com dano negativo para heal, `MSG_SetHpDam` quando aplicavel, score/HP sync.
- Visual esperado: barra de HP sobe; morto revive com CreateMob/score conforme legado.
- PASS/FAIL: PASS se clan 4/merchant/dead-target rules batem com fonte/captura.

### Skill shared 97/98

- Classe/nivel/skill: personagem com bit Sephira aprendido.
- Preparacao: para 97, colocar item 746 na celula exigida; para 98, area com celula valida.
- Acao: castar com e sem pre-condicao.
- Pacote/log esperado: falha sem item/area valida; sucesso com dano/criacao esperada.
- Visual esperado: Canhao/Muro aparece conforme cliente original.
- PASS/FAIL: PASS se pre-condicao bloqueia sem desync e sucesso aparece para players em visao.
