# Prompt Mestre — Documentação para Migração (w2pp-OpenWYD)

> **Como usar:** este é um prompt para um agente de documentação (ou um orquestrador que dispara
> subagentes, como o que gerou `docs/agents/`). Copie a seção "PROMPT" abaixo. Ele já vem com as
> decisões de migração embutidas. Rode fase por fase — não tente tudo de uma vez (o código é grande:
> `Server.cpp` ~10.5k linhas, 58 handlers `_MSG_*`).

## Contexto da migração (decisões já tomadas)

- **Stack alvo:** A DEFINIR — a documentação deve incluir uma recomendação fundamentada.
- **Cliente:** MANTIDO (`WYD.exe` 7662 + `ClientPatch_v7662.dll`). ⇒ O servidor novo precisa ser
  **byte-for-byte compatível** no protocolo. A spec de protocolo é o artefato de maior prioridade.
- **Estratégia:** REESCRITA TOTAL (big-bang). ⇒ É obrigatório material de **paridade comportamental**:
  fórmulas extraídas + casos de teste (golden cases) capturáveis do servidor atual.

---

## PROMPT

````
Você é um engenheiro de documentação fazendo a engenharia reversa de um emulador de servidor de
MMORPG (WYD "With Your Destiny") escrito em C++/Win32, para viabilizar uma REESCRITA TOTAL em uma
stack moderna. O cliente do jogo (WYD.exe 7662) será MANTIDO sem alterações, então o servidor novo
DEVE ser 100% compatível no protocolo de fio. A estratégia é big-bang, então preciso de documentação
que permita validar PARIDADE COMPORTAMENTAL com o servidor atual.

REGRAS GERAIS (válidas para todas as fases):
- Baseie TUDO em evidência do código. Cite sempre `caminho/arquivo.cpp:linha`.
- Onde não der para confirmar pela fonte, escreva "UNVERIFIED" e explique o que falta.
- Não invente offsets, tamanhos, fórmulas ou constantes. Se precisar, extraia do código real.
- Escreva pensando em quem NÃO conhece C++/Win32 nem o domínio WYD — defina termos.
- Saída em Markdown, em `docs/migration/`. Pode usar tabelas, ASCII, pseudocódigo.
- Fonte principal: `Source/Code/` (Basedef, CPSock, TMSrv, DBSrv, BISrv). Dados de exemplo: `Release/`.
- Reaproveite o que já existe em `docs/agents/` (overview, arquitetura, deep-dives) — NÃO repita;
  referencie e aprofunde no que falta para migrar.

Produza os artefatos abaixo, NESTA ordem de prioridade. Cada um é um arquivo separado.

────────────────────────────────────────────────────────
FASE 1 — ESPECIFICAÇÃO DO PROTOCOLO  → docs/migration/protocol-spec.md   [PRIORIDADE MÁXIMA]
────────────────────────────────────────────────────────
Objetivo: permitir que um servidor em QUALQUER linguagem fale exatamente com o WYD.exe 7662.
Fonte: `CPSock.h`, `CPSock.cpp`, `Basedef.h`, `ClientPatch_v7662/`, todos os `TMSrv/_MSG_*.cpp`.

1. Camada de transporte:
   - Layout exato do `HEADER` (Size, KeyWord, CheckSum, Type, ID, ClientTick): offset de cada campo,
     tamanho em bytes, tipo, endianness, e o que cada um significa. Cite `CPSock.h`.
   - Algoritmo de encode/decode da keyword-table: descreva passo a passo, de forma reproduzível,
     incluindo onde fica a tabela de 512 bytes (`pKeyWord`) e como o índice é derivado. Cite
     `CPSock.cpp` (linhas do transform).
   - Algoritmo de checksum: fórmula exata. Documente que mismatch NÃO rejeita o pacote
     (`CPSock.cpp:458-466`) e que o ClientPatch desliga a verificação no cliente
     (`ClientPatch_v7662/Hook.cpp` JE→JMP). Diga o que o servidor novo deve fazer.
   - Handshake/INITCODE: o que é, valor, fluxo.
   - Enquadramento (framing): como mensagens são delimitadas no stream TCP, tamanho mínimo/máximo.

2. Catálogo de mensagens (o coração do documento):
   - Liste TODOS os `Type` de mensagem (constantes) cliente→servidor e servidor→cliente.
   - Para CADA mensagem: nome da constante + valor numérico, struct C correspondente, e tabela
     campo-a-campo (offset, tipo, tamanho, significado, validação esperada, exemplo de valor).
   - Marque a direção (C→S, S→C, ou ambos) e o handler responsável (`_MSG_*.cpp`).
   - Inclua os protocolos internos: TMSrv↔DBSrv (ProcessDBMessage) e TMSrv↔BISrv
     (`_AUTH_GAME` de 196 bytes).
   - Se houver mensagens parcialmente entendidas, marque UNVERIFIED com o que falta.

3. Apêndice: a tabela `pKeyWord` completa (ou ponteiro exato para ela) e quaisquer constantes de
   versão (`APP_VERSION`/`ClientVersion`) que o cliente 7662 exige.

────────────────────────────────────────────────────────
FASE 2 — FORMATOS DE DADOS / PERSISTÊNCIA  → docs/migration/data-formats.md
────────────────────────────────────────────────────────
Objetivo: ler os dados existentes e desenhar o novo schema (provável banco relacional/NoSQL).
Fonte: `Basedef.h` (structs), `DBSrv/CFileDB.cpp`, `CReadFiles.cpp`, e exemplos em `Release/`.

- Para cada arquivo/estrutura de persistência, documente o layout binário/textual exato:
  - Contas/personagens: `STRUCT_ACCOUNTFILE`, `STRUCT_ACCOUNTINFO` (inclua o campo de senha em
    texto plano `AccountPass` — marcar como dívida de segurança a corrigir na migração),
    `STRUCT_MOB` persistido, cargo, coin, quests.
  - Layout da pasta `account/` (ex.: `account/A/antonio`): convenção de nome/diretório.
  - Mapas: `HeightMap.dat` (16 MB) e `AttributeMap.dat` (1 MB) — dimensões, célula, semântica.
  - Conteúdo: `ItemList.csv`/`.bin`, `SkillData.csv`, `NPCGener.txt`, `data00.csv`, `extraitem.bin`,
    `GuildInfo`, `Guilds.txt`, `serverlist.bin`, `ItemDropList.txt`, `LevelItem.txt`.
- Para CADA um: campos, tipos, unidades, chaves/relacionamentos, e um trecho de exemplo real do
  `Release/`. Aponte invariantes (tamanhos fixos, índices, sentinelas).
- Proponha um modelo de dados alvo (tabelas/coleções) equivalente, marcando o que vira normalizado.

────────────────────────────────────────────────────────
FASE 3 — MODELO DE DOMÍNIO E ESTADO  → docs/migration/domain-model.md
────────────────────────────────────────────────────────
Objetivo: descrever as entidades vivas em memória e como o estado global é organizado.
Fonte: `Basedef.h`, `TMSrv/Server.h/.cpp`, `CUser.*`, `CItem.*`, `CMob.*`.

- Entidades: `STRUCT_MOB` (note que players e mobs compartilham a mesma struct/array `pMob[]`),
  `CUser`/`pUser[]`, `STRUCT_ITEM`, guilda, party, NPC.
- Estado global mutável: arrays globais, índices, limites (MAX_*), e como conexão↔player↔mob se
  relacionam por índice.
- Máquinas de estado (sessão do jogador, AI de mob, trade, war/castle): diagramas ASCII.
- Liste os pontos onde estado global compartilhado dificulta concorrência (relevante para escolher
  o modelo de concorrência da stack nova).

────────────────────────────────────────────────────────
FASE 4 — REGRAS DE JOGO E FÓRMULAS  → docs/migration/game-rules.md
────────────────────────────────────────────────────────
Objetivo: extrair a lógica de negócio hardcoded para pseudocódigo/tabelas, para reimplementar com
paridade. Fonte: `MobKilled.cpp`, `_MSG_UseItem.cpp`, `CItem.cpp`, `CMob.cpp`, `CCastleZakum.*`,
`CWarTower.*`, `ProcessSecMinTimer.cpp`, `Rates.txt`, `gameconfig.txt`, `Common/Settings/`.

- Curva de EXP e split de party (ex.: `MobKilled.cpp:483-504`) → tabela + pseudocódigo.
- Drop: como a tabela de drop é resolvida, bônus de evento, rates (`ItemDropList.txt`, gameconfig).
- Refino/combine de itens: tabela de taxas por nível, cooldown (note o anti-spam comentado em
  `_MSG_UseItem.cpp:209-221`), os ~10 handlers `_MSG_CombineItem*` (consolidar e marcar divergências).
- Combate: cálculo de dano, ataque/defesa, acerto/esquiva, regen, morte.
- Skills/efeitos: modelo de efeitos de item/skill (`ItemEffect.h`), delays (o ClientPatch divide
  SkillDelay por 4 — documentar o efeito).
- Eventos/timers: war (GTorreHour), RVR, castle (Zakum), quests diárias.
- Para cada regra: cite arquivo:linha, dê pseudocódigo determinístico, e liste as constantes mágicas.

────────────────────────────────────────────────────────
FASE 5 — CONTRATOS DE COMPORTAMENTO POR HANDLER  → docs/migration/handlers/_MSG_<nome>.md
────────────────────────────────────────────────────────
Objetivo: um contrato por handler para reimplementar e testar 1:1. Faça em lotes (não todos de uma vez).
Para CADA `_MSG_*.cpp` (58 arquivos), gere um arquivo com:
- Gatilho (qual Type/mensagem o aciona) e struct de entrada.
- Pré-condições e validações (bounds check de slot/posição, versão, estado do jogador).
- Efeitos colaterais (mutações de estado, persistência, mensagens disparadas a outros jogadores).
- Saídas (mensagens S→C produzidas) e casos de erro.
- Verificações anti-cheat (ex.: "cra point", CheckFailAccount) e o que acontece ao falhar.
- Riscos conhecidos (confiança em dados do cliente, dup de item, overflow) — referencie o relatório
  de arquitetura.
Comece pelos críticos: `_MSG_AccountLogin`, criação/seleção de personagem, movimento, ataque,
trade, drop/pickup, combine/refino.

────────────────────────────────────────────────────────
FASE 6 — FLUXOS PONTA-A-PONTA  → docs/migration/flows.md
────────────────────────────────────────────────────────
Diagramas de sequência (ASCII) para: login (Client→TMSrv→DBSrv→Client), seleção de personagem,
movimentação, ataque/morte/drop, trade entre jogadores, refino, guild war/torre, castle (Zakum),
e o caminho de billing (TMSrv→BISrv). Mostre mensagens, atores e ordem.

────────────────────────────────────────────────────────
FASE 7 — CONFIGURAÇÃO E OPERAÇÃO  → docs/migration/config-ops.md
────────────────────────────────────────────────────────
Catálogo de TODOS os arquivos de config em `Release/` (gameconfig.txt, Rates.txt, localip.txt,
biserver.txt, serverlist.txt, admin.txt, config.txt, settings.txt, Mac.txt, redirect.sample.txt):
cada campo, tipo, efeito, e valor de exemplo. Documente a topologia de deploy (3 processos, .bat de
auto-restart), IPs hardcoded (e por que isso precisa virar config/ambiente na stack nova), e o
controle de admin/MAC. Liste o que vira variável de ambiente/secret no sistema novo.

────────────────────────────────────────────────────────
FASE 8 — PARIDADE / GOLDEN CASES  → docs/migration/parity-tests.md
────────────────────────────────────────────────────────
Objetivo: bateria de casos para validar que o servidor novo == o atual (crítico no big-bang).
- Defina formato de caso: estado inicial + pacote(s) de entrada → estado final + pacote(s) de saída
  esperados. Inclua casos de borda e de erro.
- Liste casos concretos por subsistema (login ok/falho, criar/deletar char, mover, atacar, dropar,
  refinar com sucesso/falha, trade, war). Use as fórmulas da Fase 4.
- Descreva COMO capturar golden cases do servidor atual (ex.: usar o Wyd2Client em C# ou um proxy
  de captura entre WYD.exe e TMSrv para gravar pares request/response reais).
- Aponte fontes de não-determinismo (RNG de drop/refino) e como torná-las testáveis (seed).

────────────────────────────────────────────────────────
FASE 9 — RECOMENDAÇÃO DE STACK E PLANO DE MIGRAÇÃO  → docs/migration/migration-plan.md
────────────────────────────────────────────────────────
- Requisitos não-funcionais derivados do código: reator de I/O com alto fan-out (centenas de
  conexões), parsing de structs binárias, baixa latência, estado em memória, persistência.
- Compare candidatos (ex.: Rust/tokio, C#/.NET, Go, TypeScript/Node) contra esses requisitos:
  facilidade de mapear structs binárias, modelo de concorrência, ecossistema de rede, curva para o
  time, interop/risco. Faça uma RECOMENDAÇÃO fundamentada (com trade-offs explícitos).
- Sequência do big-bang: ordem de reconstrução (sugestão: protocolo+transporte → DBSrv/persistência
  → loop TMSrv → handlers por subsistema → conteúdo → war/castle/billing).
- Registro de riscos: compatibilidade de protocolo, paridade de fórmulas, formatos de dados,
  RNG, segurança (senha plaintext, checksum, keytable estática) — com mitigação.
- Critérios de "pronto para corte" (definition of done) ligados às golden cases da Fase 8.

────────────────────────────────────────────────────────
FASE 10 — GLOSSÁRIO  → docs/migration/glossary.md
────────────────────────────────────────────────────────
Termos de domínio WYD/PT-BR e do código (Anct, Refino, quest Arch/SD/SP/DK/CS, Castle/Zakum, RVR,
GTorre, cra point, capsule, cargo, mob vs NPC vs summon, etc.), cada um com definição e onde aparece.

ENTREGÁVEL FINAL: um docs/migration/README.md indexando todos os artefatos e marcando o status de
cada fase (COMPLETO / PARCIAL / UNVERIFIED), no mesmo estilo do MANIFEST.md de docs/agents/.
````

---

## Notas de uso

- **Rode em fases.** A Fase 1 (protocolo) e a Fase 2 (dados) são as que destravam a migração — faça
  e revise essas duas antes das demais. As Fases 4, 5 e 8 são as que garantem paridade no big-bang.
- **Fase 5 em lotes.** São 58 handlers; peça 5–8 por vez para o agente não estourar contexto.
- **Captura de golden cases (Fase 8):** o `Wyd2Client` (C#) e/ou um proxy TCP entre `WYD.exe` e o
  `TMSrv.exe` atual são o caminho prático para gravar pares request/response reais como fixtures.
- **Mantenha o estilo "evidência + file:line + UNVERIFIED"** dos docs em `docs/agents/` — é o que
  torna a documentação confiável para reescrever sem o código-fonte por perto.
