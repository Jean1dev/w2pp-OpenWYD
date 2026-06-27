# Mapa de hooks do ClientPatch_v7662

> **Propósito:** documentar **o que cada patch/hook da `ClientPatch_v7662.dll` faz no `WYD.exe` 7662**,
> por endereço de memória, para servir de referência quando formos **adicionar features ao cliente**
> (que é fechado — só temos o binário). Toda evidência é `arquivo:linha` em `Source/Code/ClientPatch_v7662/`.
>
> Complementa `protocol-spec.md` (lado servidor do mesmo cliente) e o deep-dive de CPSock em
> `docs/agents/`. O cliente roda **sem** alterações no disco; o ClientPatch modifica o processo **em
> runtime**.

---

## 0. Como funciona (mecânica)

O `WYD.exe` é um binário fechado (HanbitSoft/JoyImpact — fonte nunca liberada). O ClientPatch é uma
**DLL injetada no processo** que, em `DllMain` → `DLL_PROCESS_ATTACH`, **reescreve bytes da seção de
código do cliente em memória** — não recompila nada, faz *binary patching* + *trampoline hooking*.

Sequência de boot (`main.cpp:35-59`):

```c
VirtualProtect((void*)0x401000, 0x1F3000, PAGE_READWRITE, ...); // torna .text gravável
LoadHooks();                                                    // aplica todos os patches (Hook.cpp)
// guarda: só roda dentro de WYD.EXE lançado pelo launcher
VirtualProtect((void*)0x401000, 0x1F3000, <flag original>, ...); // restaura proteção
```

Há um guard anti-execução-avulsa: se o módulo host não for `WYD.EXE` (ou for o launcher), exibe
*"Execute o jogo através launcher."* e mata o processo (`main.cpp:49-55`).

### Dois tipos de patch

1. **Patch direto de byte** — sobrescreve uma instrução por endereço. Ex.: trocar um *jump
   condicional* por `JMP` incondicional (`0xEB`) ou por `NOP` (`0x90`) para desligar uma checagem.
2. **Trampoline hook** — redireciona o fluxo para uma função nossa (`NKD_*`) via os helpers de
   `PE_Hook.h`, que escrevem o opcode de salto + a distância relativa calculada
   (`BuildIndirection`, `PE_Hook.h:38`). A função `NKD_*` faz o trabalho e devolve o controle ao
   cliente com `PUSH <endereço de retorno>; RETN`.

### Helpers de hook (`PE_Hook.h`)

| Macro | Opcode escrito | Tamanho | Uso |
|---|---|---|---|
| `JMP_NEAR(addr, fn[, nop])` | `E9` (jmp rel32) | 5 B | salto incondicional p/ trampoline |
| `JE_NEAR` / `JZ_NEAR` | `0F 84` | 3–4 B | hook condicional "se igual/zero" |
| `JNZ_NEAR` | `0F 85` | 6 B | hook condicional "se diferente" |
| `JGE_NEAR` | `0F 8D` | 6 B | hook condicional "se ≥" |
| `JG_NEAR` | `0F 8F` | 6 B | hook condicional "se >" |
| `CALL_NEAR` | `E8` (call rel32) | 5 B | chamada p/ função nossa |
| `FillWithNop(addr, n)` | `90`×n | n | apaga bytes residuais da instrução substituída |
| `DEF_STR(str, addr)` | — | — | `strcpy` em endereço fixo (texto) |

> O parâmetro opcional `dwNopedSize` preenche com `NOP` os bytes da instrução original que sobraram
> além do salto, evitando código órfão. Ex.: `JMP_NEAR(0x422007, NKD_AddVolatileMessageItem, 2)`.

---

## 1. Patches diretos de byte (`Hook.cpp:LoadHooks`, `main.cpp`)

| # | Endereço | Escrita | O que faz | Evidência |
|---|---|---|---|---|
| 1 | `0x53AC6A` | `0xEB` | **Desliga verificação de checksum** (jz→jmp). Casa com o checksum **não-rejeitante** do CPSock; o tmServer Go assume isso (`-reject-checksum` off por padrão). | `Hook.cpp:212` |
| 2 | `0x53AD52` | `0xEB` | idem (2ª checagem de checksum) | `Hook.cpp:213` |
| 3 | `0x53AE7E` | `0xEB` | idem (3ª checagem de checksum) | `Hook.cpp:214` |
| 4 | `0x0054A331+6` | `0` | "Efeito de Buffs 6.13" — zera operando (ajuste visual de buff) | `Hook.cpp:204` |
| 5 | `0x0054A351+6` | `0` | idem | `Hook.cpp:205` |
| 6 | `0x00467651+6` | `0` | idem | `Hook.cpp:206` |
| 7 | `0x427213+6` | `0` | **"Força os gráficos"** — destrava modo gráfico | `Hook.cpp:209` |
| 8 | `0x11DA838+i*96+48` (×104) | `/= 4` | **Reduz SkillDelay** (loop sobre 104 entradas; divide o delay por 4) | `Hook.cpp:230-231` |
| 9 | `0x0622CDC` | `"WYD CDK"` | **Título da janela** do cliente | `Hook.cpp:234` |
| 10 | `0x5D8E8C` | `0x9090` (NOP×2) | **Mata conexão estranha** a `211.115.86.66:2424` (servidor coreano oficial) que só dava lag | `main.cpp:28-33` |
| 11 | `0x5D9491` | `0xEB` *(comentado)* | Slot para **desligar o XTrap** (anticheat). Atualmente desativado. | `main.cpp:23-26` |

---

## 2. Trampoline hooks (funções `NKD_*`)

| Hook (instalação) | Endereço alvo | Função | O que faz |
|---|---|---|---|
| `JGE_NEAR(0x4252D6, NKD_ReadMessage)` | `0x4252D6` | `ReadMessage` (`Functions.cpp:44`) | **Intercepta TODO pacote recebido** antes do cliente processar. Inspeciona `header->Type`; já trata `Type==0xD1D` (escreve `0xFF00CD00` em `0x4A016A`). **Ponto de extensão nº 1** para ensinar o cliente a entender pacotes novos do servidor Go. |
| `JMP_NEAR(0x4676C5, NKD_SendChat)` | `0x4676C5` | `SendChat` (`Functions.cpp:61`) | **Intercepta o chat digitado**. Hoje passa tudo adiante; o `if (*command != '@')` está comentado, pronto para implementar **comandos `@` client-side**. **Ponto de extensão nº 2.** |
| `JMP_NEAR(0x422007, NKD_AddVolatileMessageItem, 2)` | `0x422007` | `AddVolatileMessageItem` (`Functions.cpp:85`) | Marca itens que disparam **caixa de confirmação volátil**. Exemplo vivo: item `3314`. |
| `JMP_NEAR(0x41FB30, NKD_AddVolatileMessageBox, 5)` | `0x41FB30` | `SetVolatileMessageBoxText` (`Functions.cpp:74`) | Define o **texto da caixa**. Ex.: item `3314` → *"Deseja comer esse delicioso Frango Assado?"*. **Ponto de extensão nº 3** (UI/diálogos novos sem recompilar o cliente). |
| `JE_NEAR(0x04974C7, NKD_FixMageMacro)` / `JE_NEAR(0x04974D7, …)` | `0x04974C7` / `0x04974D7` | `NKD_FixMageMacro` (`Hook.cpp:22`) | **Fix do macro de mago** — distingue macro contínuo do normal. |
| (chamado por hooks) | `0x40126A` | `SendPack` (`Functions.cpp:24`) | Helper que **envia pacote** pelo socket do cliente (calcula o objeto de conexão: `conn*0xC58 + 0x752BAF8`). Use para **mandar pacotes a partir de um hook**. |
| (ponteiro fixo) | `0x54DA23` | `SendPacket` (`Main.h:32`) | Endereço da função de envio original do cliente. |

---

## 3. Os 4 pontos de extensão (como adicionar features no cliente)

Como **não temos a fonte do cliente**, estes hooks são a *única* via de customização do cliente gráfico:

1. **Receber/reescrever pacotes** → `ReadMessage` (`0x4252D6`). Adicione `else if (header->Type == ...)`
   para reagir a tipos novos do tmServer.
2. **Comandos de chat** → `SendChat` (`0x4676C5`). Filtre por prefixo (`@`, `#`) e dispare ações.
3. **Itens/diálogos/UI customizados** → `AddVolatileMessageItem` + `SetVolatileMessageBoxText`.
4. **Enviar pacotes do cliente** → `SendPack` (`0x40126A`).

Tweaks sem hook (patch direto): checksum off, força-gráficos, SkillDelay, título da janela, XTrap.

---

## 4. Avisos e limitações

- ⚠️ **Todos os endereços são específicos do build 7662.** Qualquer outra versão do `WYD.exe` quebra
  *todos* os offsets. Achar offsets novos exige **engenharia reversa** (x64dbg/OllyDbg/IDA).
- ⚠️ **Frágil por natureza:** escrever na `.text` em runtime; um offset errado quebra/crasha o cliente.
- O patch nº 1-3 (checksum off) é **pré-requisito** para o cliente falar com o tmServer Go, que não
  recalcula checksum válido por padrão. Ver `protocol-spec.md` §1.5.
- O XTrap (anticheat) está **desligado** (slot comentado em `main.cpp`), coerente com servidor privado.
- **Licença:** ClientPatch é **GPL-3.0** (© 2015 Victor Klafke, Charles TheHouse). Reaproveitar mantém
  a obrigação GPL — ver nota de herança em `RELATED-PROJECTS-COMPARISON` (docs/agents).

---

## 5. Referência rápida de arquivos

```
Source/Code/ClientPatch_v7662/
├── main.cpp       # DllMain: VirtualProtect + LoadHooks + guard de launcher; strip_xtrap/strip_odd_connection
├── Hook.cpp       # LoadHooks(): patches diretos + instalação dos trampolines; funções NKD_* em asm
├── Functions.cpp  # ReadMessage / SendChat / SetVolatileMessageBoxText / AddVolatileMessageItem / SendPack
├── Main.h         # protótipos + ponteiro SendPacket(0x54DA23); inclui Basedef.h (structs do protocolo)
└── PE_Hook.h      # helpers JMP_/JE_/JGE_/CALL_NEAR, FillWithNop, BuildIndirection (motor do hooking)
```
</content>
