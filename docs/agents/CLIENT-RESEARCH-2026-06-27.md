# Pesquisa — Cliente WYD modificável / customizável

**Gerado em:** 2026-06-27
**Autor da pesquisa:** Claude Code (Opus 4.8)
**Objetivo:** Procurar na internet um cliente de WYD (With Your Destiny) que possamos **alterar** (modificar o
código) ou, no mínimo, **customizar** (editar dados/comportamento), para casar com o servidor Go deste projeto.
**Atenção a links maliciosos:** cada fonte abaixo recebeu uma avaliação de risco. Nenhum download de binário foi
executado; a análise é baseada apenas em metadados públicos das páginas (READMEs/GitHub).

---

## 0. Contexto do projeto (por que isso importa)

Este repositório é uma **reescrita em Go** do servidor WYD que mira o **cliente `WYD.exe` build 7662**
(protocolo **7640**, também referido como "7.59"), **sem modificá-lo** — o tmServer fala o protocolo CPSock legado
(HEADER de 12 bytes + tabela de obfuscação `pKeyWord` + checksum não-rejeitante). O repo já contém
`Source/Code/ClientPatch_v7662` (uma DLL que "destrava" o cliente oficial, NOP'ando os checks de checksum).

Portanto "um cliente que podemos alterar" tem três interpretações práticas, e a pesquisa cobre as três:

1. **Cliente gráfico 3D com código-fonte aberto** (alterar de verdade o jogo) — *o que seria ideal*.
2. **Cliente oficial (`WYD.exe`) + DLL de patch** (alterar comportamento por binary-patching em runtime) — *o caminho atual do projeto*.
3. **Reimplementação de rede / simulador de cliente** (sem gráficos, mas 100% editável) — *bom para testes/bots/QA*.

---

## 1. Conclusão principal (resumo executivo)

> **Não existe um cliente gráfico 3D de WYD com código-fonte aberto.** O cliente original (`WYD.exe`, engine 3D
> proprietária da HanbitSoft/JoyImpact) **nunca teve a fonte liberada**. Tudo que a comunidade produziu se enquadra
> em: (a) DLLs de *patch* que alteram o binário oficial em runtime, (b) editores de arquivos de dados do cliente
> (ex.: `ItemList.bin`), e (c) **simuladores de rede headless** (C# e C++) que falam o protocolo mas não renderizam
> o jogo.

Implicação prática para este projeto: **"customizar o cliente" = patch da `WYD.exe` oficial (via ClientPatch, que já
temos) + edição dos arquivos de conteúdo do cliente.** "Alterar o cliente de verdade" (código) só é viável hoje no
nível do **simulador de rede** (Wyd2Client / WYD2Bot), que serve melhor como cliente de teste/bot do que como cliente
de jogo.

---

## 2. Candidatos encontrados (com avaliação de risco)

Legenda de risco: 🟢 baixo (fonte aberta, sem binários executáveis suspeitos) · 🟡 médio (contém binários
pré-compilados ou propósito dual-use) · 🔴 alto (evitar / sinais de risco).

### 2.1 — Simuladores de rede / clientes programáveis (categoria mais útil para "alterar")

| Projeto | Linguagem | Versão alvo | Licença | Tipo | Risco |
|---|---|---|---|---|---|
| [kevinkouketsu/Wyd2Client](https://github.com/kevinkouketsu/Wyd2Client) | C# (WPF/MVVM) | WYD Global (RaidHut) | educacional | Simulador de cliente com UI | 🟢 |
| [kevinkouketsu/WYD2Bot](https://github.com/kevinkouketsu/WYD2Bot) | C/C++ | 759+ | GPL-3.0 | Cliente headless (bot) | 🟡 |

**Wyd2Client** 🟢 — É o candidato mais alinhado com "um cliente que podemos alterar". App C#/WPF limpo (MVVM,
dispatcher orientado a eventos) dividido em `W2Open.Common` (estruturas do protocolo — espelho C# do `Basedef.h`),
`Wyd2.Network` (rede) e `Wyd2Client` (UI: login, criar/deletar/selecionar char, movimento, teleporte `#tele`, chat
normal/sussurro, sistema de macros extensível). README declara "apenas para fins educacionais". **Sem binários
executáveis** no tree; compila no Visual Studio. **Ressalva técnica:** conecta no servidor **oficial RaidHut**, então
as `STRUCT_*`/offsets podem divergir do 7662 usado aqui — exigiria ajuste de versão/offsets antes de apontar para o
tmServer local. Já está catalogado em `RELATED-PROJECTS-COMPARISON-2026-06-19.md` como "lado cliente do mesmo
protocolo".

**WYD2Bot** 🟡 — Cliente **sem interface gráfica** (só rede), C/C++, GPL-3.0. Faz auto-login, seleção de char,
reconhecimento de mobs, macros de combate/buff, pathfinding. Suporta "client 759 ou superior". README avisa que é
preciso "atualizar com o hash usado na versão 756+" antes de compilar. **Sem binários pré-compilados.** Risco 🟡
apenas porque é, na prática, um **bot** — dual-use; ótimo para load-test/QA do nosso servidor, mas usar contra
servidores de terceiros viola ToS. Para *este* projeto (testar o nosso tmServer) é um uso legítimo.

### 2.2 — Cliente oficial + patches (caminho de "customização" atual do projeto)

| Projeto | O que é | Licença | Risco |
|---|---|---|---|
| `Source/Code/ClientPatch_v7662` (neste repo) | DLL que destrava o cliente oficial 7662 | GPL-3.0 (herdado) | 🟢 (já no nosso repo) |
| [seitbnao/W2PP](https://github.com/seitbnao/W2PP) | Ancestral deste projeto; inclui `ClientPatch_v7662` (fonte) | GPL-3.0 | 🟢 |
| [tiagosucci/WYDGlobal_W2ppPatch](https://github.com/tiagosucci/WYDGlobal_W2ppPatch) | Patch para WYD Global 763 | — | 🟡 (não auditado a fundo) |
| [nikkw/WYD2-CrackPatch-v7556](https://github.com/nikkw/WYD2-CrackPatch-v7556) · [Amatsukan/ (fork)](https://github.com/Amatsukan/WYD2-CrackPatch-v7556) | "Crack patch" da v7556 | GPL-3.0 | 🟡 |

**W2PP** 🟢 — O **ancestral direto** deste repositório (já documentado em `RELATED-PROJECTS-COMPARISON`). Do lado
*cliente*, só contém o **`ClientPatch_v7662`** (DLL "to unlock Client") como **código-fonte**, não binário. Não há
fonte de cliente gráfico. Útil para diff/sincronização do nosso próprio ClientPatch.

**WYD2-CrackPatch-v7556** 🟡 — Código-fonte (C++/C, GPL-3.0) de um patch para a v7556, **mas o diretório `/bin`
contém executáveis/DLLs pré-compilados**. O nome "CrackPatch" indica contorno de proteção/checks do cliente — é
exatamente o tipo de coisa que o nosso `ClientPatch` faz (NOP de checksum), então o *conceito* é legítimo para
servidor privado. **Risco 🟡:** não rode os binários do `/bin` sem auditar/recompilar a partir da fonte. Versão 7556
≠ 7662, então só serve como **referência**, não plug-and-play.

### 2.3 — Editores de dados do cliente (customização de conteúdo, sem mexer no engine)

| Projeto | Edita | Licença | Risco |
|---|---|---|---|
| [Rechdan/WYD-ItemList-Editor](https://github.com/Rechdan/WYD-ItemList-Editor) | `ItemList.bin` do cliente | GPL-3.0 | 🟢 |

**WYD-ItemList-Editor** 🟢 — Ferramenta C# que edita o `ItemList.bin` (catálogo de itens lido pelo cliente).
Última atualização 2018, projeto pequeno (compila da fonte; sem binário pré-buildado evidente). É o caminho mais
seguro para **customizar conteúdo visível no cliente** sem tocar no engine. Note que customizar dados do cliente em
geral exige **paridade com os dados do servidor** (`Release/Common/`).

### 2.4 — Servidores (não são clientes, mas apareceram muito na busca)

Para registro: [venantivs/snalmir](https://github.com/venantivs/snalmir) (server C, v7.54, GPL-3.0, **só servidor,
sem cliente**), [Rechdan/Open-WYD-Server](https://github.com/Rechdan/Open-WYD-Server) e
[Rechdan/W2.Server.Node](https://github.com/Rechdan/W2.Server.Node) (Node.js), [nikkw/Secrets-Of-Destiny-WYD](https://github.com/nikkw/Secrets-Of-Destiny-WYD).
🟢 quanto a malware, mas **fora do escopo** (são servidores) — não fornecem cliente.

### 2.5 — Cliente oficial original

[wyd.t3fun.com/downloads/client.asp](https://wyd.t3fun.com/downloads/client.asp) é o **publisher oficial legítimo**
(T3Fun/HanbitSoft). 🟢 quanto à origem, **porém:** os servidores oficiais NA fecharam em 2015; o domínio segue de pé
mas pode estar desatualizado/sem manutenção. É a origem "limpa" do `WYD.exe` para então aplicar o ClientPatch. **Não**
é um cliente *alterável por código* (binário fechado).

---

## 3. Avaliação de links maliciosos

- **Nenhum download de binário foi executado** durante esta pesquisa; nenhuma URL foi aberta além de páginas de
  repositório/publisher.
- **Sinais a vigiar (não confirmados como maliciosos, mas exigem cautela):** repositórios com `/bin` contendo
  `.exe`/`.dll` pré-compilados — especialmente `WYD2-CrackPatch-v7556` e `WYDGlobal_W2ppPatch`. **Regra:** sempre
  **recompilar a partir da fonte**; nunca executar a DLL/EXE shippada sem auditoria (cracks de cliente são vetor
  clássico de malware na cena de servidores privados).
- **Não encontrei** nenhum "cliente WYD" hospedado fora do GitHub (fóruns, MEGA, mediafire) nesta rodada — esses
  seriam os de **maior risco** e devem ser evitados.
- **Avisos de ToS/legais:** WYD2Bot é um bot (dual-use); patches de cliente removem proteções — legítimos só para
  **servidor privado próprio / pesquisa**, nunca contra servidores oficiais de terceiros.

---

## 4. Recomendações priorizadas

1. **Para testes ponta-a-ponta do tmServer (recomendado):** adotar **Wyd2Client** 🟢 (C#) como cliente
   programável, ajustando offsets/`STRUCT_*` do RaidHut para o nosso 7662. Cobre a lacuna total de testes
   automatizados apontada no `PROJECT-OVERVIEW`. Alternativa headless para load-test/bot: **WYD2Bot** 🟡.
2. **Para "jogar de verdade":** continuar com o **`WYD.exe` oficial + nosso `ClientPatch_v7662`** (já no repo). É a
   única forma de cliente *gráfico*. "Customização" = patch (NOP de checks) + edição de dados do cliente.
3. **Para customizar conteúdo do cliente:** **WYD-ItemList-Editor** 🟢 (`ItemList.bin`), mantendo paridade com
   `Release/Common/`.
4. **Referência de patch:** comparar nosso `ClientPatch_v7662` com o de **W2PP** 🟢 e (com cautela, só a fonte) com
   **WYD2-CrackPatch-v7556** 🟡.
5. **Evitar/auditar:** quaisquer **binários pré-compilados** de repos de "crack/patch" — recompilar sempre.

---

## 5. Lacuna identificada (oportunidade futura)

Não existe cliente WYD **open-source e gráfico**. A única forma de ter um cliente totalmente alterável seria
**escrever um do zero** sobre `Wyd2.Network` (Wyd2Client) — caminho caro. A curto prazo, o par
**`WYD.exe` oficial + ClientPatch (gráfico) / Wyd2Client (testável)** cobre as duas necessidades sem reinventar o
engine.

---

### Fontes

- [kevinkouketsu/Wyd2Client](https://github.com/kevinkouketsu/Wyd2Client)
- [kevinkouketsu/WYD2Bot](https://github.com/kevinkouketsu/WYD2Bot)
- [seitbnao/W2PP](https://github.com/seitbnao/W2PP)
- [Rechdan/WYD-ItemList-Editor](https://github.com/Rechdan/WYD-ItemList-Editor)
- [nikkw/WYD2-CrackPatch-v7556](https://github.com/nikkw/WYD2-CrackPatch-v7556) · [Amatsukan fork](https://github.com/Amatsukan/WYD2-CrackPatch-v7556)
- [tiagosucci/WYDGlobal_W2ppPatch](https://github.com/tiagosucci/WYDGlobal_W2ppPatch)
- [venantivs/snalmir](https://github.com/venantivs/snalmir) · [Rechdan/Open-WYD-Server](https://github.com/Rechdan/Open-WYD-Server) · [Rechdan/W2.Server.Node](https://github.com/Rechdan/W2.Server.Node) · [nikkw/Secrets-Of-Destiny-WYD](https://github.com/nikkw/Secrets-Of-Destiny-WYD)
- [Cliente oficial — wyd.t3fun.com](https://wyd.t3fun.com/downloads/client.asp)
</content>
</invoke>
