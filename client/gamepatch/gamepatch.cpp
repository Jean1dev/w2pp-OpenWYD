// GamePatch.dll — cores nas linhas do tooltip de montaria do WYD.exe 7662.
//
// O executável já carrega este DLL sozinho: o ponto de entrada foi desviado para
// um carregador (0x5F3C66) que chama LoadLibraryA("GamePatch.dll"), depois
// "Shield.dll", depois "ClientPatch.dll", e só então entra no jogo. Nenhum dos
// dois primeiros existe no cliente, então basta este arquivo estar na pasta —
// sem mexer no WYD.exe nem no ClientPatch.dll (cujo código no repositório não é
// exatamente o que está compilado no cliente, e por isso não é recompilado).
//
// O que ele faz: envolve a função que monta o tooltip de item (0x416A80). O
// cliente monta tudo normalmente; no fim, se o tooltip tiver alguma linha de
// "Absorção" — só montaria tem —, as linhas de dano, magia e absorção são
// repintadas. Espada, armadura e todo o resto continuam como o cliente desenha.
//
// Como ele sabe o texto de cada linha sem decifrar onde o controle guarda a
// string: intercepta o método SetText (vtable +0x80) das 13 linhas do tooltip e
// anota o texto no momento em que o cliente o escreve. A cor é aplicada depois,
// pelo próprio SetColor (vtable +0x84) do controle.
//
// Tudo o que ele grava na memória é conferido antes: se os bytes do cliente não
// forem os da build 7662, ele não faz nada. Um tooltip sem cor é um incômodo;
// um gancho no lugar errado derruba o jogo.

#include <windows.h>
#include <string.h>

namespace {

// Endereços da build 7662 (WYD.exe de 2 347 008 bytes).
constexpr DWORD kTooltipFunc = 0x416A80; // função que monta o tooltip de item
constexpr DWORD kTooltipBody = 0x416A88; // primeira instrução depois dos 8 bytes copiados
constexpr DWORD kUIRoot = 0x6F0AB0;      // ponteiro para a janela que tem as linhas
constexpr DWORD kLinesOffset = 0x27A10;  // lista de ponteiros para as 13 linhas
constexpr int kLines = 13;               // controles 0x305..0x321
constexpr int kSetTextSlot = 0x80 / 4;
constexpr int kSetColorSlot = 0x84 / 4;

// Os oito primeiros bytes da função: push ebp / mov ebp,esp / mov eax,0x1498.
constexpr BYTE kExpectedPrologue[8] = {0x55, 0x8B, 0xEC, 0xB8, 0x98, 0x14, 0x00, 0x00};

// As cores, em ARGB.
constexpr DWORD kColorDamage = 0xFF66FF66; // verde
constexpr DWORD kColorMagic = 0xFF66B2FF;  // azul
constexpr DWORD kColorAbsorb = 0xFFFF5555; // vermelho

// Os rótulos como o cliente os escreve (Windows-1252).
const char kLabelDamage[] = "Aumento de Dano";
const char kLabelMagic[] = "Ataque M\xe1gico";
const char kLabelAbsorb[] = "Absor\xe7\xe3o";

typedef void(__fastcall* SetTextFn)(void* self, void* edx, const char* text, int flag);
typedef void(__fastcall* SetColorFn)(void* self, void* edx, DWORD color);

// Cada classe de controle tem a sua vtable; as 13 linhas podem não ser todas da
// mesma. Guarda o SetText original de cada vtable que for trocada.
struct HookedVtable {
    void** vtable;
    SetTextFn original;
};
HookedVtable g_vtables[8];
int g_vtableCount = 0;
bool g_linesHooked = false;

// O que foi escrito em cada linha durante o tooltip atual.
DWORD g_lineColor[kLines];
bool g_tooltipHasAbsorb = false;

bool StartsWith(const char* text, const char* prefix) {
    return text != nullptr && strncmp(text, prefix, strlen(prefix)) == 0;
}

void** LineControls() {
    DWORD root = *reinterpret_cast<DWORD*>(kUIRoot);
    if (root == 0) {
        return nullptr;
    }
    return reinterpret_cast<void**>(root + kLinesOffset);
}

int LineIndexOf(void* control) {
    void** lines = LineControls();
    if (lines == nullptr) {
        return -1;
    }
    for (int i = 0; i < kLines; i++) {
        if (lines[i] == control) {
            return i;
        }
    }
    return -1;
}

SetTextFn OriginalSetTextFor(void* control) {
    void** vtable = *reinterpret_cast<void***>(control);
    for (int i = 0; i < g_vtableCount; i++) {
        if (g_vtables[i].vtable == vtable) {
            return g_vtables[i].original;
        }
    }
    return nullptr;
}

void __fastcall HookedSetText(void* self, void* edx, const char* text, int flag) {
    int line = LineIndexOf(self);
    if (line >= 0) {
        DWORD color = 0;
        if (StartsWith(text, kLabelDamage)) {
            color = kColorDamage;
        } else if (StartsWith(text, kLabelMagic)) {
            color = kColorMagic;
        } else if (StartsWith(text, kLabelAbsorb)) {
            color = kColorAbsorb;
            g_tooltipHasAbsorb = true;
        }
        g_lineColor[line] = color;
    }
    SetTextFn original = OriginalSetTextFor(self);
    if (original != nullptr) {
        original(self, edx, text, flag);
    }
}

// Troca o SetText das vtables das linhas. Feito na primeira vez que um tooltip
// é aberto, porque é só então que os controles existem.
void HookLineControls() {
    void** lines = LineControls();
    if (lines == nullptr) {
        return;
    }
    for (int i = 0; i < kLines; i++) {
        if (lines[i] == nullptr) {
            return; // a janela ainda não terminou de ser montada; tenta no próximo
        }
    }
    for (int i = 0; i < kLines; i++) {
        void** vtable = *reinterpret_cast<void***>(lines[i]);
        bool known = false;
        for (int v = 0; v < g_vtableCount; v++) {
            known = known || g_vtables[v].vtable == vtable;
        }
        if (known) {
            continue;
        }
        if (g_vtableCount == ARRAYSIZE(g_vtables)) {
            return;
        }
        DWORD oldProtect = 0;
        if (!VirtualProtect(&vtable[kSetTextSlot], sizeof(void*), PAGE_READWRITE, &oldProtect)) {
            return;
        }
        g_vtables[g_vtableCount].vtable = vtable;
        g_vtables[g_vtableCount].original = reinterpret_cast<SetTextFn>(vtable[kSetTextSlot]);
        g_vtableCount++;
        vtable[kSetTextSlot] = reinterpret_cast<void*>(&HookedSetText);
        VirtualProtect(&vtable[kSetTextSlot], sizeof(void*), oldProtect, &oldProtect);
    }
    g_linesHooked = true;
}

void __cdecl BeforeTooltip() {
    if (!g_linesHooked) {
        HookLineControls();
    }
    for (int i = 0; i < kLines; i++) {
        g_lineColor[i] = 0;
    }
    g_tooltipHasAbsorb = false;
}

void __cdecl AfterTooltip() {
    // Só o tooltip de montaria tem "Absorção"; nos outros nada muda.
    if (!g_tooltipHasAbsorb) {
        return;
    }
    void** lines = LineControls();
    if (lines == nullptr) {
        return;
    }
    for (int i = 0; i < kLines; i++) {
        if (g_lineColor[i] == 0 || lines[i] == nullptr) {
            continue;
        }
        void** vtable = *reinterpret_cast<void***>(lines[i]);
        reinterpret_cast<SetColorFn>(vtable[kSetColorSlot])(lines[i], nullptr, g_lineColor[i]);
    }
}

// Executa os oito bytes que o gancho cobriu e segue para o resto da função.
__declspec(naked) void TooltipTrampoline() {
    __asm {
        push ebp
        mov ebp, esp
        mov eax, 0x1498
        push 0x416A88 // kTooltipBody, literal: em asm inline um nome viraria leitura de memória
        ret
    }
}

// A função original é __thiscall com três argumentos e termina em ret 0xC.
// O gancho repassa os três, chama a original pelo trampolim e pinta as linhas.
__declspec(naked) void TooltipHook() {
    __asm {
        push ecx
        call BeforeTooltip
        pop ecx
        push dword ptr [esp + 12]
        push dword ptr [esp + 12]
        push dword ptr [esp + 12]
        call TooltipTrampoline
        push eax
        call AfterTooltip
        pop eax
        ret 0xC
    }
}

bool InstallTooltipHook() {
    BYTE* target = reinterpret_cast<BYTE*>(kTooltipFunc);
    if (memcmp(target, kExpectedPrologue, sizeof(kExpectedPrologue)) != 0) {
        return false; // outra build: melhor sem cor do que com o jogo caindo
    }
    DWORD oldProtect = 0;
    if (!VirtualProtect(target, 5, PAGE_EXECUTE_READWRITE, &oldProtect)) {
        return false;
    }
    target[0] = 0xE9; // jmp rel32
    *reinterpret_cast<DWORD*>(target + 1) =
        reinterpret_cast<DWORD>(&TooltipHook) - (kTooltipFunc + 5);
    VirtualProtect(target, 5, oldProtect, &oldProtect);
    FlushInstructionCache(GetCurrentProcess(), target, 5);
    return true;
}

} // namespace

BOOL APIENTRY DllMain(HMODULE module, DWORD reason, LPVOID) {
    if (reason == DLL_PROCESS_ATTACH) {
        DisableThreadLibraryCalls(module);
        InstallTooltipHook();
    }
    return TRUE;
}
