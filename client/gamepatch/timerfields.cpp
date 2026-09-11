// O contador do MsgStartTime (0x3A1) em campos do mapa que o WYD.exe 7662 não
// conhece.
//
// O cliente guarda os segundos do pacote e, a cada quadro, só desenha o contador
// se o campo de 128x128 em que o personagem está for um de 15 pares fixos (laço
// em 0x47DAA4-0x47DE75: Duelo, Carta, Água, Pesadelo e mais sete). Fora deles ele
// esconde o contador e zera a flag, e só um pacote novo o traz de volta. O
// Castelo Orc fica no campo (19,16), fora da lista: o servidor manda o 0x3A1 e
// nada aparece.
//
// O desvio entra no primeiro par, em 0x47DACA. Se o campo for um dos daqui, salta
// para o desenho (0x47DD1A), como fazem os 15 pares; senão refaz a comparação que
// cobriu e devolve ao laço, que segue igual. Campo novo com contador: mais um par
// de cmp/jne em FieldHook.
//
// Instalado por um objeto global, que o runtime constrói antes do DllMain. Assim
// este arquivo não mexe no gamepatch.cpp e só depende da linha do build.bat.

#include <windows.h>

#include <cstring>

namespace {

constexpr DWORD kFieldHookAt = 0x47DACA;
// Depois das três instruções cobertas: o jnz do primeiro par, (1,31).
constexpr DWORD kFieldHookBack = 0x47DADA;
// Para onde todo par encontrado salta: a conta dos segundos e o desenho.
constexpr DWORD kDrawTimer = 0x47DD1A;

// mov ecx,[ebp-7B8h] / mov edx,[ecx+40h] / cmp dword ptr [edx+20A20h],1 — o
// objeto do jogo em [ebp-7B8h], o personagem em +0x40, e o campo x e y dele em
// +0x20A20 e +0x20A24.
const BYTE kExpectedFieldBytes[16] = {
    0x8B, 0x8D, 0x48, 0xF8, 0xFF, 0xFF,
    0x8B, 0x51, 0x40,
    0x83, 0xBA, 0x20, 0x0A, 0x02, 0x00, 0x01,
};

// Os saltos de volta são literais (push/ret): kDrawTimer e kFieldHookBack não
// entram no __asm como constexpr. push e ret não mexem nas flags, e o jnz em
// kFieldHookBack lê as do cmp refeito.
__declspec(naked) void FieldHook() {
    __asm {
        mov ecx, dword ptr [ebp - 0x7B8]
        mov edx, dword ptr [ecx + 0x40]
        // Castelo Orc: x 2432-2559, y 2048-2175.
        cmp dword ptr [edx + 0x20A20], 19
        jne original
        cmp dword ptr [edx + 0x20A24], 16
        jne original
        push 0x47DD1A // kDrawTimer
        ret
    original:
        cmp dword ptr [edx + 0x20A20], 1
        push 0x47DADA // kFieldHookBack
        ret
    }
}

// Confere os bytes antes de gravar, como os outros ganchos: se não forem os da
// 7662, o contador fica como o cliente o faz, em vez de o jogo cair.
bool InstallFieldHook() {
    BYTE* target = reinterpret_cast<BYTE*>(kFieldHookAt);
    if (memcmp(target, kExpectedFieldBytes, sizeof(kExpectedFieldBytes)) != 0) {
        return false;
    }
    DWORD oldProtect = 0;
    if (!VirtualProtect(target, sizeof(kExpectedFieldBytes), PAGE_EXECUTE_READWRITE, &oldProtect)) {
        return false;
    }
    target[0] = 0xE9; // jmp rel32
    *reinterpret_cast<DWORD*>(target + 1) = reinterpret_cast<DWORD>(&FieldHook) - (kFieldHookAt + 5);
    memset(target + 5, 0x90, sizeof(kExpectedFieldBytes) - 5); // o resto das três instruções
    VirtualProtect(target, sizeof(kExpectedFieldBytes), oldProtect, &oldProtect);
    FlushInstructionCache(GetCurrentProcess(), target, sizeof(kExpectedFieldBytes));
    return true;
}

struct FieldHookInstaller {
    FieldHookInstaller() { InstallFieldHook(); }
} g_fieldHookInstaller;

} // namespace
