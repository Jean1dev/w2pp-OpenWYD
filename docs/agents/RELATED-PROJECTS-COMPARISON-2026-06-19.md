# Projetos Relacionados — Análise Comparativa

**Gerado em:** 2026-06-19
**Base:** documentos em `docs/agents/` (PROJECT-OVERVIEW, architectural-report, component-deep-dives) confrontados com três repositórios GitHub.

## Projetos analisados

| # | Repositório | Linguagem | O que é | Licença |
|---|---|---|---|---|
| 1 | [seitbnao/W2PP](https://github.com/seitbnao/W2PP) | C++ (VS2015) | Emulador de servidor WYD a partir do decompile do *Polly's Server Release* | GPL-3.0 |
| 2 | [open-wyd/open-wyd-scripts](https://github.com/open-wyd/open-wyd-scripts) | Lua 5.3 (+XML) | Camada de scripts data-driven para o engine "Open WYD" | — |
| 3 | [kevinkouketsu/Wyd2Client](https://github.com/kevinkouketsu/Wyd2Client) | C# (WPF/MVVM) | Cliente/simulador + biblioteca de rede do protocolo WYD | educacional |

---

## Resumo executivo

Os três projetos **têm relação com este projeto**, mas em níveis muito diferentes:

- **W2PP é o ancestral direto deste repositório.** O próprio nome da pasta — `w2pp-OpenWYD` — é a fusão de **W2PP** + **OpenWYD**. A árvore `Source/Code/` aqui é estruturalmente idêntica à de W2PP. Não é "semelhante": é a mesma base de código, com as alterações do autor (fork do "WYD cdk / Cavaleiros de Kersef").
- **open-wyd-scripts representa o "futuro arquitetural"** que este projeto não tem: conteúdo de jogo (eventos, itens, NPCs, teleportes, spawns) descrito em **Lua + XML** em vez de hardcoded em C++. Complementa por contraste — mostra exatamente o que falta aqui.
- **Wyd2Client complementa pelo lado cliente/protocolo:** é uma reimplementação em C# do mesmo protocolo de fio (HEADER + keyword-table) que o `CPSock` deste projeto. Serve como documentação viva do protocolo e como base para bots/testes de carga.

---

## 1. seitbnao/W2PP — **Ancestral direto / fork irmão**

### Evidência da relação (não é coincidência, é a mesma base)

A raiz `Code/` de W2PP e a `Source/Code/` deste projeto têm exatamente os mesmos arquivos e módulos centrais:

| W2PP (`Code/`) | Este projeto (`Source/Code/`) |
|---|---|
| `Basedef.cpp` / `Basedef.h` | `Basedef.cpp` / `Basedef.h` |
| `CPSock.cpp` / `CPSock.h` | `CPSock.cpp` / `CPSock.h` |
| `ItemEffect.h` | `ItemEffect.h` |
| `ClientPatch_v7662/` | `ClientPatch_v7662/` |
| `DBSrv/` | `DBSrv/` |
| `TMSrv/` | `TMSrv/` |

As ferramentas standalone descritas no README do W2PP (DBSrv, TMSrv, DropTool, ExpTool, NPTool, EDITAPPMOB/EDITAPNPC, AttributeMap_Editor) são as **mesmas** listadas no `architectural-report` deste projeto (seção 2.1).

### Linhagem reconstruída

```
Polly's Server Release (decompile, © Hanbitsoft)
        │  (Klafke + TheHouse)
        ▼
   W2PP (seitbnao, mantido por Luis Gustavo / Woz Farias / Eric Santos)
        │
        ▼
   "WYD cdk" / Cavaleiros de Kersef  (Cavaleiros de Kersef.sln)
        │  (fork de Jeanluca)
        ▼
   w2pp-OpenWYD  ← ESTE PROJETO
```

O README local confirma: *"fork do WYD cdk com as minhas alterações"*, e a solution chama-se `Cavaleiros de Kersef.sln`.

### Semelhanças (herdadas)
- Mesmo modelo: cluster de 3 processos (TMSrv/DBSrv/BISrv), reator single-thread com `WSAAsyncSelect`.
- Mesmo protocolo `CPSock` (HEADER + tabela `pKeyWord` de 512 bytes + checksum).
- Mesma persistência file-based (`STRUCT_ACCOUNTFILE`, senha em texto plano).
- Mesmos riscos catalogados no `architectural-report` (checksum que não rejeita pacote, key table estática compartilhada, senhas em plaintext, IP de billing hardcoded).

### O que cada um traz de diferente
- **W2PP** é mais "limpo/upstream" (5 arquivos + 3 pastas no `Code/`, sem BISrv no tree público, sem os editores extras).
- **Este projeto** está **à frente** de W2PP: tem BISrv, os editores adicionais (ZerarSkill, SearchPass, EDITAPPSHOP), conteúdo de NPC/skills atualizado (vide commits recentes "NPCS Atualizados", "Corrigido Skill Exterminar") e toolset mais novo (v143/VS2022 vs VS2015 do W2PP).

### O que faz sentido aproveitar do W2PP
- **Diff upstream:** comparar `Basedef.h`/`CPSock.cpp` linha a linha pode revelar correções de bugs feitas pela comunidade do W2PP (mantido até fev/2025) que ainda não estão aqui.
- **`license.txt` + GPL-3.0:** como este é um fork de base GPL-3.0, vale confirmar a conformidade de licenciamento (o repo local tem `LICENSE`, mas a herança GPL deveria ser explicitada).

---

## 2. open-wyd/open-wyd-scripts — **O modelo data-driven que falta aqui**

### Relação
Pertence à organização **open-wyd** (open-wyd-scripts, owlauncher, open-wyd-forum, ows-clientdata, e um `open-wyd` privado/sem descrição). É a parte "OpenWYD" do nome deste repositório. Diferente do W2PP, **não é a mesma base de código** — é uma camada de scripting (Lua 5.3 + XML) para um engine "Open WYD" cujo núcleo C/C++ não é público.

### O que traz de novo (e que este projeto NÃO tem)
O engine alvo do open-wyd-scripts separa **conteúdo** de **binário**: eventos, itens, mercadores/NPCs, teleportes, spawns e comportamento de criaturas são descritos em Lua + XML, com APIs como `iGameServer`, `iSend`, `FindItem`. É o modelo TFS/OpenTibia aplicado a WYD.

Isso ataca diretamente as dívidas técnicas apontadas no `PROJECT-OVERVIEW` / component deep-dives deste projeto:

| Dívida técnica catalogada aqui | Como o modelo OpenWYD resolveria |
|---|---|
| Curva de EXP hardcoded em faixas de nível (`TMSrv/MobKilled.cpp:483-504`) | Tabela/Lua data-driven |
| 10 handlers `_MSG_CombineItem*` quase duplicados | Lógica em script reutilizável |
| `Server.cpp` ~10.5k linhas, `MobKilled.cpp` função de 3.5k linhas | Regras movidas para scripts |
| 58 arquivos `_MSG_*.cpp` (toda regra em C++) | Eventos/itens scriptáveis sem recompilar |
| Conteúdo só editável via ferramentas binárias (DropTool, ExpTool…) | Arquivos Lua/XML versionáveis em git |

### Complementa / coisas que fazem sentido para este projeto
- **Adotar um interpretador Lua embutido** no TMSrv seria a evolução natural — permitiria mover balanceamento/eventos para fora do C++.
- **owlauncher** (atualiza IP dos canais via DNS) resolve diretamente um problema apontado no `architectural-report`: IPs hardcoded em `localip.txt`/`serverlist.txt`/`biserver.txt` e nas pastas `serverlist bin/` por IP.
- **ows-clientdata** pode ser referência para empacotar os dados de cliente que aqui estão soltos em `Release/Common/`.

### Coisas faltantes (gap)
Este é o maior gap arquitetural: o projeto local é **100% hardcoded**, sem nenhuma camada de script — exatamente o oposto da filosofia OpenWYD. Migrar não é trivial (engines diferentes), mas o open-wyd-scripts serve como **blueprint da arquitetura-alvo**.

---

## 3. kevinkouketsu/Wyd2Client — **Lado cliente / referência de protocolo**

### Relação
Cliente/simulador WYD em **C# (WPF, MVVM)** com biblioteca de rede reutilizável. Kevin Kouketsu é desenvolvedor conhecido no reverse-engineering de WYD. Conecta-se ao **WYD Global (RaidHut oficial)**, mas implementa o **mesmo protocolo de fio** que o `CPSock` deste projeto (HEADER + estruturas WYD). A dependência `Wyd2.Common` é creditada a André Santa Cruz (ptr0x), com "estruturas básicas do WYD".

### Estrutura
- `W2Open.Common` — utilitários e estruturas do protocolo WYD (espelho C# do `Basedef.h`/`CPSock`).
- `Wyd2.Network` — camada de rede orientada a eventos (dispatcher).
- `Wyd2Client` — app WPF (login, criar/deletar personagem, movimento, teleporte, chat normal/sussurro, sistema de macros extensível).

### Como complementa este projeto
- **Documentação viva do protocolo:** o `architectural-report` aponta que o protocolo só existe como `CPSock.cpp` em C++; o Wyd2Client é uma segunda implementação independente do mesmo wire format — ótima para validar/entender os pacotes e as `STRUCT_*` de `Basedef.h`.
- **Cliente de teste / bot / load-test:** o projeto local não tem **nenhum teste automatizado** (apontado no PROJECT-OVERVIEW). Um cliente programável em C# permite scripts de fumaça (login, criar char, mover, chat) contra o TMSrv local — preenche a lacuna de testes ponta-a-ponta.
- **Padrão de macros/dispatcher** é reaproveitável para automação de QA.

### O que traz de novo
Arquitetura limpa (MVVM, dispatcher orientado a eventos) e a ideia de **desacoplar a biblioteca de protocolo** do app — algo que aqui está fundido em `CPSock`/`Basedef` (alto acoplamento, `Basedef.h` incluído por 16 arquivos).

### Coisas faltantes / atenção
- Conecta no servidor **oficial RaidHut**, então as estruturas podem divergir da versão 7.59/7662 usada aqui — exigiria ajuste de offsets/versão antes de apontar para o TMSrv local.
- Limitação conhecida: problemas de sincronização de threads entre envio e UI.

---

## Matriz consolidada

| Projeto | Tipo de relação | Mesma base de código? | Complementa | Traz de novo | Aplicável aqui |
|---|---|---|---|---|---|
| **W2PP** | Ancestral / fork irmão | **Sim** (idêntica) | — (é a origem) | Correções upstream da comunidade | Diff/merge de fixes; conformidade GPL |
| **open-wyd-scripts** | Mesma "família" OpenWYD, engine distinto | Não | Sim (arquitetura-alvo) | Scripting Lua+XML data-driven; owlauncher (DNS); ows-clientdata | Blueprint p/ tirar regras do C++; resolver IPs hardcoded |
| **Wyd2Client** | Lado cliente do mesmo protocolo | Não (porta C#) | Sim (cliente/protocolo) | Lib de protocolo desacoplada; cliente programável | Doc de protocolo; cliente de teste/bot; cobrir lacuna de testes |

## Recomendações priorizadas para w2pp-OpenWYD

1. **Sincronizar com W2PP upstream** — diff em `Basedef.*`/`CPSock.*` para puxar correções da comunidade (W2PP foi mantido até 2025); registrar a herança GPL-3.0.
2. **Resolver IPs hardcoded** — adotar a abordagem do **owlauncher** (DNS) no lugar de `biserver.txt`/`localip.txt`/`serverlist bin/` fixos (risco "High" no architectural-report).
3. **Testes ponta-a-ponta** — usar **Wyd2Client** (ajustado p/ versão 7662/7.59) como cliente automatizado para cobrir a ausência total de testes.
4. **Roadmap de scripting** — usar **open-wyd-scripts** como referência para, a médio prazo, migrar balanceamento (curva de EXP, drops, eventos) de C++ hardcoded para dados/script, reduzindo `Server.cpp`/`MobKilled.cpp` e os 58 `_MSG_*.cpp`.

---

*Observações baseadas nos READMEs/metadados públicos dos repositórios (jun/2026) e nos relatórios em `docs/agents/`. O núcleo C/C++ do engine "Open WYD" não é público; a comparação com open-wyd-scripts é arquitetural, não de código.*
